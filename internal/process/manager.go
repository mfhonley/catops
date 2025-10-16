//go:build !windows
// +build !windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"catops/internal/logger"
)

// Global lock file reference (held by daemon process)
var globalLock *LockFile

// IsRunning checks if the monitoring process is running
// Uses lockfile-based detection (much more reliable than old PID-only check)
func IsRunning() bool {
	running, pid, err := Check()
	if err != nil {
		logger.Error("Failed to check if running: %v", err)
		return false
	}

	if !running {
		return false
	}

	// Verify it's actually a catops process (not PID reuse)
	if !IsCatOpsProcess(pid) {
		logger.Warning("PID file exists but process %d is not catops, cleaning up", pid)
		CleanupStale()
		return false
	}

	return true
}

// StopProcess stops the monitoring process gracefully
func StopProcess() error {
	if !IsRunning() {
		return fmt.Errorf("catops is not running")
	}

	running, pid, err := Check()
	if err != nil {
		return err
	}

	if !running {
		return fmt.Errorf("catops is not running")
	}

	logger.Info("Stopping catops daemon (PID: %d)", pid)

	// Send SIGTERM for graceful shutdown
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for graceful shutdown (max 5 seconds)
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !IsRunning() {
			logger.Info("Daemon stopped successfully")
			return nil
		}
	}

	// If still running after 5 seconds, force kill
	logger.Warning("Daemon did not stop gracefully, sending SIGKILL")
	process.Signal(syscall.SIGKILL)
	time.Sleep(500 * time.Millisecond)

	// Cleanup any remaining lock file
	CleanupStale()

	return nil
}

// StartProcess starts the monitoring process in background
func StartProcess() error {
	// Check if already running (lockfile-based check)
	if IsRunning() {
		return fmt.Errorf("catops is already running")
	}

	// Clean up any stale lock files
	CleanupStale()

	logger.Info("Starting catops daemon in background")

	// Start the process in background using nohup
	// The daemon itself will acquire the lockfile
	cmd := exec.Command("bash", "-c", "nohup catops daemon > /dev/null 2>&1 &")
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait a moment for daemon to start and acquire lock
	time.Sleep(500 * time.Millisecond)

	// Verify it started successfully
	if !IsRunning() {
		return fmt.Errorf("daemon failed to start (check logs)")
	}

	logger.Info("Daemon started successfully")
	return nil
}

// RestartProcess stops and starts the monitoring process
func RestartProcess() error {
	logger.Info("Restarting catops daemon")

	// Stop if running
	if IsRunning() {
		err := StopProcess()
		if err != nil {
			return fmt.Errorf("failed to stop daemon: %w", err)
		}
	}

	// Wait a moment before restarting
	time.Sleep(1 * time.Second)

	// Start new process
	err := StartProcess()
	if err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	return nil
}

// AcquireLock acquires the global lock file (called by daemon on startup)
// This MUST be called as the first thing in daemon.go main()
func AcquireLock() (*LockFile, error) {
	lock, err := Acquire()
	if err != nil {
		return nil, err
	}

	globalLock = lock
	return lock, nil
}

// ReleaseLock releases the global lock file (called by daemon on shutdown)
func ReleaseLock() {
	if globalLock != nil {
		globalLock.Release()
		globalLock = nil
	}
}

// GetPID returns the PID of the running daemon (if any)
func GetPID() (int, error) {
	running, pid, err := Check()
	if err != nil {
		return 0, err
	}

	if !running {
		return 0, fmt.Errorf("daemon is not running")
	}

	return pid, nil
}

// KillAll forcefully kills all catops daemon processes (emergency cleanup)
// This is a last resort - should only be used for manual cleanup
func KillAll() error {
	logger.Warning("Emergency cleanup: killing all catops daemon processes")

	// Use pkill to kill all catops daemon processes
	cmd := exec.Command("pkill", "-9", "-f", "catops daemon")
	err := cmd.Run()
	if err != nil {
		// pkill returns error if no processes found, which is fine
		logger.Info("No catops daemon processes found to kill")
	}

	// Clean up lock file
	CleanupStale()

	return nil
}
