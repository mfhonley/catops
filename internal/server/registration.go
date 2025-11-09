package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	constants "catops/config"
	"catops/internal/config"
	"catops/internal/logger"
	"catops/internal/metrics"
	"catops/pkg/utils"
)

// RegisterServer registers the server with the backend
func RegisterServer(userToken, currentVersion string, cfg *config.Config) bool {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	var osName string
	systemMetrics, err := metrics.GetMetrics()
	if err != nil {
		osName = runtime.GOOS // Fallback
	} else {
		osName = systemMetrics.OSName
	}

	// determine platform
	platform := runtime.GOOS
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		// Keep original arch value for other architectures
	}

	// Get server specifications
	serverSpecs, err := metrics.GetServerSpecs()
	if err != nil {
		// Log error but continue with default values
		logger.Warning("Could not get server specs: %v", err)
		// Set default values
		serverSpecs = map[string]interface{}{
			"cpu_cores":     0,
			"total_memory":  0,
			"total_storage": 0,
		}
	}

	serverData := map[string]interface{}{
		"platform":     platform, // remove "-" + arch
		"architecture": arch,
		"type":         "install",
		"timestamp":    fmt.Sprintf("%d", time.Now().Unix()), // string with Unix timestamp like in install.sh
		"user_token":   userToken,
		"server_info": map[string]string{
			"hostname":       hostname,
			"os_type":        osName,
			"os_version":     runtime.GOOS + "/" + runtime.GOARCH, // Add OS version info
			"catops_version": currentVersion,
		},
		// Add server specifications
		"cpu_cores":     serverSpecs["cpu_cores"],
		"total_memory":  serverSpecs["total_memory"],
		"total_storage": serverSpecs["total_storage"],
	}

	jsonData, _ := json.Marshal(serverData)

	// Debug: Log what we're sending
	logger.Debug("JSON data: %s", string(jsonData))

	// Debug: Log pretty JSON for better readability
	prettyJSON, _ := json.MarshalIndent(serverData, "", "  ")
	logger.Debug("Pretty JSON:\n%s", string(prettyJSON))

	// Debug: Log HTTP request details
	logger.Debug("Sending to URL: %s", constants.INSTALL_URL)
	logger.Debug("Request method: POST")

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

	// read response body
	bodyBytes, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return false
	}

	if result["success"] == true {
		// Extract user_token and server_id from response
		// New backend returns: data.user_token and data.server_id

		if data, ok := result["data"].(map[string]interface{}); ok {
			// Extract permanent user_token (replaces server_token)
			if userToken, ok := data["user_token"].(string); ok && userToken != "" {
				cfg.AuthToken = userToken // Store permanent user_token as AuthToken
				logger.Info("Permanent user_token received and saved")
			}

			// Extract server_id (MongoDB ObjectId)
			if serverID, ok := data["server_id"].(string); ok && serverID != "" {
				cfg.ServerID = serverID
				logger.Info("Server ID received and saved: %s", serverID)
			}

			// Log successful registration
			logger.Success("Server registration completed")
		} else {
			// log that data section not found
			logger.Error("data section not found in response")
		}
		return true
	}

	return false
}

// SendUninstallNotification sends uninstall notification to backend
func SendUninstallNotification(authToken, serverID, currentVersion string) bool {
	// Get hostname for better server identification
	hostname, err := os.Hostname()
	if err != nil {
		logger.Warning("Could not get hostname: %v", err)
		hostname = "" // Backend will fall back to IP-based search
	}

	// ServerUninstallRequest format - timestamp, user_token, and hostname
	uninstallData := map[string]interface{}{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().Unix()),
		"user_token": authToken,
		"hostname":   hostname,
	}

	jsonData, _ := json.Marshal(uninstallData)

	// Debug logging
	logger.Debug("Uninstall request data: %s", string(jsonData))
	logger.Debug("Uninstall URL: %s", constants.UNINSTALL_URL)

	// create request
	req, err := utils.CreateCLIRequest("POST", constants.UNINSTALL_URL, bytes.NewBuffer(jsonData), currentVersion)
	if err != nil {
		// Debug logging for error
		logger.Error("Failed to create uninstall request: %v", err)
		return false
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Debug logging for HTTP error
		logger.Error("HTTP request failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	// log result
	if resp.StatusCode == 200 {
		logger.Info("Uninstall notification sent successfully")
	} else {
		logger.Warning("Uninstall notification failed with status %d", resp.StatusCode)
	}

	return resp.StatusCode == 200
}

// TransferServerOwnership transfers server ownership to a new user
func TransferServerOwnership(oldToken, newToken, serverID, currentVersion string) bool {
	// ServerOwnerChangeRequest format - no server_id needed, backend finds it via token
	changeData := map[string]interface{}{
		"timestamp":      fmt.Sprintf("%d", time.Now().Unix()),
		"old_user_token": oldToken,
		"new_user_token": newToken,
	}

	jsonData, _ := json.Marshal(changeData)

	req, err := utils.CreateCLIRequest("POST", constants.SERVERS_URL, bytes.NewBuffer(jsonData), currentVersion)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	return result["success"] == true
}
