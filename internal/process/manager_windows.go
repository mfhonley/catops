//go:build windows
// +build windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	constants "catops/config"
	"golang.org/x/sys/windows"
)

// ReadPID reads the PID from the PID file
func ReadPID() (int, error) {
	data, err := os.ReadFile(constants.PID_FILE)
	if err != nil {
		return 0, err
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, err
	}

	return pid, nil
}

// IsRunning checks if the monitoring process is running on Windows
func IsRunning() bool {
	pid, err := ReadPID()
	if err != nil {
		return false
	}

	// Windows: Check if process exists using OpenProcess
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	// Check if process is actually running (not zombie)
	var exitCode uint32
	err = windows.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}

	// STILL_ACTIVE = 259
	return exitCode == 259
}

// StopProcess stops the monitoring process on Windows
func StopProcess() error {
	if !IsRunning() {
		return fmt.Errorf("catops is not running")
	}

	pid, err := ReadPID()
	if err != nil {
		return err
	}

	// Windows: Use taskkill for graceful shutdown
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
	err = cmd.Run()
	if err != nil {
		// If graceful shutdown fails, force kill
		cmd = exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to stop process: %w", err)
		}
	}

	// Wait for process to stop
	time.Sleep(2 * time.Second)

	// Remove PID file
	os.Remove(constants.PID_FILE)

	return nil
}

// KillDuplicateProcesses kills duplicate catops daemon processes on Windows
func KillDuplicateProcesses() {
	// Windows: Use WMIC to find processes
	cmd := exec.Command("wmic", "process", "where", "name='catops.exe'", "get", "ProcessId")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	var pids []int

	for i, line := range lines {
		// Skip header line
		if i == 0 || strings.TrimSpace(line) == "" || strings.Contains(line, "ProcessId") {
			continue
		}

		pidStr := strings.TrimSpace(line)
		if pidStr == "" {
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		pids = append(pids, pid)
	}

	// Keep first process, kill duplicates
	if len(pids) > 1 {
		for i := 1; i < len(pids); i++ {
			cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pids[i]))
			cmd.Run() // Ignore errors
		}
	}
}

// CleanupZombieProcesses - not needed on Windows
func CleanupZombieProcesses() {
	// Windows doesn't have zombie processes like Unix
	// This function is here for API compatibility
	return
}

// KillAllCatOpsProcesses kills all catops processes on Windows
func KillAllCatOpsProcesses() {
	// Windows: Use taskkill with image name
	cmd := exec.Command("taskkill", "/F", "/IM", "catops.exe")
	cmd.Run() // Ignore errors

	// Remove PID file
	os.Remove(constants.PID_FILE)
}

// StartProcess starts the monitoring process in background on Windows
func StartProcess() error {
	// Clean up any existing issues first
	KillDuplicateProcesses()

	if IsRunning() {
		return fmt.Errorf("catops is already running")
	}

	// Get executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Windows: Use cmd /C start /B to run in background
	cmd := exec.Command(exePath, "daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	// Detach from parent process
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Detach the process
	err = cmd.Process.Release()
	if err != nil {
		return fmt.Errorf("failed to detach process: %w", err)
	}

	return nil
}

// RestartProcess restarts the monitoring process on Windows
func RestartProcess() error {
	// Kill any duplicate processes before restarting
	KillDuplicateProcesses()

	// Stop if running
	if IsRunning() {
		err := StopProcess()
		if err != nil {
			return err
		}
	}

	// Wait a bit to ensure process is fully stopped
	time.Sleep(1 * time.Second)

	// Start new process
	return StartProcess()
}
