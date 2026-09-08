package tls

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SecurityProfileWatcher watches the cluster APIServer object for profile
// changes. The callback can restart a process so it rebuilds TLS settings.
type SecurityProfileWatcher struct {
	client.Client

	// InitialTLSProfileSpec is the profile applied when the process started.
	InitialTLSProfileSpec configv1.TLSProfileSpec
	// OnProfileChange is called after the effective profile changes.
	OnProfileChange func(
		ctx context.Context,
		oldProfile configv1.TLSProfileSpec,
		newProfile configv1.TLSProfileSpec,
	)
}

// SetupWithManager registers the non-leader-elected profile watcher.
func (w *SecurityProfileWatcher) SetupWithManager(manager ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(manager).
		Named("tls-security-profile-watcher").
		WithOptions(controller.Options{NeedLeaderElection: new(false)}).
		For(&configv1.APIServer{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(value event.CreateEvent) bool {
				return value.Object.GetName() == APIServerName
			},
			DeleteFunc: func(value event.DeleteEvent) bool {
				return value.Object.GetName() == APIServerName
			},
			GenericFunc: func(value event.GenericEvent) bool {
				return value.Object.GetName() == APIServerName
			},
			UpdateFunc: func(value event.UpdateEvent) bool {
				return value.ObjectNew.GetName() == APIServerName
			},
		})).
		Complete(w)
	if err != nil {
		return fmt.Errorf("register TLS profile watcher: %w", err)
	}

	return nil
}

// Reconcile invokes the callback when the effective TLS profile changes.
func (w *SecurityProfileWatcher) Reconcile(
	ctx context.Context,
	request ctrl.Request,
) (ctrl.Result, error) {
	apiServer := &configv1.APIServer{}
	err := w.Get(ctx, request.NamespacedName, apiServer)
	if err != nil {
		switch {
		case apierrors.IsNotFound(err):
			return ctrl.Result{}, nil
		default:
			return ctrl.Result{}, fmt.Errorf("get APIServer %s: %w", request.String(), err)
		}
	}

	currentProfile := *ProfileSpecFromSecurityProfile(apiServer.Spec.TLSSecurityProfile)
	if equality.Semantic.DeepEqual(w.InitialTLSProfileSpec, currentProfile) {
		return ctrl.Result{}, nil
	}

	if w.OnProfileChange != nil {
		w.OnProfileChange(ctx, w.InitialTLSProfileSpec, currentProfile)
	}

	w.InitialTLSProfileSpec = currentProfile
	return ctrl.Result{}, nil
}

var _ reconcile.Reconciler = (*SecurityProfileWatcher)(nil)
