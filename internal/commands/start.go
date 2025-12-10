package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"catops/internal/process"
	"catops/internal/ui"
)

// NewStartCmd creates the start command
func NewStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start background monitoring service",
		Long: `Start the monitoring service in the background.
The service will continuously collect system metrics and send them to the cloud.

To run in background (recommended):
  nohup catops start > /dev/null 2>&1 &

To run in foreground (for testing):
  catops start

Examples:
  catops start           # Start monitoring service (foreground)
  nohup catops start &   # Start monitoring service (background)`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Starting Monitoring Service")

			if process.IsRunning() {
				ui.PrintStatus("warning", "Monitoring service is already running")
				ui.PrintSectionEnd()
				return
			}

			err := process.StartProcess()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to start: %v", err))
				ui.PrintSectionEnd()
				return
			}

			ui.PrintStatus("success", "Monitoring service started successfully")
			ui.PrintSectionEnd()
		},
	}
}
