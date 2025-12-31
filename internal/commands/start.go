package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"catops/internal/service"
	"catops/internal/ui"
)

// NewStartCmd creates the start command
func NewStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start background monitoring service",
		Long: `Start the monitoring service in the background.
The service will continuously collect system metrics and send them to the cloud.

For first-time setup, use 'catops service install' to install as a system service.

Examples:
  catops start              # Start the monitoring service
  catops service install    # Install as system service (recommended)`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Starting Monitoring Service")

			svc, err := service.New()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to create service: %v", err))
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			status, err := svc.Start()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to start: %v", err))
				ui.PrintStatus("info", "Try 'catops service install' to install the service first")
				ui.PrintSectionEnd()
				os.Exit(1)
			}

			ui.PrintStatus("success", status)
			ui.PrintSectionEnd()
		},
	}
}
