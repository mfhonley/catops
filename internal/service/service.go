//go:build !windows
// +build !windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/okzk/sdnotify"
	"github.com/takama/daemon"

	"catops/internal/logger"
)

const (
	serviceName        = "catops"
	serviceDescription = "CatOps System Monitor - Lightweight server monitoring"
)

// Service wraps takama/daemon for cross-platform service management
type Service struct {
	daemon daemon.Daemon
}

// New creates a new Service instance
func New() (*Service, error) {
	// Use SystemDaemon for system-wide service (requires root)
	// Use UserAgent for user-level service (no root needed)
	kind := daemon.UserAgent
	if os.Geteuid() == 0 {
		kind = daemon.SystemDaemon
	}

	d, err := daemon.New(serviceName, serviceDescription, kind)
	if err != nil {
		return nil, fmt.Errorf("failed to create daemon: %w", err)
	}

	return &Service{daemon: d}, nil
}

// Install installs the service (creates systemd/launchd config)
func (s *Service) Install() (string, error) {
	// Install with "daemon" argument only
	// Note: takama/daemon automatically determines the executable path,
	// we only need to pass the command arguments
	status, err := s.daemon.Install("daemon")
	if err != nil {
		return status, err
	}

	logger.Info("Service installed: %s", status)
	return status, nil
}

// Remove removes the service
func (s *Service) Remove() (string, error) {
	status, err := s.daemon.Remove()
	if err != nil {
		return status, err
	}

	logger.Info("Service removed: %s", status)
	return status, nil
}

// Start starts the service
func (s *Service) Start() (string, error) {
	status, err := s.daemon.Start()
	if err != nil {
		return status, err
	}

	logger.Info("Service started: %s", status)
	return status, nil
}

// Stop stops the service
func (s *Service) Stop() (string, error) {
	status, err := s.daemon.Stop()
	if err != nil {
		return status, err
	}

	logger.Info("Service stopped: %s", status)
	return status, nil
}

// Status returns the service status
func (s *Service) Status() (string, error) {
	return s.daemon.Status()
}

// IsRunning checks if the service is currently running
func (s *Service) IsRunning() bool {
	status, err := s.daemon.Status()
	if err != nil {
		return false
	}
	// takama/daemon returns "running" in status string when service is active
	return status != "" && err == nil
}

// NotifyReady notifies systemd that service is ready (Type=notify)
func NotifyReady() {
	if runtime.GOOS == "linux" {
		sdnotify.Ready()
		logger.Debug("Sent READY notification to systemd")
	}
}

// NotifyStopping notifies systemd that service is stopping
func NotifyStopping() {
	if runtime.GOOS == "linux" {
		sdnotify.Stopping()
		logger.Debug("Sent STOPPING notification to systemd")
	}
}

// NotifyWatchdog sends watchdog ping to systemd
func NotifyWatchdog() {
	if runtime.GOOS == "linux" {
		sdnotify.Watchdog()
	}
}

// NotifyStatus sends status message to systemd
func NotifyStatus(status string) {
	if runtime.GOOS == "linux" {
		sdnotify.Status(status)
	}
}

// MigrateServiceFile checks and fixes the systemd service file if it has the duplicate path bug
// This is needed for systems that installed catops before the fix
func MigrateServiceFile() {
	if runtime.GOOS != "linux" {
		return
	}

	servicePath := "/etc/systemd/system/catops.service"

	// Read current service file
	content, err := os.ReadFile(servicePath)
	if err != nil {
		// Service file doesn't exist or can't be read, nothing to migrate
		return
	}

	contentStr := string(content)

	// Check for the duplicate path bug pattern: ExecStart=/path/catops /path/catops daemon
	// The bug was: s.daemon.Install(executable, "daemon") which passed the path twice
	if !strings.Contains(contentStr, "catops daemon") {
		return // No daemon argument found, might be a different issue
	}

	// Look for duplicate path pattern
	lines := strings.Split(contentStr, "\n")
	needsFix := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "ExecStart=") {
			// Count how many times the path appears
			// Bug pattern: ExecStart=/root/.local/bin/catops /root/.local/bin/catops daemon
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				// Check if second part looks like a path (contains catops)
				if strings.Contains(parts[1], "catops") {
					needsFix = true
					break
				}
			}
		}
	}

	if !needsFix {
		return
	}

	logger.Info("Detected outdated service file, migrating...")

	// Fix using sed (safer than rewriting the whole file)
	cmd := exec.Command("sed", "-i", `s|ExecStart=.*/catops .*/catops daemon|ExecStart=/root/.local/bin/catops daemon|`, servicePath)
	if err := cmd.Run(); err != nil {
		logger.Warning("Failed to migrate service file: %v", err)
		return
	}

	// Reload systemd
	cmd = exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		logger.Warning("Failed to reload systemd: %v", err)
		return
	}

	logger.Info("Service file migrated successfully")
}
