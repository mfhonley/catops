package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	constants "catops/config"
	"catops/internal/analytics"
	"catops/internal/config"
	"catops/internal/logger"
	"catops/internal/metrics"
	"catops/internal/ui"
	"catops/pkg/utils"
)

// CheckServerVersion checks server version against latest version via API
func CheckServerVersion(authToken, currentVersion string) (string, string, bool, error) {
	// Create request to server version check endpoint
	req, err := utils.CreateCLIRequest("GET", constants.VERSIONS_BASE_URL+"/server-check?user_token="+authToken, nil, currentVersion)
	if err != nil {
		return "", "", false, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", false, err
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", false, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", "", false, err
	}

	// Extract version information directly (no success/data wrapper)
	serverVersion, _ := result["server_version"].(string)
	latestVersion, _ := result["latest_version"].(string)
	needsUpdate, _ := result["needs_update"].(bool)

	// Check if we got valid data
	if serverVersion == "" && latestVersion == "" {
		return "", "", false, fmt.Errorf("invalid response format")
	}

	return serverVersion, latestVersion, needsUpdate, nil
}

// CheckBasicUpdate performs basic update check without server version
func CheckBasicUpdate(currentVersion string) {
	ui.PrintStatus("info", "Checking for latest version...")

	// Get current version
	ui.PrintStatus("info", fmt.Sprintf("Current version: %s", currentVersion))

	// Check API for latest version
	req, err := utils.CreateCLIRequest("GET", constants.VERSIONS_URL, nil, currentVersion)
	if err != nil {
		ui.PrintStatus("warning", fmt.Sprintf("Failed to check latest version: %v", err))
		ui.PrintStatus("info", "Continuing with update script...")
		ExecuteUpdateScript(currentVersion)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		ui.PrintStatus("warning", fmt.Sprintf("Failed to check latest version: %v", err))
		ui.PrintStatus("info", "Continuing with update script...")
		ExecuteUpdateScript(currentVersion)
		return
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ui.PrintStatus("warning", fmt.Sprintf("Failed to read response: %v", err))
		ui.PrintStatus("info", "Continuing with update script...")
		ExecuteUpdateScript(currentVersion)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		ui.PrintStatus("warning", fmt.Sprintf("Failed to parse response: %v", err))
		ui.PrintStatus("info", "Continuing with update script...")
		ExecuteUpdateScript(currentVersion)
		return
	}

	// Extract latest version
	latestVersion, ok := result["version"].(string)
	if !ok || latestVersion == "" {
		ui.PrintStatus("warning", "Could not determine latest version")
		ui.PrintStatus("info", "Continuing with update script...")
		ExecuteUpdateScript(currentVersion)
		return
	}

	ui.PrintStatus("info", fmt.Sprintf("Latest version: %s", latestVersion))

	if currentVersion == latestVersion {
		ui.PrintStatus("success", "Already up to date!")
		ui.PrintSectionEnd()
		return
	}

	ui.PrintStatus("info", "Update available! Installing...")
	ui.PrintSectionEnd()
	ExecuteUpdateScript(currentVersion)
}

// ExecuteUpdateScript runs the update script
func ExecuteUpdateScript(currentVersion string) {
	updateCmd := exec.Command("bash", "-c", "CATOPS_CLI_MODE=1 curl -sfL "+constants.GET_CATOPS_URL+"/update.sh | bash")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr

	if err := updateCmd.Run(); err != nil {
		// don't treat any exit code as error (update.sh handles its own exit codes)
		return
	}

	// Migrate service file after update (fix for duplicate path bug)
	MigrateServiceFile()

	// Note: Version update is handled by daemon on restart (new CLI version will update on daemon start)
	// Send analytics event
	cfg, err := config.LoadConfig()
	if err == nil && cfg.IsCloudMode() {
		analytics.NewSender(cfg, currentVersion).SendEvent("update_installed")
	}
}

// UpdateServerVersion updates server version in database after update
func UpdateServerVersion(userToken, currentVersion string, cfg *config.Config) bool {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	var osName string
	systemMetrics, err := metrics.GetMetrics()
	if err != nil {
		osName = runtime.GOOS
	} else {
		osName = systemMetrics.OSName
	}

	platform := runtime.GOOS
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	}

	serverSpecs, err := metrics.GetServerSpecs()
	if err != nil {
		serverSpecs = map[string]interface{}{
			"cpu_cores":     0,
			"total_memory":  0,
			"total_storage": 0,
		}
	}

	serverData := map[string]interface{}{
		"platform":     platform,
		"architecture": arch,
		"type":         "update",
		"timestamp":    fmt.Sprintf("%d", time.Now().Unix()),
		"user_token":   userToken,
		"server_id":    cfg.ServerID, // Include server_id for exact server match during update
		"server_info": map[string]string{
			"hostname":       hostname,
			"os_type":        osName,
			"os_version":     runtime.GOOS + "/" + runtime.GOARCH,
			"catops_version": currentVersion,
		},
		"cpu_cores":     serverSpecs["cpu_cores"],
		"total_memory":  serverSpecs["total_memory"],
		"total_storage": serverSpecs["total_storage"],
	}

	jsonData, _ := json.Marshal(serverData)

	req, err := utils.CreateCLIRequest("POST", constants.INSTALL_URL, bytes.NewBuffer(jsonData), currentVersion)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		logger.Info("Server version updated to %s", currentVersion)
	} else {
		logger.Warning("Failed to update server version - Status: %d", resp.StatusCode)
	}

	return resp.StatusCode == 200
}

// MigrateServiceFile checks and fixes the systemd service file if it has the duplicate path bug
func MigrateServiceFile() {
	if runtime.GOOS != "linux" {
		return
	}

	servicePath := "/etc/systemd/system/catops.service"

	// Read current service file
	content, err := os.ReadFile(servicePath)
	if err != nil {
		return
	}

	contentStr := string(content)

	// Check for the duplicate path bug pattern
	if !strings.Contains(contentStr, "catops daemon") {
		return
	}

	// Look for duplicate path pattern
	lines := strings.Split(contentStr, "\n")
	needsFix := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "ExecStart=") {
			parts := strings.Fields(line)
			if len(parts) >= 3 && strings.Contains(parts[1], "catops") {
				needsFix = true
				break
			}
		}
	}

	if !needsFix {
		return
	}

	ui.PrintStatus("info", "Migrating service file...")

	// Fix using sed
	cmd := exec.Command("sed", "-i", `s|ExecStart=.*/catops .*/catops daemon|ExecStart=/root/.local/bin/catops daemon|`, servicePath)
	if err := cmd.Run(); err != nil {
		ui.PrintStatus("warning", "Failed to migrate service file")
		return
	}

	// Reload systemd
	cmd = exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		ui.PrintStatus("warning", "Failed to reload systemd")
		return
	}

	ui.PrintStatus("success", "Service file migrated")
}
