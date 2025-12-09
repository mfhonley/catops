package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"catops/internal/analytics"
	"catops/internal/config"
	"catops/internal/process"
	"catops/internal/ui"
)

// NewRestartCmd creates the restart command
func NewRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Stop and restart the monitoring service",
		Long: `Stop the current monitoring process and start a new one.
This ensures the monitoring service uses the latest configuration.
The Telegram bot will also be restarted if configured.

Examples:
  catops restart         # Restart monitoring with current config`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Restarting Monitoring Service")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to load config: %v", err))
				ui.PrintSectionEnd()
				return
			}

			// stop current process
			if process.IsRunning() {
				err := process.StopProcess()
				if err != nil {
					ui.PrintErrorWithSupport(fmt.Sprintf("Failed to stop: %v", err))
					ui.PrintSectionEnd()
					return
				}
				ui.PrintStatus("success", "Monitoring service stopped")
			}

			// start new process
			err = process.StartProcess()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to start: %v", err))
				ui.PrintSectionEnd()
				return
			}

			ui.PrintStatus("success", "Monitoring service restarted successfully")

			// Send service_restart event
			if cfg.AuthToken != "" && cfg.ServerID != "" {
				analytics.NewSender(cfg, GetCurrentVersion()).SendEvent("service_restart")
			}

			ui.PrintSectionEnd()
		},
	}
}
