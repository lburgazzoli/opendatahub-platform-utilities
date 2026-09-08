package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"

	moduleconfig "github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/pkg/config"
	modulemanager "github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/pkg/manager"
)

func main() {
	command := &cobra.Command{
		Use:          "helm-builder",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			configuration, err := moduleconfig.Load(cmd.Context())
			if err != nil {
				return fmt.Errorf("load configuration: %w", err)
			}

			manager, err := modulemanager.New(ctrl.GetConfigOrDie(), configuration)
			if err != nil {
				return fmt.Errorf("create manager: %w", err)
			}

			return manager.Start(cmd.Context())
		},
	}

	err := command.ExecuteContext(ctrl.SetupSignalHandler())
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
