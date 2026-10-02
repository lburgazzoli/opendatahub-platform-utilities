package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	aigatewayv1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/aigateway/v1alpha1"
	kservev1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/kserve/v1alpha1"
	platformv1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/module"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/platform"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/serving"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	platformMode = "platform"
	servingMode  = "serving"
	moduleMode   = "module"
)

var (
	ErrUsage             = errors.New("usage: manager run controller {platform|serving} | run module {kserve|aigateway}")
	ErrModuleImage       = errors.New("PLATFORM_MODULE_IMAGE is required")
	ErrUnknownController = errors.New("unknown controller")
)

func main() {
	err := run(os.Args[1:])
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	mode, moduleName, err := parseCommand(args)
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

	manager, err := ctrl.NewManager(config, ctrl.Options{
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

	ctx := ctrl.SetupSignalHandler()
	err = setupController(manager, mode, moduleName)
	if err != nil {
		return err
	}

	return manager.Start(ctx)
}

func parseCommand(args []string) (string, string, error) {
	if len(args) != 3 || args[0] != "run" {
		return "", "", ErrUsage
	}

	switch {
	case args[1] == "controller" && (args[2] == platformMode || args[2] == servingMode):
		return args[2], "", nil
	case args[1] == moduleMode && args[2] != "":
		return moduleMode, args[2], nil
	}

	return "", "", ErrUsage
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

func setupController(manager ctrlmanager.Manager, mode string, moduleName string) error {
	root := cmp.Or(os.Getenv("PLATFORM_MODULES_DIR"), "config/modules")
	registry, err := modules.Load(root)
	if err != nil {
		return err
	}

	switch mode {
	case moduleMode:
		definition, found := registry.Get(moduleName)
		if !found {
			return fmt.Errorf("%w: module %q", ErrUnknownController, moduleName)
		}
		object, err := manager.GetScheme().New(definition.GVK())
		if err != nil {
			return fmt.Errorf("construct module %q: %w", moduleName, err)
		}
		prototype, ok := object.(platformapi.PlatformObject)
		if !ok {
			return fmt.Errorf("%w: module %q does not implement PlatformObject", ErrUnknownController, moduleName)
		}
		return module.Setup(manager, prototype)
	case platformMode:
		image := os.Getenv("PLATFORM_MODULE_IMAGE")
		if image == "" {
			return ErrModuleImage
		}

		namespace := cmp.Or(os.Getenv("PLATFORM_MODULE_NAMESPACE"), "default")
		return platform.Setup(manager, registry, image, namespace)
	case servingMode:
		return serving.Setup(manager, registry, filepath.Join(filepath.Dir(root), "charts"))
	default:
		return fmt.Errorf("%w: %q", ErrUnknownController, mode)
	}
}
