package main

import (
	"context"
	"flag"
	"log"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/internal/controller"
	v2manager "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/manager"
)

func main() {
	defaults := new(options)
	flag.StringVar(&defaults.ChartPath, "chart", "", "path to the Helm chart to render")
	flag.Parse()
	loadedOptions, err := loadOptions(context.Background(), *defaults)
	if err != nil {
		log.Fatalf("load options: %v", err)
	}

	scheme := runtime.NewScheme()
	err = v1alpha1.AddToScheme(scheme)
	if err != nil {
		log.Fatalf("add example API to scheme: %v", err)
	}
	//nolint:exhaustruct_v5 // the example configures only scheme and health probes.
	runtimeManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: loadedOptions.HealthProbeBindAddress,
	})
	if err != nil {
		log.Fatalf("create manager: %v", err)
	}
	err = runtimeManager.AddHealthzCheck("healthz", healthz.Ping)
	if err != nil {
		log.Fatalf("add health check: %v", err)
	}
	err = runtimeManager.AddReadyzCheck("readyz", healthz.Ping)
	if err != nil {
		log.Fatalf("add readiness check: %v", err)
	}
	manager := v2manager.New(runtimeManager)

	err = controller.Setup(manager, loadedOptions.ChartPath)
	if err != nil {
		log.Fatalf("setup controller: %v", err)
	}

	err = manager.Start(ctrl.SetupSignalHandler())
	if err != nil {
		log.Fatalf("start manager: %v", err)
	}
}
