package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	constants "catops/config"
	"catops/internal/analytics"
	"catops/internal/config"
	"catops/internal/ui"
	"catops/pkg/utils"
)

// NewSetCmd creates the set command
func NewSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "Configure alert thresholds for CPU, Memory, and Disk",
		Long: `Set individual alert thresholds for system metrics.
After changing thresholds, run 'catops restart' to apply changes to the running service.

Supported metrics:
  • cpu          - CPU usage percentage (0-100)
  • mem          - Memory usage percentage (0-100)
  • disk         - Disk usage percentage (0-100)
  • spike        - Sudden spike detection threshold (0-100)
  • gradual      - Gradual rise detection threshold (0-100)
  • anomaly      - Statistical anomaly threshold (standard deviations, 1.0-10.0)
  • renotify     - Alert re-notification interval in minutes
  • interval     - Metrics collection interval in seconds (10-300)
  • buffer       - Historical data buffer size in data points (10-100)
  • resolution   - Alert resolution timeout in minutes (1-60)

Examples:
  catops set cpu=90              # Set CPU threshold to 90%
  catops set mem=80 disk=85      # Set Memory to 80%, Disk to 85%
  catops set spike=30 gradual=15 # Set spike detection to 30%, gradual to 15%
  catops set anomaly=4.0         # Alert only for 4+ std deviations (less sensitive)
  catops set renotify=120        # Re-notify every 2 hours
  catops set interval=30         # Collect metrics every 30 seconds
  catops set buffer=40           # Store 40 data points in history
  catops set resolution=10       # Mark alerts as resolved after 10 minutes
  catops set cpu=70 spike=25 anomaly=5.0  # Set multiple at once`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Configuring Alert Thresholds")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Using default values")
				cfg = &config.Config{
					CPUThreshold:  constants.DEFAULT_CPU_THRESHOLD,
					MemThreshold:  constants.DEFAULT_MEMORY_THRESHOLD,
					DiskThreshold: constants.DEFAULT_DISK_THRESHOLD,
				}
			}

			if len(args) == 0 {
				ui.PrintStatus("error", "Usage: catops set cpu=90 mem=90 disk=90")
				ui.PrintStatus("info", "Supported: cpu, mem, disk, spike, gradual, anomaly, renotify, interval, buffer, resolution")
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

				// Validate threshold range (0-100 for percentages, any positive for renotify)
				if metric != "renotify" && !utils.IsValidThreshold(value) {
					ui.PrintStatus("error", fmt.Sprintf("Invalid threshold for %s: %.1f%% (must be 0-100)", metric, value))
					continue
				}

				switch metric {
				case "cpu":
					cfg.CPUThreshold = value
					ui.PrintStatus("success", fmt.Sprintf("Set %s threshold to %.1f%%", metric, value))
				case "mem":
					cfg.MemThreshold = value
					ui.PrintStatus("success", fmt.Sprintf("Set %s threshold to %.1f%%", metric, value))
				case "disk":
					cfg.DiskThreshold = value
					ui.PrintStatus("success", fmt.Sprintf("Set %s threshold to %.1f%%", metric, value))
				case "spike":
					cfg.SuddenSpikeThreshold = value
					ui.PrintStatus("success", fmt.Sprintf("Set sudden spike threshold to %.1f%%", value))
				case "gradual":
					cfg.GradualRiseThreshold = value
					ui.PrintStatus("success", fmt.Sprintf("Set gradual rise threshold to %.1f%%", value))
				case "anomaly":
					if value < 1.0 || value > 10.0 {
						ui.PrintStatus("error", "Anomaly threshold must be between 1.0 and 10.0 (standard deviations)")
						continue
					}
					cfg.AnomalyThreshold = value
					ui.PrintStatus("success", fmt.Sprintf("Set anomaly threshold to %.1fσ (std deviations)", value))
				case "renotify":
					if value <= 0 {
						ui.PrintStatus("error", "Re-notify interval must be positive")
						continue
					}
					cfg.AlertRenotifyInterval = int(value)
					ui.PrintStatus("success", fmt.Sprintf("Set re-notify interval to %d minutes", int(value)))
				case "interval":
					if value < 10 || value > 300 {
						ui.PrintStatus("error", "Collection interval must be between 10 and 300 seconds")
						continue
					}
					cfg.CollectionInterval = int(value)
					ui.PrintStatus("success", fmt.Sprintf("Set collection interval to %d seconds", int(value)))
				case "buffer":
					if value < 10 || value > 100 {
						ui.PrintStatus("error", "Buffer size must be between 10 and 100 data points")
						continue
					}
					cfg.BufferSize = int(value)
					ui.PrintStatus("success", fmt.Sprintf("Set buffer size to %d data points", int(value)))
				case "resolution":
					if value < 1 || value > 60 {
						ui.PrintStatus("error", "Alert resolution timeout must be between 1 and 60 minutes")
						continue
					}
					cfg.AlertResolutionTimeout = int(value)
					ui.PrintStatus("success", fmt.Sprintf("Set alert resolution timeout to %d minutes", int(value)))
				default:
					ui.PrintStatus("error", fmt.Sprintf("Unknown metric: %s", metric))
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
