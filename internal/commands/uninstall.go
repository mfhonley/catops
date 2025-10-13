package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"catops/internal/config"
	"catops/internal/process"
	"catops/internal/server"
	"catops/internal/ui"
)

// NewUninstallCmd creates the uninstall command
func NewUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Completely remove CatOps from the system",
		Long: `Completely remove CatOps from the system.

This command will:
• Stop the monitoring service
• Remove the binary from PATH
• Delete configuration files
• Remove autostart services
• Clean up all CatOps-related files

Examples:
  	catops uninstall        # Remove CatOps completely
  catops uninstall --yes  # Skip confirmation prompt`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Uninstall CatOps")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Continuing with uninstall without backend notification")
				cfg = &config.Config{} // Use empty config
			}

			// check if --yes flag is set
			skipConfirm := cmd.Flags().Lookup("yes").Changed

			if !skipConfirm {
				ui.PrintStatus("warning", "This will completely remove CatOps from your system!")
				ui.PrintStatus("warning", "This will completely remove CatOps from your system!")
				ui.PrintStatus("info", "All configuration and data will be lost.")

				fmt.Print("\nAre you sure you want to continue? (y/N): ")
				var response string
				fmt.Scanln(&response)

				if response != "y" && response != "Y" {
					ui.PrintStatus("info", "Uninstall cancelled")
					ui.PrintSectionEnd()
					return
				}
			}

			// send uninstall notification to backend if we have tokens
			ui.PrintStatus("debug", fmt.Sprintf("AuthToken present: %t, ServerID present: %t", cfg.AuthToken != "", cfg.ServerID != ""))
			backendNotified := false
			if cfg.AuthToken != "" && cfg.ServerID != "" {
				ui.PrintStatus("info", "Notifying backend about uninstall...")
				if server.SendUninstallNotification(cfg.AuthToken, cfg.ServerID, GetCurrentVersion()) {
					ui.PrintStatus("success", "Backend notified about uninstall")
					backendNotified = true
				} else {
					ui.PrintStatus("warning", "Could not notify backend (continuing with uninstall)")
				}
			} else {
				ui.PrintStatus("warning", "No auth token or server ID found - skipping backend notification")
			}

			// remove autostart services FIRST (before stopping service)
			switch runtime.GOOS {
			case "linux":
				homeDir, _ := os.UserHomeDir()
				systemdService := homeDir + "/.config/systemd/user/catops.service"
				if _, err := os.Stat(systemdService); err == nil {
					if err := exec.Command("systemctl", "--user", "disable", "catops.service").Run(); err != nil {
						ui.PrintStatus("warning", "Failed to disable systemd service (may already be disabled)")
					}
					if err := exec.Command("systemctl", "--user", "stop", "catops.service").Run(); err != nil {
						ui.PrintStatus("warning", "Failed to stop systemd service (may already be stopped)")
					}
					if err := os.Remove(systemdService); err != nil {
						ui.PrintStatus("warning", fmt.Sprintf("Failed to remove systemd service file: %v", err))
					}
				}
			case "darwin":
				homeDir, _ := os.UserHomeDir()
				launchAgent := homeDir + "/Library/LaunchAgents/com.catops.monitor.plist"
				if _, err := os.Stat(launchAgent); err == nil {
					if err := exec.Command("launchctl", "unload", launchAgent).Run(); err != nil {
						ui.PrintStatus("warning", "Failed to unload launchd service (may already be unloaded)")
					}
					if err := os.Remove(launchAgent); err != nil {
						ui.PrintStatus("warning", fmt.Sprintf("Failed to remove launchd plist: %v", err))
					}
				}
			}

			// remove configuration directory
			homeDir, err := os.UserHomeDir()
			if err != nil {
				ui.PrintStatus("error", "Could not determine home directory")
				homeDir = os.Getenv("HOME") // fallback
			}
			configDir := filepath.Join(homeDir, ".catops")
			if err := os.RemoveAll(configDir); err == nil {
				ui.PrintStatus("success", "Configuration directory removed: "+configDir)
			} else {
				ui.PrintStatus("warning", fmt.Sprintf("Could not remove configuration directory: %v", err))
			}

			// remove log files only if backend was notified successfully
			if backendNotified {
				logFiles := []string{
					"/tmp/catops.log",
					"/tmp/catops.pid",
				}

				for _, logFile := range logFiles {
					if _, err := os.Stat(logFile); err == nil {
						if err := os.Remove(logFile); err == nil {
							ui.PrintStatus("success", "Removed log file: "+logFile)
						}
					}
				}
			} else {
				ui.PrintStatus("info", "Keeping log files for debugging (backend not notified)")
			}

			// stop ALL catops processes (after removing config)
			process.KillAllCatOpsProcesses()
			ui.PrintStatus("success", "All processes stopped")

			// remove ALL CatOps binaries from PATH LAST
			binaryPaths := []string{}

			// Unix-like systems
			binaryPaths = append(binaryPaths,
				"/usr/local/bin/catops",
				"/usr/bin/catops",
				filepath.Join(homeDir, ".local", "bin", "catops"),
			)

			// also search for any other catops binaries in PATH
			pathSeparator := ":"
			pathDirs := strings.Split(os.Getenv("PATH"), pathSeparator)
			for _, dir := range pathDirs {
				if strings.Contains(dir, "catops") || strings.Contains(dir, ".local") || strings.Contains(dir, "bin") {
					binaryName := "catops"
					potentialPath := filepath.Join(dir, binaryName)
					if _, err := os.Stat(potentialPath); err == nil {
						binaryPaths = append(binaryPaths, potentialPath)
					}
				}
			}

			// remove all found binaries
			binaryRemoved := false
			for _, path := range binaryPaths {
				if _, err := os.Stat(path); err == nil {
					if err := os.Remove(path); err == nil {
						ui.PrintStatus("success", "Removed binary: "+path)
						binaryRemoved = true
					} else {
						ui.PrintStatus("warning", "Could not remove binary: "+path)
					}
				}
			}

			if !binaryRemoved {
				ui.PrintStatus("warning", "Could not find any CatOps binaries in standard locations")
			}

			ui.PrintStatus("success", "CatOps completely removed from the system")
			ui.PrintSectionEnd()
		},
	}

	// add --yes flag to uninstall command
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	return cmd
}
