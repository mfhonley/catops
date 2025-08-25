package process

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	constants "moniq/config"
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

// IsRunning checks if the monitoring process is running
func IsRunning() bool {
	pid, err := ReadPID()
	if err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process is alive
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// StopProcess stops the monitoring process
func StopProcess() error {
	if !IsRunning() {
		return fmt.Errorf("moniq is not running")
	}

	pid, err := ReadPID()
	if err != nil {
		return err
	}

	// Kill the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	err = process.Kill()
	if err != nil {
		return err
	}

	// Remove PID file
	os.Remove(constants.PID_FILE)

	return nil
}

// KillDuplicateProcesses kills any duplicate moniq daemon processes
func KillDuplicateProcesses() {
	// Find all moniq daemon processes
	cmd := exec.Command("pgrep", "-f", "moniq daemon")
	output, err := cmd.Output()
	if err != nil {
		// No processes found, which is fine
		return
	}

	// Parse PIDs
	pids := strings.Fields(string(output))
	if len(pids) <= 1 {
		// Only one or no processes, which is fine
		return
	}

	// Kill all but the first process (keep one)
	for i := 1; i < len(pids); i++ {
		pid, err := strconv.Atoi(pids[i])
		if err != nil {
			continue
		}

		// Kill the duplicate process
		process, err := os.FindProcess(pid)
		if err == nil {
			process.Kill()
		}
	}
}

// CleanupZombieProcesses cleans up any zombie moniq processes
func CleanupZombieProcesses() {
	// Find zombie processes more efficiently
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		// macOS doesn't support --no-headers
		cmd = exec.Command("ps", "-eo", "pid,state,comm")
	} else {
		cmd = exec.Command("ps", "-eo", "pid,state,comm", "--no-headers")
	}

	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		// Skip header on macOS
		if runtime.GOOS == "darwin" && i == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 {
			// Check if it's a zombie process and moniq-related
			if fields[1] == "Z" && (strings.Contains(fields[2], "moniq") || strings.Contains(fields[2], "[moniq]")) {
				pid, err := strconv.Atoi(fields[0])
				if err == nil {
					// Try to kill zombie process
					process, err := os.FindProcess(pid)
					if err == nil {
						process.Kill()
					}
				}
			}
		}
	}
}

// KillAllMoniqProcesses kills ALL moniq daemon processes for complete cleanup
func KillAllMoniqProcesses() {
	// Kill all moniq daemon processes
	killCmd := exec.Command("pkill", "-f", "moniq daemon")
	killCmd.Run() // Ignore errors

	// Clean up zombie processes
	CleanupZombieProcesses()

	// Remove PID file
	os.Remove(constants.PID_FILE)
}

// StartProcess starts the monitoring process in background
func StartProcess() error {
	// Clean up any existing issues first
	KillDuplicateProcesses()
	CleanupZombieProcesses()

	if IsRunning() {
		return fmt.Errorf("moniq is already running")
	}

	// Start the process in background using shell
	cmd := exec.Command("bash", "-c", "nohup moniq daemon > /dev/null 2>&1 &")
	err := cmd.Start()
	if err != nil {
		return err
	}

	return nil
}

// RestartProcess stops and starts the monitoring process
func RestartProcess() error {
	// Kill any duplicate processes before stopping
	KillDuplicateProcesses()

	// Stop if running
	if IsRunning() {
		err := StopProcess()
		if err != nil {
			return err
		}
	}

	// Start new process
	return StartProcess()
}
