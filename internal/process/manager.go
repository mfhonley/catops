package process

import (
	"fmt"
	"os"
	"os/exec"
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

// StartProcess starts the monitoring process in background
func StartProcess() error {
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
