package manager

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/internal/controller"
	moduleconfig "github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/pkg/config"
	v2manager "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/manager"
)

var (
	ErrKubeConfigRequired    = errors.New("kubeconfig is required")
	ErrConfigurationRequired = errors.New("configuration is required")
)

// New creates the controller-runtime manager, configures cache/client
// behavior, registers the controller, and installs health checks.
func New(
	kubeConfig *rest.Config,
	configuration *moduleconfig.Config,
) (ctrlmanager.Manager, error) {
	if kubeConfig == nil {
		return nil, ErrKubeConfigRequired
	}
	if configuration == nil {
		return nil, ErrConfigurationRequired
	}

	err := configuration.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate configuration: %w", err)
	}

	scheme := runtime.NewScheme()

	err = clientgoscheme.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("add Kubernetes APIs to scheme: %w", err)
	}

	err = v1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("add example API to scheme: %w", err)
	}

	//nolint:exhaustruct_v5 // the example configures cache and client paths explicitly.
	runtimeManager, err := ctrl.NewManager(kubeConfig, ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: configuration.HealthProbeBindAddress,
		Cache: cache.Options{
			ReaderFailOnMissingInformer: true,
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				Unstructured: true,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create controller manager: %w", err)
	}

	manager := v2manager.New(runtimeManager)
	err = controller.Setup(manager, configuration)
	if err != nil {
		return nil, fmt.Errorf("setup controller: %w", err)
	}

	err = manager.AddHealthzCheck("healthz", healthz.Ping)
	if err != nil {
		return nil, fmt.Errorf("add health check: %w", err)
	}

	err = manager.AddReadyzCheck("readyz", healthz.Ping)
	if err != nil {
		return nil, fmt.Errorf("add readiness check: %w", err)
	}

	return manager, nil
}
