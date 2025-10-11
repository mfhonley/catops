package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// createWindowsTask creates Windows Task Scheduler task for autostart
func createWindowsTask(executable string) error {
	taskName := "CatOpsMonitor"

	// Create XML task definition
	taskXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>CatOps System Monitor - Automatically monitors system resources</Description>
    <Author>CatOps</Author>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions>
    <Exec>
      <Command>%s</Command>
      <Arguments>daemon</Arguments>
    </Exec>
  </Actions>
</Task>`, executable)

	// Write XML to temp file
	tempFile := filepath.Join(os.TempDir(), "catops_task.xml")
	err := os.WriteFile(tempFile, []byte(taskXML), 0644)
	if err != nil {
		return fmt.Errorf("failed to write task XML: %w", err)
	}
	defer os.Remove(tempFile)

	// Create task using schtasks
	cmd := exec.Command("schtasks", "/Create", "/TN", taskName, "/XML", tempFile, "/F")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create task: %s - %w", string(output), err)
	}

	return nil
}

// removeWindowsTask removes Windows Task Scheduler task
func removeWindowsTask() error {
	taskName := "CatOpsMonitor"

	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete task: %s - %w", string(output), err)
	}

	return nil
}
