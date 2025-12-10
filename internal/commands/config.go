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
		Long: `Show current CatOps configuration including cloud mode status.

Use 'catops config show' to see current settings.
Use 'catops set' to change monitoring settings.
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
				ui.PrintStatus("info", "Metrics sent to backend with notifications")
			} else {
				ui.PrintStatus("warning", "Cloud Mode: Disabled")
				ui.PrintStatus("info", "Running in local mode (no notifications)")
				ui.PrintStatus("info", "Run 'catops auth login <token>' to enable cloud mode")
			}
			ui.PrintSectionEnd()

			ui.PrintSection("Monitoring Configuration")
			ui.PrintStatus("info", fmt.Sprintf("Collection Interval: %d seconds", cfg.CollectionInterval))
			ui.PrintStatus("info", "Use 'catops set interval=30' to adjust")
			ui.PrintSectionEnd()
		},
	}
}
