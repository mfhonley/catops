package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"catops/internal/config"
	"catops/internal/logger"
	"catops/internal/server"
	"catops/internal/ui"
)

// NewUpdateCmd creates the update command
func NewUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Download and install the latest version",
		Long: `Check for and install the latest version of CatOps.
This will check if updates are available and install them if found.
The update process is handled by the official update script.

Examples:
  catops update          # Check and install updates`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Checking for Updates")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Continuing with update check without server version")
				cfg = &config.Config{}
			}

			// Check if we have authentication
			if cfg.AuthToken == "" {
				ui.PrintStatus("warning", "No authentication token found")
				ui.PrintStatus("info", "Run 'catops auth login <token>' to authenticate")
				ui.PrintStatus("info", "Continuing with basic update check...")

				// Fallback to basic update check
				server.CheckBasicUpdate(GetCurrentVersion())
				return
			}

			// Check server version against latest
			ui.PrintStatus("info", "Checking server version...")
			serverVersion, latestVersion, _, err := server.CheckServerVersion(cfg.AuthToken, GetCurrentVersion())
			if err != nil {
				ui.PrintStatus("warning", fmt.Sprintf("Failed to check server version: %v", err))
				ui.PrintStatus("info", "Falling back to basic update check...")
				server.CheckBasicUpdate(GetCurrentVersion())
				return
			}

			// Show CLI binary version as current version (not database version)
			currentVersion := GetCurrentVersion()
			ui.PrintStatus("info", fmt.Sprintf("Current version: %s", currentVersion))
			ui.PrintStatus("info", fmt.Sprintf("Latest version: %s", latestVersion))

			// Log if database version differs from binary version (for debugging)
			if serverVersion != "" && serverVersion != currentVersion {
				logger.Warning("Version mismatch - Binary: %s, Database: %s", currentVersion, serverVersion)
			}

			// Check if binary needs update (compare binary version with latest)
			if currentVersion == latestVersion {
				ui.PrintStatus("success", "Server is up to date!")
				ui.PrintSectionEnd()
				return
			}

			ui.PrintStatus("info", "Update available! Installing...")
			ui.PrintSectionEnd()

			// Execute the update script
			server.ExecuteUpdateScript(GetCurrentVersion())
		},
	}
}
