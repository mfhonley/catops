package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	constants "catops/config"
	"catops/internal/ui"
)

// Enable enables autostart on boot
func Enable(executable string) {
	switch runtime.GOOS {
	case "linux":
		// create systemd user service
		homeDir, _ := os.UserHomeDir()
		systemdDir := homeDir + "/.config/systemd/user"
		os.MkdirAll(systemdDir, 0755)

		serviceContent := fmt.Sprintf(`[Unit]
Description=CatOps System Monitor
After=network.target

[Service]
Type=forking
ExecStart=%s start
Restart=on-failure
RestartSec=30
TimeoutStopSec=15
Environment=PATH=%s:/usr/local/bin:/usr/bin:/bin

# Prevent startup if already running
ExecStartPre=/bin/sh -c 'pgrep -f "catops daemon" && exit 1 || exit 0'

[Install]
WantedBy=default.target`, executable, executable[:len(executable)-len("/catops")])

		serviceFile := systemdDir + "/catops.service"
		if err := os.WriteFile(serviceFile, []byte(serviceContent), 0644); err != nil {
			ui.PrintStatus("error", "Failed to create systemd service file")
			return
		}

		// enable and start service
		exec.Command("systemctl", "--user", "daemon-reload").Run()
		exec.Command("systemctl", "--user", "enable", "catops.service").Run()
		exec.Command("systemctl", "--user", "start", "catops.service").Run()

		ui.PrintStatus("success", "Systemd service created and enabled")
		ui.PrintStatus("info", "CatOps will start automatically on boot")

	case "darwin":
		// create launchd service
		homeDir, _ := os.UserHomeDir()
		launchAgentsDir := homeDir + "/Library/LaunchAgents"
		os.MkdirAll(launchAgentsDir, 0755)

		plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.catops.monitor</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>start</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
	<key>ThrottleInterval</key>
	<integer>30</integer>
</dict>
</plist>`, executable, constants.LOG_FILE, constants.LOG_FILE)

		plistFile := launchAgentsDir + "/com.catops.monitor.plist"
		if err := os.WriteFile(plistFile, []byte(plistContent), 0644); err != nil {
			ui.PrintStatus("error", "Failed to create launchd plist file")
			return
		}

		// load the service
		exec.Command("launchctl", "load", plistFile).Run()

		ui.PrintStatus("success", "Launchd service created and enabled")
		ui.PrintStatus("info", "CatOps will start automatically on boot")

	default:
		ui.PrintStatus("error", "Autostart not supported on this operating system")
	}
}

// Disable disables autostart on boot
func Disable() {
	switch runtime.GOOS {
	case "linux":
		// disable systemd service (without stopping to avoid duplicate Telegram messages)
		exec.Command("systemctl", "--user", "disable", "catops.service").Run()

		// remove service file
		homeDir, _ := os.UserHomeDir()
		serviceFile := homeDir + "/.config/systemd/user/catops.service"
		os.Remove(serviceFile)

		ui.PrintStatus("success", "Systemd service disabled and removed")

	case "darwin":
		// unload launchd service
		exec.Command("launchctl", "unload", "~/Library/LaunchAgents/com.catops.monitor.plist").Run()

		// remove plist file
		homeDir, _ := os.UserHomeDir()
		plistFile := homeDir + "/Library/LaunchAgents/com.catops.monitor.plist"
		os.Remove(plistFile)

		ui.PrintStatus("success", "Launchd service disabled and removed")

	default:
		ui.PrintStatus("error", "Autostart not supported on this operating system")
	}
}

// CheckStatus checks if autostart is enabled
func CheckStatus() {
	switch runtime.GOOS {
	case "linux":
		// check systemd service status
		cmd := exec.Command("systemctl", "--user", "is-enabled", "catops.service")
		if err := cmd.Run(); err == nil {
			ui.PrintStatus("success", "Autostart is enabled (systemd)")
		} else {
			ui.PrintStatus("info", "Autostart is disabled (systemd)")
		}

	case "darwin":
		// check launchd service status
		homeDir, _ := os.UserHomeDir()
		plistFile := homeDir + "/Library/LaunchAgents/com.catops.monitor.plist"
		if _, err := os.Stat(plistFile); err == nil {
			ui.PrintStatus("success", "Autostart is enabled (launchd)")
		} else {
			ui.PrintStatus("info", "Autostart is disabled (launchd)")
		}

	default:
		ui.PrintStatus("error", "Autostart not supported on this operating system")
	}
}
