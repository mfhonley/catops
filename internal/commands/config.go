package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"catops/internal/analytics"
	"catops/internal/config"
	"catops/internal/metrics"
	"catops/internal/ui"
	"catops/pkg/utils"
)

// NewConfigCmd creates the config command
func NewConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Configure Telegram bot and backend analytics",
		Long: `Configure Telegram bot token and group ID.
This allows you to set up or change your configuration for notifications.

Use 'catops config show' to see current settings.`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Configuration")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Using default values")
				cfg = &config.Config{}
			}

			if len(args) == 0 {
				ui.PrintStatus("error", "Usage: catops config [token=...|group=...|show]")
				ui.PrintStatus("info", "Run 'catops config show' to see current settings")
				ui.PrintSectionEnd()
				return
			}

			arg := args[0]
			if arg == "show" {
				// show current configuration
				ui.PrintSection("Backend Analytics Configuration")
				if cfg.AuthToken != "" {
					ui.PrintStatus("success", fmt.Sprintf("Auth Token: %s...%s", cfg.AuthToken[:10], cfg.AuthToken[len(cfg.AuthToken)-10:]))
					ui.PrintStatus("info", "Analytics will be sent to backend")
				} else {
					ui.PrintStatus("warning", "Auth Token: Not configured")
					ui.PrintStatus("info", "Analytics won't be sent to backend")
				}
				ui.PrintSectionEnd()

				ui.PrintSection("Alert Thresholds")
				ui.PrintStatus("success", fmt.Sprintf("CPU Threshold: %.1f%%", cfg.CPUThreshold))
				ui.PrintStatus("success", fmt.Sprintf("Memory Threshold: %.1f%%", cfg.MemThreshold))
				ui.PrintStatus("success", fmt.Sprintf("Disk Threshold: %.1f%%", cfg.DiskThreshold))
				ui.PrintSectionEnd()
				return
			}

			// parse argument
			parts := strings.Split(arg, "=")
			if len(parts) != 2 {
				ui.PrintStatus("error", "Invalid format. Use: token=..., group=..., or auth=...")
				ui.PrintSectionEnd()
				return
			}

			key := parts[0]
			value := parts[1]

			switch key {
			case "token":
				if len(value) < 20 {
					ui.PrintStatus("error", "Invalid token format. Token should be longer")
					ui.PrintSectionEnd()
					return
				}
				ui.PrintStatus("success", "Bot token updated successfully")

			case "group":
				groupID, err := utils.ParseInt(value)
				if err != nil {
					ui.PrintStatus("error", "Invalid group ID. Must be a number")
					ui.PrintSectionEnd()
					return
				}
				ui.PrintStatus("success", fmt.Sprintf("Group ID updated to: %d", groupID))

			case "auth":
				ui.PrintStatus("error", "Use 'catops auth login <token>' instead")
				ui.PrintSectionEnd()
				return

			default:
				ui.PrintStatus("error", "Unknown setting. Use: token, group, or show")
				ui.PrintSectionEnd()
				return
			}

			// save configuration
			{
				err = config.SaveConfig(cfg)
				if err != nil {
					ui.PrintStatus("error", fmt.Sprintf("Failed to save config: %v", err))
					return
				}
			}

			ui.PrintStatus("success", "Configuration saved successfully")

			// Send config_change event
			if cfg.AuthToken != "" && cfg.ServerID != "" {
				ui.PrintStatus("info", "Sending config_change event to backend...")
				currentMetrics, err := metrics.GetMetrics()
				if err != nil {
					ui.PrintStatus("warning", fmt.Sprintf("Failed to get metrics for event: %v", err))
					ui.PrintStatus("info", "Sending event without metrics...")
					// Still send event without metrics
					emptyMetrics := &metrics.Metrics{}
					analytics.NewSender(cfg, GetCurrentVersion()).SendAllSync("config_change", emptyMetrics)
				} else {
					analytics.NewSender(cfg, GetCurrentVersion()).SendAllSync("config_change", currentMetrics)
				}
				ui.PrintStatus("success", "Config change event sent")
			} else {
				ui.PrintStatus("info", "Cloud mode not configured - event not sent")
			}

			ui.PrintStatus("info", "Run 'catops restart' to apply changes to the monitoring service")
			ui.PrintSectionEnd()
		},
	}
}
