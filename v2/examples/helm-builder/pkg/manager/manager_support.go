package manager

import (
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
)

func cacheOptions(namespace string) cache.Options {
	return cache.Options{
		ReaderFailOnMissingInformer: true,
		DefaultNamespaces: map[string]cache.Config{
			namespace: {},
		},
	}
}

func createRuntimeManager(
	kubeConfig *rest.Config,
	configuration *moduleconfig.Config,
) (ctrlmanager.Manager, error) {
	scheme := runtime.NewScheme()

	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("add Kubernetes APIs to scheme: %w", err)
	}

	err = v1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("add example API to scheme: %w", err)
	}

	//nolint:exhaustruct_v5 // the example configures cache and client paths explicitly.
	return ctrl.NewManager(kubeConfig, ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: configuration.HealthProbeBindAddress,
		Cache:                  cacheOptions(configuration.Namespace),
		Client: client.Options{
			Cache: &client.CacheOptions{
				Unstructured: true,
			},
		},
	})
}

func setupManager(
	manager ctrlmanager.Manager,
	configuration *moduleconfig.Config,
) error {
	err := controller.Setup(manager, configuration)
	if err != nil {
		return fmt.Errorf("setup controller: %w", err)
	}

	err = manager.AddHealthzCheck("healthz", healthz.Ping)
	if err != nil {
		return fmt.Errorf("add health check: %w", err)
	}

	err = manager.AddReadyzCheck("readyz", healthz.Ping)
	if err != nil {
		return fmt.Errorf("add readiness check: %w", err)
	}

	return nil
}
