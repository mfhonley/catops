//go:build !windows
// +build !windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"catops/internal/logger"
)

// LockFile represents an exclusive lock on a PID file
type LockFile struct {
	path string
	fd   int
}

// getPIDFilePath returns the appropriate PID file path for the OS
// Variable (not function) to allow override in tests
var getPIDFilePath = func() string {
	if runtime.GOOS == "linux" {
		// Prefer XDG Runtime Dir (cleaned on logout, per-user)
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir != "" {
			return filepath.Join(runtimeDir, "catops.pid")
		}

		// Fallback: user's home directory
		home, err := os.UserHomeDir()
		if err == nil {
			runDir := filepath.Join(home, ".local", "run")
			os.MkdirAll(runDir, 0755)
			return filepath.Join(runDir, "catops.pid")
		}

		// Last resort: /tmp (but per-user)
		return fmt.Sprintf("/tmp/catops-%d.pid", os.Getuid())
	}

	// macOS: use Application Support
	home, err := os.UserHomeDir()
	if err == nil {
		appSupport := filepath.Join(home, "Library", "Application Support", "catops")
		os.MkdirAll(appSupport, 0755)
		return filepath.Join(appSupport, "catops.pid")
	}

	// Fallback
	return "/tmp/catops.pid"
}

// Acquire creates and locks PID file atomically
// Returns error if another instance is already running
func Acquire() (*LockFile, error) {
	pidFile := getPIDFilePath()

	// Ensure directory exists
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create PID directory: %w", err)
	}

	// Open file (create if not exists, but don't truncate yet)
	fd, err := syscall.Open(
		pidFile,
		syscall.O_RDWR|syscall.O_CREAT,
		0644,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open PID file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	// If another instance is running, this will fail immediately
	err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		syscall.Close(fd)

		// Check if it's a stale lock (process doesn't exist)
		if isStale, stalePID := checkStaleLock(pidFile); isStale {
			logger.Info("Cleaning up stale PID file (process %d no longer exists)", stalePID)
			os.Remove(pidFile)
			// Retry acquisition
			return Acquire()
		}

		return nil, fmt.Errorf("another CatOps instance is already running")
	}

	// Successfully locked! Now write our PID
	// Truncate file first to clear old PID
	if err := syscall.Ftruncate(fd, 0); err != nil {
		syscall.Flock(fd, syscall.LOCK_UN)
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to truncate PID file: %w", err)
	}

	// Write current PID
	pid := fmt.Sprintf("%d\n", os.Getpid())
	if _, err := syscall.Write(fd, []byte(pid)); err != nil {
		syscall.Flock(fd, syscall.LOCK_UN)
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to write PID: %w", err)
	}

	logger.Info("Acquired PID file lock: %s (PID: %d)", pidFile, os.Getpid())

	// Keep fd open to maintain lock
	return &LockFile{
		path: pidFile,
		fd:   fd,
	}, nil
}

// Release releases the lock and removes PID file
func (lf *LockFile) Release() error {
	if lf.fd <= 0 {
		return nil
	}

	logger.Info("Releasing PID file lock: %s", lf.path)

	// Unlock and close
	syscall.Flock(lf.fd, syscall.LOCK_UN)
	syscall.Close(lf.fd)

	// Remove PID file
	os.Remove(lf.path)

	lf.fd = 0
	return nil
}

// Check checks if another instance is running
// Returns: (isRunning bool, pid int, error)
func Check() (bool, int, error) {
	pidFile := getPIDFilePath()

	// Try to open PID file
	fd, err := syscall.Open(pidFile, syscall.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil // No PID file = not running
		}
		return false, 0, fmt.Errorf("failed to open PID file: %w", err)
	}
	defer syscall.Close(fd)

	// Try to acquire lock (non-blocking)
	err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		// Failed to lock = another instance is running
		pid := readPIDFromFd(fd)
		return true, pid, nil
	}

	// Successfully locked = stale PID file (no process holding lock)
	syscall.Flock(fd, syscall.LOCK_UN)
	return false, 0, nil
}

// checkStaleLock checks if PID file is stale (process no longer exists)
// Returns: (isStale bool, pid int)
func checkStaleLock(pidFile string) (bool, int) {
	fd, err := syscall.Open(pidFile, syscall.O_RDONLY, 0)
	if err != nil {
		return false, 0
	}
	defer syscall.Close(fd)

	// Try to lock
	err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		// Locked by another process - not stale
		return false, 0
	}

	// Lock succeeded = stale
	syscall.Flock(fd, syscall.LOCK_UN)

	// Read PID for logging
	pid := readPIDFromFd(fd)
	return true, pid
}

// readPIDFromFd reads PID from file descriptor
func readPIDFromFd(fd int) int {
	buf := make([]byte, 32)
	n, err := syscall.Read(fd, buf)
	if err != nil || n == 0 {
		return 0
	}

	var pid int
	fmt.Sscanf(string(buf[:n]), "%d", &pid)
	return pid
}

// IsCatOpsProcess verifies that the given PID is actually a catops daemon process
// This prevents false positives from PID reuse
func IsCatOpsProcess(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Read process command line
	var cmdline string

	if runtime.GOOS == "linux" {
		// Linux: read /proc/{pid}/cmdline
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			return false
		}
		// Replace null bytes with spaces
		cmdline = strings.ReplaceAll(string(data), "\x00", " ")
	} else {
		// macOS: use ps command via exec.Command
		cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "command=")
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		cmdline = string(output)
	}

	// Check if command line contains "catops" and "daemon"
	cmdline = strings.ToLower(cmdline)
	return strings.Contains(cmdline, "catops") &&
		strings.Contains(cmdline, "daemon")
}

// CleanupStale removes stale PID file if process doesn't exist
func CleanupStale() error {
	pidFile := getPIDFilePath()

	running, pid, err := Check()
	if err != nil {
		return err
	}

	if !running {
		// PID file doesn't exist or is already stale
		os.Remove(pidFile)
		return nil
	}

	// Process is running - verify it's actually catops
	if !IsCatOpsProcess(pid) {
		// PID was reused by another process - safe to remove
		logger.Info("PID file contains PID of non-catops process (%d), cleaning up", pid)
		os.Remove(pidFile)
		return nil
	}

	// Valid catops process is running
	return fmt.Errorf("catops daemon is running (PID %d)", pid)
}
