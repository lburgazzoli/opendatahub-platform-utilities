package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const cleanupReason = "Cleanup"

//nolint:cyclop // Cleanup keeps deadline, outcome, and finalizer decisions together.
func (r *Reconciler) cleanup(ctx context.Context, instance api.PlatformObject) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(instance, r.options.FinalizerName) {
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
		r.recorder.Event(instance, corev1.EventTypeWarning, cleanupReason, outcome.Error())
		return ctrl.Result{}, outcome.Err()
	case outcome.Type() == action.ErrorTypeAdvisory:
		r.recorder.Event(instance, corev1.EventTypeNormal, cleanupReason, outcome.Error())
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
			r.recorder.Event(instance, corev1.EventTypeNormal, cleanupReason, outcome.Error())
		}
	default:
		if outcome.Err() != nil {
			r.recorder.Event(instance, corev1.EventTypeWarning, cleanupReason, outcome.Error())
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
	if controllerutil.RemoveFinalizer(instance, r.options.FinalizerName) {
		err := r.client.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("remove reconciler finalizer: %w", err)
		}
	}

	r.recorder.Event(instance, eventType, cleanupReason, message)

	return ctrl.Result{}, nil
}
