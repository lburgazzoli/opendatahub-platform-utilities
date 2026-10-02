package main

import (
	"context"
	"fmt"

	aigatewayv1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/aigateway/v1alpha1"
	kservev1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/kserve/v1alpha1"
	platformv1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func startController(
	ctx context.Context,
	modulesDir string,
	setup func(manager.Manager, *modules.Registry) error,
) error {
	registry, err := modules.Load(modulesDir)
	if err != nil {
		return err
	}

	scheme, err := controllerScheme()
	if err != nil {
		return err
	}

	config, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("get Kubernetes config: %w", err)
	}

	controllerManager, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                server.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		Client: client.Options{Cache: &client.CacheOptions{
			Unstructured: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("create controller manager: %w", err)
	}

	err = setup(controllerManager, registry)
	if err != nil {
		return err
	}

	return controllerManager.Start(ctx)
}

func controllerScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("register Kubernetes APIs: %w", err)
	}

	err = apiextensionsv1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("register CRD API: %w", err)
	}

	err = platformv1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("register platform APIs: %w", err)
	}

	err = kservev1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("register Kserve API: %w", err)
	}

	err = aigatewayv1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("register AIGateway API: %w", err)
	}

	return scheme, nil
}
