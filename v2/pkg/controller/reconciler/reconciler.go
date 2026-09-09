package reconciler

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler/dynamicwatcher"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	provisioningReason = "Provisioning"
	advisoryReason     = "Advisory"
	cleanupReason      = "Cleanup"
)

// Reconciler owns controller-runtime lifecycle integration for one primary object.
//
//nolint:govet // The fields are grouped by client, pipeline, and lifecycle state.
type Reconciler struct {
	client    client.Client
	scheme    *runtime.Scheme
	prototype api.PlatformObject
	pipeline  *pipeline.Pipeline
	options   Options
	recorder  record.EventRecorder
	dynamic   *dynamicwatcher.Watcher
}

var _ reconcile.Reconciler = (*Reconciler)(nil)

func newInstance(prototype api.PlatformObject) (api.PlatformObject, error) {
	if prototype == nil || reflect.ValueOf(prototype).IsZero() {
		return nil, ErrPrototypeRequired
	}

	instance, ok := prototype.DeepCopyObject().(api.PlatformObject)
	if !ok || instance == nil || reflect.ValueOf(instance).IsZero() {
		return nil, fmt.Errorf("%w: %T does not implement api.PlatformObject", ErrPrototypeCopy, prototype)
	}

	return instance, nil
}

// Reconcile loads one authoritative primary object, runs the appropriate
// lifecycle, persists status, and translates the final action outcome.
func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	instance, err := newInstance(r.prototype)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	_, err = resources.EnsureGroupVersionKind(r.scheme, instance)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("ensure primary GVK: %w", err)
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		return r.cleanup(ctx, instance)
	}

	if r.pipeline.HasCleanupActions() && !controllerutil.ContainsFinalizer(instance, DefaultFinalizerName) {
		controllerutil.AddFinalizer(instance, DefaultFinalizerName)
		err = r.client.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("add reconciler finalizer: %w", err)
		}

		return ctrl.Result{}, nil
	}

	requestValue := r.request(instance)

	outcome := r.pipeline.Run(ctx, requestValue)
	err = r.applyStatus(ctx, instance, outcome)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.interpret(instance, outcome, "reconcile", provisioningReason)
}

//nolint:cyclop // Cleanup keeps deadline, outcome, and finalizer decisions together.
func (r *Reconciler) cleanup(ctx context.Context, instance api.PlatformObject) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(instance, DefaultFinalizerName) {
		return ctrl.Result{}, nil
	}

	deadline, hasDeadline := r.cleanupDeadline(instance)
	if hasDeadline && !time.Now().Before(deadline) {
		return r.finishCleanup(ctx, instance, corev1.EventTypeWarning, "cleanup deadline reached before execution")
	}

	cleanupContext := ctx
	var cancel context.CancelFunc
	if hasDeadline {
		cleanupContext, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	requestValue := r.request(instance)
	outcome := r.pipeline.Cleanup(cleanupContext, requestValue)

	if hasDeadline && !time.Now().Before(deadline) {
		message := "cleanup deadline reached"
		if outcome.Err() != nil {
			message += ": " + outcome.Error()
		}

		return r.finishCleanup(
			ctx,
			instance,
			corev1.EventTypeWarning,
			message,
		)
	}

	switch {
	case outcome.Err() != nil && outcome.Type() != action.ErrorTypeAdvisory:
		return r.interpretCleanup(ctx, instance, outcome, deadline, hasDeadline)
	case outcome.RequeueAfter() > 0:
		return r.interpretCleanup(ctx, instance, outcome, deadline, hasDeadline)
	default:
		return r.finishCleanup(ctx, instance, corev1.EventTypeNormal, "cleanup completed")
	}
}

func (r *Reconciler) cleanupDeadline(instance client.Object) (time.Time, bool) {
	if r.options.CleanupTimeout == nil || *r.options.CleanupTimeout == 0 || instance.GetDeletionTimestamp().IsZero() {
		return time.Time{}, false
	}

	return instance.GetDeletionTimestamp().Add(*r.options.CleanupTimeout), true
}

func (r *Reconciler) interpretCleanup(
	ctx context.Context,
	instance api.PlatformObject,
	outcome action.ActionError,
	deadline time.Time,
	hasDeadline bool,
) (ctrl.Result, error) {
	switch {
	case outcome.RequeueAfter() > 0:
		return r.cleanupRequeue(ctx, instance, outcome, deadline, hasDeadline)
	case outcome.Err() != nil && outcome.Type() != action.ErrorTypeAdvisory:
		r.emit(instance, corev1.EventTypeWarning, cleanupReason, outcome.Error())
		return ctrl.Result{}, outcome.Err()
	case outcome.Type() == action.ErrorTypeAdvisory:
		r.emit(instance, corev1.EventTypeNormal, cleanupReason, outcome.Error())
	}

	return r.finishCleanup(ctx, instance, corev1.EventTypeNormal, "cleanup completed")
}

func (r *Reconciler) cleanupRequeue(
	ctx context.Context,
	instance api.PlatformObject,
	outcome action.ActionError,
	deadline time.Time,
	hasDeadline bool,
) (ctrl.Result, error) {
	if hasDeadline {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return r.finishCleanup(ctx, instance, corev1.EventTypeWarning, "cleanup deadline reached")
		}

		if outcome.RequeueAfter() > remaining {
			return ctrl.Result{RequeueAfter: remaining}, nil
		}
	}

	switch outcome.Type() {
	case action.ErrorTypeAdvisory:
		if outcome.Err() != nil {
			r.emit(instance, corev1.EventTypeNormal, cleanupReason, outcome.Error())
		}
	default:
		if outcome.Err() != nil {
			r.emit(instance, corev1.EventTypeWarning, cleanupReason, outcome.Error())
		}
	}

	return ctrl.Result{RequeueAfter: outcome.RequeueAfter()}, nil
}

func (r *Reconciler) finishCleanup(
	ctx context.Context,
	instance api.PlatformObject,
	eventType string,
	message string,
) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(instance, DefaultFinalizerName)
	err := r.client.Update(ctx, instance)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("remove reconciler finalizer: %w", err)
	}

	r.emit(instance, eventType, cleanupReason, message)

	return ctrl.Result{}, nil
}

func (r *Reconciler) interpret(
	instance api.PlatformObject,
	outcome action.ActionError,
	actionName string,
	reason string,
) (ctrl.Result, error) {
	switch {
	case outcome.Type() == action.ErrorTypeAdvisory && outcome.Err() != nil:
		r.emit(instance, corev1.EventTypeNormal, reason, outcome.Error())
	case outcome.Err() != nil:
		r.emit(instance, corev1.EventTypeWarning, reason, outcome.Error())
	}

	if outcome.RequeueAfter() > 0 {
		return ctrl.Result{RequeueAfter: outcome.RequeueAfter()}, nil
	}

	if outcome.Err() != nil && outcome.Type() != action.ErrorTypeAdvisory {
		return ctrl.Result{}, fmt.Errorf("%s failed: %w", actionName, outcome.Err())
	}

	return ctrl.Result{RequeueAfter: r.options.DefaultRequeueAfter}, nil
}

func (r *Reconciler) emit(object client.Object, eventType string, reason string, message string) {
	if r.recorder == nil {
		return
	}

	r.recorder.Event(object, eventType, reason, message)
}
