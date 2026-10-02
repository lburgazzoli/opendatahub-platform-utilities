package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"

	aigatewayv1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/aigateway/v1alpha1"
	kservev1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/kserve/v1alpha1"
	modulecontroller "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/module"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/platform"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/platformmodule"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/serving"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var ErrModuleImage = errors.New("PLATFORM_MODULE_IMAGE is required")

func main() {
	err := newCommand().ExecuteContext(ctrl.SetupSignalHandler())
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

//nolint:funlen // Keep the complete command tree visible in one place.
func newCommand() *cobra.Command {
	modulesDir := cmp.Or(os.Getenv("PLATFORM_MODULES_DIR"), "config/modules")
	root := &cobra.Command{
		Use:           "manager",
		Short:         "Run the platform modules example",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true

	run := &cobra.Command{
		Use:   "run",
		Short: "Run a controller",
		Args:  cobra.NoArgs,
	}

	controller := &cobra.Command{
		Use:   "controller",
		Short: "Run a platform controller",
		Args:  cobra.NoArgs,
	}

	controller.AddCommand(
		&cobra.Command{
			Use:   "platform",
			Short: "Reconcile Platform and PlatformModule resources",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return startController(cmd.Context(), modulesDir, func(manager manager.Manager, registry *modules.Registry) error {
					image := os.Getenv("PLATFORM_MODULE_IMAGE")
					if image == "" {
						return ErrModuleImage
					}

					err := platform.Setup(manager, registry)
					if err != nil {
						return err
					}

					return platformmodule.Setup(manager, registry, image)
				})
			},
		},
		&cobra.Command{
			Use:   "serving",
			Short: "Reconcile Serving resources",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return startController(cmd.Context(), modulesDir, serving.Setup)
			},
		},
	)

	module := &cobra.Command{
		Use:   "module",
		Short: "Run a module controller",
		Args:  cobra.NoArgs,
	}

	module.AddCommand(
		&cobra.Command{
			Use:   "kserve",
			Short: "Reconcile Kserve resources",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return startController(cmd.Context(), modulesDir, func(manager manager.Manager, _ *modules.Registry) error {
					return modulecontroller.Setup(manager, new(kservev1alpha1.Kserve))
				})
			},
		},
		&cobra.Command{
			Use:   "aigateway",
			Short: "Reconcile AI Gateway resources",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return startController(cmd.Context(), modulesDir, func(manager manager.Manager, _ *modules.Registry) error {
					return modulecontroller.Setup(manager, new(aigatewayv1alpha1.AIGateway))
				})
			},
		},
	)

	run.AddCommand(controller, module)
	root.AddCommand(run)

	return root
}
