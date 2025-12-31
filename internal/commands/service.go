package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"catops/internal/service"
	"catops/internal/ui"
)

// NewServiceCmd creates the service command with subcommands
func NewServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage CatOps system service",
		Long: `Manage CatOps as a system service (systemd on Linux, launchd on macOS).

The service command allows you to install, remove, start, stop, and check
the status of CatOps as a background system service.

Examples:
  catops service install   # Install and enable CatOps service
  catops service start     # Start the service
  catops service stop      # Stop the service
  catops service status    # Check service status
  catops service remove    # Remove the service`,
	}

	cmd.AddCommand(newServiceInstallCmd())
	cmd.AddCommand(newServiceRemoveCmd())
	cmd.AddCommand(newServiceStartCmd())
	cmd.AddCommand(newServiceStopCmd())
	cmd.AddCommand(newServiceStatusCmd())
	cmd.AddCommand(newServiceRestartCmd())

	return cmd
}

func newServiceInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install CatOps as a system service",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Installing Service")

			svc, err := service.New()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to create service: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			status, err := svc.Install()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to install: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			ui.PrintStatus("success", status)
			ui.PrintStatus("info", "Run 'catops service start' to start monitoring")
			ui.PrintSectionEnd()
		},
	}
}

func newServiceRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove CatOps system service",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Removing Service")

			svc, err := service.New()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to create service: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			// Stop first if running
			svc.Stop()

			status, err := svc.Remove()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to remove: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			ui.PrintStatus("success", status)
			ui.PrintSectionEnd()
		},
	}
}

func newServiceStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start CatOps service",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Starting Service")

			svc, err := service.New()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to create service: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			status, err := svc.Start()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to start: %v", err))
				ui.PrintStatus("info", "Try 'catops service install' first")
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			ui.PrintStatus("success", status)
			ui.PrintSectionEnd()
		},
	}
}

func newServiceStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop CatOps service",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Stopping Service")

			svc, err := service.New()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to create service: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			status, err := svc.Stop()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to stop: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			ui.PrintStatus("success", status)
			ui.PrintSectionEnd()
		},
	}
}

func newServiceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check CatOps service status",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Service Status")

			svc, err := service.New()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to create service: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			status, err := svc.Status()
			if err != nil {
				ui.PrintStatus("warning", fmt.Sprintf("Status: %v", err))
			} else {
				ui.PrintStatus("info", status)
			}
			ui.PrintSectionEnd()
		},
	}
}

func newServiceRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart CatOps service",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Restarting Service")

			svc, err := service.New()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to create service: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			// Stop
			svc.Stop()

			// Start
			status, err := svc.Start()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to restart: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			ui.PrintStatus("success", status)
			ui.PrintSectionEnd()
		},
	}
}
