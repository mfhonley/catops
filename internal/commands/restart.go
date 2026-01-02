package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"catops/internal/analytics"
	"catops/internal/config"
	"catops/internal/service"
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

			svc, err := service.New()
			if err != nil {
				ui.PrintErrorWithSupport(fmt.Sprintf("Failed to create service: %v", err))
				ui.PrintSectionEnd()
				return
			}

			// Stop current service
			svc.Stop()
			ui.PrintStatus("info", "Service stopped")

			// Start service
			status, err := svc.Start()
			if err != nil {
				// Service might not be installed, try installing it first
				ui.PrintStatus("warning", "Service not installed, installing...")
				installStatus, installErr := svc.Install()
				if installErr != nil {
					ui.PrintErrorWithSupport(fmt.Sprintf("Failed to install service: %v", installErr))
					ui.PrintSectionEnd()
					return
				}
				ui.PrintStatus("success", installStatus)

				// Now try starting again
				status, err = svc.Start()
				if err != nil {
					ui.PrintErrorWithSupport(fmt.Sprintf("Failed to start: %v", err))
					ui.PrintSectionEnd()
					return
				}
			}

			ui.PrintStatus("success", status)

			// Send service_restart event
			if cfg.AuthToken != "" && cfg.ServerID != "" {
				analytics.NewSender(cfg, GetCurrentVersion()).SendEvent("service_restart")
			}

			ui.PrintSectionEnd()
		},
	}
}
