package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"catops/internal/analytics"
	"catops/internal/config"
	"catops/internal/ui"
	"catops/pkg/utils"
)

// NewSetCmd creates the set command
func NewSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "Configure monitoring settings",
		Long: `Set monitoring configuration options.
After changing settings, run 'catops restart' to apply changes to the running service.

Supported settings:
  • interval     - Metrics collection interval in seconds (10-300)

Examples:
  catops set interval=30         # Collect metrics every 30 seconds`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Configuring Monitoring Settings")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Using default values")
				cfg = &config.Config{}
			}

			if len(args) == 0 {
				ui.PrintStatus("error", "Usage: catops set interval=30")
				ui.PrintStatus("info", "Supported: interval")
				ui.PrintSectionEnd()
				return
			}

			// parse arguments and update config
			for _, arg := range args {
				parts := strings.Split(arg, "=")
				if len(parts) != 2 {
					ui.PrintStatus("error", fmt.Sprintf("Invalid format: %s", arg))
					continue
				}

				metric := parts[0]
				value, err := utils.ParseFloat(parts[1])
				if err != nil {
					ui.PrintStatus("error", fmt.Sprintf("Invalid value for %s: %s", metric, parts[1]))
					continue
				}

				switch metric {
				case "interval":
					if value < 10 || value > 300 {
						ui.PrintStatus("error", "Collection interval must be between 10 and 300 seconds")
						continue
					}
					cfg.CollectionInterval = int(value)
					ui.PrintStatus("success", fmt.Sprintf("Set collection interval to %d seconds", int(value)))
				default:
					ui.PrintStatus("error", fmt.Sprintf("Unknown setting: %s", metric))
					continue
				}
			}

			// save configuration
			err = config.SaveConfig(cfg)
			if err != nil {
				ui.PrintStatus("error", fmt.Sprintf("Failed to save config: %v", err))
				ui.PrintSectionEnd()
				return
			}

			ui.PrintStatus("success", "Configuration saved successfully")

			// Send config_change event
			if cfg.AuthToken != "" && cfg.ServerID != "" {
				ui.PrintStatus("info", "Sending config_change event to backend...")
				analytics.NewSender(cfg, GetCurrentVersion()).SendEventSync("config_change")
				ui.PrintStatus("success", "Config change event sent")
			} else {
				ui.PrintStatus("info", "Cloud mode not configured - event not sent")
			}

			ui.PrintStatus("info", "Run 'catops restart' to apply changes")
			ui.PrintSectionEnd()
		},
	}
}
