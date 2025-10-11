package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"catops/internal/commands"
	"catops/internal/config"
	"catops/internal/ui"
)

// VERSION is set during build via ldflags
var VERSION string

// Helper functions for server operations

// getCurrentVersion retrieves the current version from build flags or version.txt
func getCurrentVersion() string {
	version := VERSION
	if version == "" {
		// Read version from version.txt if VERSION is not set
		if versionData, err := os.ReadFile("version.txt"); err == nil {
			version = strings.TrimSpace(string(versionData))
		}
	}
	return version
}

func main() {
	// load configuration
	_, err := config.LoadConfig()
	if err != nil {
		ui.PrintErrorWithSupport(fmt.Sprintf("Error loading config: %v", err))
		os.Exit(1)
	}

	// Set version function for commands package
	commands.GetCurrentVersion = getCurrentVersion

	// create root command
	rootCmd := &cobra.Command{
		Use:                "catops",
		Short:              "Professional CatOps Tool",
		DisableSuggestions: true,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			// check if --version flag is set
			if cmd.Flags().Lookup("version").Changed {
				version := VERSION
				if version == "" {
					// read version from version.txt if VERSION is not set
					if versionData, err := os.ReadFile("version.txt"); err == nil {
						version = strings.TrimSpace(string(versionData))
					}
				}
				fmt.Printf("v%s\n", version)
				return nil
			}

			ui.PrintHeader()

			ui.PrintSection("Core Features")

			// features section
			featuresData := map[string]string{
				"System Monitoring": "CPU, Memory, Disk metrics",
				"Alert System":      "Telegram notifications",
				"Remote Control":    "Telegram bot commands",
				"Open Source":       "Free monitoring solution",
				"Lightweight":       "Minimal resource usage",
			}
			fmt.Print(ui.CreateBeautifulList(featuresData))
			ui.PrintSectionEnd()

			// quick start section
			ui.PrintSection("Quick Start")
			quickStartData := map[string]string{
				"Start Service":  "catops start",
				"Set Thresholds": "catops set cpu=90",
				"Apply Changes":  "catops restart",
				"Check Status":   "catops status",
				"Telegram Bot":   "Auto-configured",
				"Cloud Mode":     "catops auth login <token>",
			}
			fmt.Print(ui.CreateBeautifulList(quickStartData))
			ui.PrintSectionEnd()

			// available commands section
			ui.PrintSection("Commands")
			commandsData := map[string]string{
				"status":  "Show system metrics",
				"start":   "Start monitoring service",
				"restart": "Restart monitoring service",
				"set":     "Set alert thresholds",
				"update":  "Update to latest version",
				"auth":    "Manage Cloud Mode authentication",
			}
			fmt.Print(ui.CreateBeautifulList(commandsData))
			ui.PrintSectionEnd()

			ui.PrintStatus("info", "Use 'catops [command] --help' for detailed help")
			ui.PrintStatus("info", "💬 Need help? Telegram: @mfhonley")
			return nil
		},
	}

	// add version flag
	rootCmd.Flags().BoolP("version", "v", false, "Show version information")

	// Create all commands using commands package
	statusCmd := commands.NewStatusCmd()
	processesCmd := commands.NewProcessesCmd()
	restartCmd := commands.NewRestartCmd()
	updateCmd := commands.NewUpdateCmd()
	startCmd := commands.NewStartCmd()
	setCmd := commands.NewSetCmd()
	daemonCmd := commands.NewDaemonCmd()
	uninstallCmd := commands.NewUninstallCmd()
	cleanupCmd := commands.NewCleanupCmd()
	forceCleanupCmd := commands.NewForceCleanupCmd()
	configCmd := commands.NewConfigCmd()
	autostartCmd := commands.NewAutostartCmd()
	authCmd := commands.NewAuthCmd()

	// add commands to root
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(processesCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(forceCleanupCmd)
	rootCmd.AddCommand(autostartCmd)
	rootCmd.AddCommand(authCmd)

	// execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
