package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"catops/internal/config"
	"catops/internal/ui"
)

// NewConfigCmd creates the config command
func NewConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show current configuration",
		Long: `Show current CatOps configuration including thresholds and cloud mode status.

Use 'catops config show' to see current settings.
Use 'catops set' to change alert thresholds.
Use 'catops auth' to manage cloud mode authentication.`,
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

			// Show current configuration
			ui.PrintSection("Cloud Mode Status")
			if cfg.AuthToken != "" {
				ui.PrintStatus("success", fmt.Sprintf("Auth Token: %s...%s", cfg.AuthToken[:10], cfg.AuthToken[len(cfg.AuthToken)-10:]))
				ui.PrintStatus("success", "Cloud Mode: Enabled")
				ui.PrintStatus("info", "Metrics sent to backend with Telegram alerts")
			} else {
				ui.PrintStatus("warning", "Cloud Mode: Disabled")
				ui.PrintStatus("info", "Running in local mode (no alerts)")
				ui.PrintStatus("info", "Run 'catops auth login <token>' to enable cloud mode")
			}
			ui.PrintSectionEnd()

			ui.PrintSection("Alert Thresholds")
			ui.PrintStatus("success", fmt.Sprintf("CPU Threshold: %.1f%%", cfg.CPUThreshold))
			ui.PrintStatus("success", fmt.Sprintf("Memory Threshold: %.1f%%", cfg.MemThreshold))
			ui.PrintStatus("success", fmt.Sprintf("Disk Threshold: %.1f%%", cfg.DiskThreshold))
			ui.PrintSectionEnd()

			ui.PrintSection("Alert Sensitivity")
			ui.PrintStatus("info", fmt.Sprintf("Spike Detection: %.1f%%", cfg.SuddenSpikeThreshold))
			ui.PrintStatus("info", fmt.Sprintf("Gradual Rise: %.1f%%", cfg.GradualRiseThreshold))
			ui.PrintStatus("info", fmt.Sprintf("Anomaly Threshold: %.1fσ (std deviations)", cfg.AnomalyThreshold))
			ui.PrintStatus("info", fmt.Sprintf("Re-notify Interval: %d minutes", cfg.AlertRenotifyInterval))
			ui.PrintStatus("info", "Use 'catops set spike=30 gradual=15 anomaly=4.0' to adjust")
			ui.PrintSectionEnd()
		},
	}
}
