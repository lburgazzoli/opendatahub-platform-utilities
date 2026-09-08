package manager

import (
	"errors"
	"fmt"

	"k8s.io/client-go/rest"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"

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

	runtimeManager, err := createRuntimeManager(kubeConfig, configuration)
	if err != nil {
		return nil, fmt.Errorf("create controller manager: %w", err)
	}

	manager := v2manager.New(runtimeManager)
	err = setupManager(manager, configuration)
	if err != nil {
		return nil, err
	}

	return manager, nil
}
