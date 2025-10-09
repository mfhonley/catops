package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	constants "catops/config"
	"catops/internal/config"
	"catops/internal/metrics"
	"catops/internal/process"
	"catops/internal/telegram"
	"catops/internal/ui"
	"catops/pkg/utils"
)

// VERSION is set during build via ldflags
var VERSION string

// Helper functions for server operations

// sendAllAnalytics sends all analytics (metrics, processes, events) synchronously with WaitGroup
func sendAllAnalytics(cfg *config.Config, eventType string, currentMetrics *metrics.Metrics) {
	if cfg.AuthToken == "" || cfg.ServerID == "" {
		return // Skip if not in cloud mode
	}

	var wg sync.WaitGroup

	// Send service analytics
	if eventType != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sendServiceAnalytics(cfg, eventType, currentMetrics)
		}()
	}

	// Send metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendMetrics(cfg, currentMetrics)
	}()

	// Send process metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendProcessMetrics(cfg, currentMetrics)
	}()

	// Wait for all goroutines to complete (with timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait max 10 seconds for all analytics to be sent
	select {
	case <-done:
		// All done successfully
	case <-time.After(10 * time.Second):
		// Timeout - log but don't block
		if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer logFile.Close()
			logFile.WriteString(fmt.Sprintf("[%s] WARNING: Analytics send timeout after 10 seconds\n",
				time.Now().Format("2006-01-02 15:04:05")))
		}
	}
}

// getCurrentVersion retrieves the current version from build flags or version.txt
func getCurrentVersion() string {
	version := VERSION
	if version == "" {
		// Read version from version.txt if VERSION is not set
		if versionData, err := os.ReadFile("version.txt"); err == nil {
			version = strings.TrimSpace(string(versionData))
		}
	}
	return version
}

// checkServerVersion checks server version against latest version via API
func checkServerVersion(authToken string) (string, string, bool, error) {
	// Create request to server version check endpoint
	req, err := utils.CreateCLIRequest("GET", constants.VERSIONS_BASE_URL+"/server-check?user_token="+authToken, nil, getCurrentVersion())
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

// checkBasicUpdate performs basic update check without server version
func checkBasicUpdate() {
	ui.PrintStatus("info", "Checking for latest version...")

	// Get current version
	currentVersion := getCurrentVersion()
	ui.PrintStatus("info", fmt.Sprintf("Current version: %s", currentVersion))

	// Check API for latest version
	req, err := utils.CreateCLIRequest("GET", constants.VERSIONS_URL, nil, getCurrentVersion())
	if err != nil {
		ui.PrintStatus("warning", fmt.Sprintf("Failed to check latest version: %v", err))
		ui.PrintStatus("info", "Continuing with update script...")
		executeUpdateScript()
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		ui.PrintStatus("warning", fmt.Sprintf("Failed to check latest version: %v", err))
		ui.PrintStatus("info", "Continuing with update script...")
		executeUpdateScript()
		return
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ui.PrintStatus("warning", fmt.Sprintf("Failed to read response: %v", err))
		ui.PrintStatus("info", "Continuing with update script...")
		executeUpdateScript()
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		ui.PrintStatus("warning", fmt.Sprintf("Failed to parse response: %v", err))
		ui.PrintStatus("info", "Continuing with update script...")
		executeUpdateScript()
		return
	}

	// Extract latest version
	latestVersion, ok := result["version"].(string)
	if !ok || latestVersion == "" {
		ui.PrintStatus("warning", "Could not determine latest version")
		ui.PrintStatus("info", "Continuing with update script...")
		executeUpdateScript()
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
	executeUpdateScript()
}

// executeUpdateScript runs the update script
func executeUpdateScript() {
	updateCmd := exec.Command("bash", "-c", "CATOPS_CLI_MODE=1 curl -sfL "+constants.GET_CATOPS_URL+"/update.sh | bash")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr

	if err := updateCmd.Run(); err != nil {
		// don't treat any exit code as error (update.sh handles its own exit codes)
		return
	}

	// Note: Version update is handled by daemon on restart (new CLI version will update on daemon start)
	// Send analytics event
	cfg, err := config.LoadConfig()
	if err == nil && cfg.IsCloudMode() {
		if currentMetrics, err := metrics.GetMetrics(); err == nil {
			sendAllAnalytics(cfg, "update_installed", currentMetrics)
		}
	}
}

// sendAlertAnalytics sends alert data to the backend for monitoring and analytics
func sendAlertAnalytics(cfg *config.Config, alerts []string, metrics *metrics.Metrics) {

	if cfg.AuthToken == "" || cfg.ServerID == "" {
		return // Don't send if token or server_id is missing
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Convert alert strings to AlertModel format
	alertModels := []map[string]interface{}{}

	for i, alertText := range alerts {
		// Generate unique alert ID (use UTC timestamp)
		alertID := fmt.Sprintf("alert_%s_%d_%d", cfg.ServerID, time.Now().UTC().Unix(), i)

		// Parse alert text to extract metric info
		var metricType, metricName string
		var currentValue, thresholdValue float64
		var level string

		// Parse alert patterns like "CPU: 95.0% (limit: 90.0%)"
		if strings.Contains(alertText, "CPU") {
			metricType = "system"
			metricName = "cpu_usage"
			currentValue = metrics.CPUUsage
			thresholdValue = cfg.CPUThreshold
		} else if strings.Contains(alertText, "Memory") {
			metricType = "system"
			metricName = "memory_usage"
			currentValue = metrics.MemoryUsage
			thresholdValue = cfg.MemThreshold
		} else if strings.Contains(alertText, "Disk") {
			metricType = "system"
			metricName = "disk_usage"
			currentValue = metrics.DiskUsage
			thresholdValue = cfg.DiskThreshold
		} else {
			// Default for unknown alerts
			metricType = "system"
			metricName = "unknown"
			currentValue = 0
			thresholdValue = 0
		}

		// Determine alert level based on severity
		if currentValue >= thresholdValue*1.5 {
			level = "critical"
		} else if currentValue >= thresholdValue*1.2 {
			level = "error"
		} else if currentValue >= thresholdValue {
			level = "warning"
		} else {
			level = "info"
		}

		alertModel := map[string]interface{}{
			"timestamp":          time.Now().UTC().Format("2006-01-02T15:04:05Z"),
			"server_id":          cfg.ServerID,
			"alert_id":           alertID,
			"metric_type":        metricType,
			"metric_name":        metricName,
			"level":              level,
			"status":             "active",
			"title":              fmt.Sprintf("%s Threshold Exceeded", strings.Title(metricName)),
			"message":            alertText,
			"current_value":      currentValue,
			"threshold_value":    thresholdValue,
			"threshold_operator": ">=",
			"resolved_at":        nil,
			"tags": map[string]string{
				"hostname":       hostname,
				"os_type":        metrics.OSName,
				"catops_version": getCurrentVersion(),
				"alert_type":     "threshold_exceeded",
			},
			"metadata": fmt.Sprintf(`{"process_analytics":{"total_processes":%d,"running_processes":%d,"sleeping_processes":%d,"zombie_processes":%d}}`,
				len(metrics.TopProcesses),
				countProcessesByStatus(metrics.TopProcesses, "R"),
				countProcessesByStatus(metrics.TopProcesses, "S"),
				countProcessesByStatus(metrics.TopProcesses, "Z")),
		}

		alertModels = append(alertModels, alertModel)
	}

	// Create AlertsBatchRequest format
	alertData := map[string]interface{}{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().Unix()),
		"user_token": cfg.AuthToken,
		"alerts":     alertModels,
	}

	jsonData, _ := json.Marshal(alertData)

	// Send analytics data to backend asynchronously
	go func() {
		// Логируем начало запроса аналитики
		if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer logFile.Close()
			logFile.WriteString(fmt.Sprintf("[%s] INFO: Analytics request started - Type: alert, URL: %s\n",
				time.Now().Format("2006-01-02 15:04:05"), constants.ANALYTICS_URL))
		}

		req, err := utils.CreateCLIRequest("POST", constants.ANALYTICS_URL, bytes.NewBuffer(jsonData), getCurrentVersion())
		if err != nil {
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// Логируем ошибку отправки аналитики
			if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer logFile.Close()
				logFile.WriteString(fmt.Sprintf("[%s] ERROR: Analytics failed - Type: alert, Error: %v\n",
					time.Now().Format("2006-01-02 15:04:05"), err))
			}
			return
		}
		defer resp.Body.Close()

		// Логируем успешную отправку аналитики
		if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer logFile.Close()
			logFile.WriteString(fmt.Sprintf("[%s] INFO: Analytics sent - Type: alert, Status: success\n",
				time.Now().Format("2006-01-02 15:04:05")))
		}

	}()
}

// sendServiceAnalytics sends service event data to the backend for monitoring and analytics
func sendServiceAnalytics(cfg *config.Config, eventType string, metrics *metrics.Metrics) {
	if cfg.AuthToken == "" || cfg.ServerID == "" {
		return // Don't send if token or server_id is missing
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Get process PID
	pid := os.Getpid()

	// Map CLI event types to backend EventType enum
	var backendEventType string
	switch eventType {
	case "service_start":
		backendEventType = "service_start"
	case "service_stop":
		backendEventType = "service_stop"
	case "system_monitoring":
		backendEventType = "system_monitoring"
	case "update_installed":
		backendEventType = "update_installed"
	case "config_change":
		backendEventType = "config_change"
	case "service_restart":
		backendEventType = "service_restart"
	default:
		backendEventType = "service_start"
	}

	// Determine severity based on event type
	var severity string
	switch eventType {
	case "service_start", "system_monitoring", "update_installed", "service_restart":
		severity = "info"
	case "service_stop", "config_change":
		severity = "warning"
	default:
		severity = "info"
	}

	// Create message for the event
	var message string
	switch eventType {
	case "service_start":
		message = "CatOps monitoring service started successfully"
	case "service_stop":
		message = "CatOps monitoring service stopped"
	case "system_monitoring":
		message = "CatOps monitoring service is running and collecting metrics"
	case "update_installed":
		message = "CatOps update installed successfully"
	case "config_change":
		message = "CatOps configuration changed"
	case "service_restart":
		message = "CatOps monitoring service restarted"
	default:
		message = fmt.Sprintf("CatOps service event: %s", eventType)
	}

	// Create single EventModel object
	eventModel := map[string]interface{}{
		"timestamp":     time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"server_id":     cfg.ServerID,
		"event_type":    backendEventType,
		"service_name":  "catops",
		"process_name":  "catops",
		"pid":           pid,
		"message":       message,
		"severity":      severity,
		"error_message": nil,
		"tags": map[string]string{
			"hostname":       hostname,
			"os_type":        metrics.OSName,
			"catops_version": getCurrentVersion(),
			"original_event": eventType,
		},
		"metadata": fmt.Sprintf(`{"process_analytics":{"total_processes":%d,"running_processes":%d,"sleeping_processes":%d,"zombie_processes":%d},"metrics":{"cpu_usage":%.2f,"memory_usage":%.2f,"disk_usage":%.2f}}`,
			len(metrics.TopProcesses),
			countProcessesByStatus(metrics.TopProcesses, "R"),
			countProcessesByStatus(metrics.TopProcesses, "S"),
			countProcessesByStatus(metrics.TopProcesses, "Z"),
			metrics.CPUUsage,
			metrics.MemoryUsage,
			metrics.DiskUsage),
	}

	// Create EventsBatchRequest format
	serviceData := map[string]interface{}{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().Unix()),
		"user_token": cfg.AuthToken,
		"events":     []map[string]interface{}{eventModel},
	}

	jsonData, _ := json.Marshal(serviceData)

	// send analytics to backend
	go func() {
		// log service analytics request start
		if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer logFile.Close()
			logFile.WriteString(fmt.Sprintf("[%s] INFO: Analytics request started - Type: service_%s, URL: %s\n",
				time.Now().Format("2006-01-02 15:04:05"), eventType, constants.EVENTS_URL))
		}

		req, err := utils.CreateCLIRequest("POST", constants.EVENTS_URL, bytes.NewBuffer(jsonData), getCurrentVersion())
		if err != nil {
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// log analytics send error
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] ERROR: Failed to send service analytics: %v\n",
					time.Now().Format("2006-01-02 15:04:05"), err))
			}
			return
		}
		defer resp.Body.Close()

		// log analytics success
		if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer logFile.Close()
			logFile.WriteString(fmt.Sprintf("[%s] INFO: Analytics sent - Type: service_%s, Status: success\n",
				time.Now().Format("2006-01-02 15:04:05"), eventType))
		}

	}()
}

// sendProcessMetrics sends process analytics to the backend processes endpoint
func sendProcessMetrics(cfg *config.Config, metrics *metrics.Metrics) {
	if cfg.AuthToken == "" || cfg.ServerID == "" {
		return // Don't send if token or server_id is missing
	}

	if len(metrics.TopProcesses) == 0 {
		return // No processes to send
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Get top 10 CPU processes
	topCPU := getTopProcessesByCPU(metrics.TopProcesses, 10)
	topCPUData := make([]map[string]interface{}, 0, len(topCPU))
	for i, proc := range topCPU {
		topCPUData = append(topCPUData, map[string]interface{}{
			"pid":           proc.PID,
			"name":          proc.Name,
			"cpu_usage":     proc.CPUUsage,
			"memory_usage":  proc.MemoryUsage,
			"memory_kb":     proc.MemoryKB,
			"command":       proc.Command,
			"user":          proc.User,
			"status":        proc.Status,
			"start_time":    proc.StartTime,
			"threads":       proc.Threads,
			"virtual_mem":   proc.VirtualMem,
			"resident_mem":  proc.ResidentMem,
			"tty":           proc.TTY,
			"cpu_num":       proc.CPU,
			"priority":      proc.Priority,
			"nice":          proc.Nice,
			"rank_type":     "cpu",
			"rank_position": i + 1,
		})
	}

	// Get top 10 Memory processes
	topMemory := getTopProcessesByMemory(metrics.TopProcesses, 10)
	topMemoryData := make([]map[string]interface{}, 0, len(topMemory))
	for i, proc := range topMemory {
		topMemoryData = append(topMemoryData, map[string]interface{}{
			"pid":           proc.PID,
			"name":          proc.Name,
			"cpu_usage":     proc.CPUUsage,
			"memory_usage":  proc.MemoryUsage,
			"memory_kb":     proc.MemoryKB,
			"command":       proc.Command,
			"user":          proc.User,
			"status":        proc.Status,
			"start_time":    proc.StartTime,
			"threads":       proc.Threads,
			"virtual_mem":   proc.VirtualMem,
			"resident_mem":  proc.ResidentMem,
			"tty":           proc.TTY,
			"cpu_num":       proc.CPU,
			"priority":      proc.Priority,
			"nice":          proc.Nice,
			"rank_type":     "memory",
			"rank_position": i + 1,
		})
	}

	// Calculate process summary statistics
	totalProcs := len(metrics.TopProcesses)
	runningProcs := countProcessesByStatus(metrics.TopProcesses, "R")
	sleepingProcs := countProcessesByStatus(metrics.TopProcesses, "S")
	zombieProcs := countProcessesByStatus(metrics.TopProcesses, "Z")
	diskSleepProcs := countProcessesByStatus(metrics.TopProcesses, "D")
	stoppedProcs := countProcessesByStatus(metrics.TopProcesses, "T")

	// Calculate aggregated resource usage
	var totalCPU float64
	var totalMemoryKB int64
	for _, proc := range metrics.TopProcesses {
		totalCPU += proc.CPUUsage
		totalMemoryKB += proc.MemoryKB
	}

	avgCPU := float64(0)
	avgMemoryKB := int64(0)
	if totalProcs > 0 {
		avgCPU = totalCPU / float64(totalProcs)
		avgMemoryKB = totalMemoryKB / int64(totalProcs)
	}

	processSummary := map[string]interface{}{
		"total_processes":           totalProcs,
		"running_processes":         runningProcs,
		"sleeping_processes":        sleepingProcs,
		"zombie_processes":          zombieProcs,
		"disk_sleep_processes":      diskSleepProcs,
		"stopped_processes":         stoppedProcs,
		"total_cpu_usage":           totalCPU,
		"total_memory_kb":           totalMemoryKB,
		"avg_cpu_per_process":       avgCPU,
		"avg_memory_kb_per_process": avgMemoryKB,
		"tags": map[string]string{
			"hostname": hostname,
			"os_name":  metrics.OSName,
		},
	}

	// Create the request payload according to ProcessesBatchRequest schema
	requestData := map[string]interface{}{
		"timestamp":            fmt.Sprintf("%d", time.Now().Unix()),
		"user_token":           cfg.AuthToken,
		"server_id":            cfg.ServerID,
		"top_cpu_processes":    topCPUData,
		"top_memory_processes": topMemoryData,
		"process_summary":      processSummary,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return
	}

	// Send process metrics to backend asynchronously
	go func() {
		// Log process metrics request start
		if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer logFile.Close()
			logFile.WriteString(fmt.Sprintf("[%s] INFO: Process metrics request started - URL: %s\n",
				time.Now().Format("2006-01-02 15:04:05"), constants.PROCESSES_URL))
		}

		req, err := utils.CreateCLIRequest("POST", constants.PROCESSES_URL, bytes.NewBuffer(jsonData), getCurrentVersion())
		if err != nil {
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] ERROR: Failed to create process metrics request: %v\n",
					time.Now().Format("2006-01-02 15:04:05"), err))
			}
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// Log process metrics send error
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] ERROR: Failed to send process metrics: %v\n",
					time.Now().Format("2006-01-02 15:04:05"), err))
			}
			return
		}
		defer resp.Body.Close()

		// Log process metrics success
		if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer logFile.Close()
			if resp.StatusCode == 200 {
				logFile.WriteString(fmt.Sprintf("[%s] INFO: Process metrics sent successfully - Status: %d\n",
					time.Now().Format("2006-01-02 15:04:05"), resp.StatusCode))
			} else {
				logFile.WriteString(fmt.Sprintf("[%s] WARNING: Process metrics sent with non-200 status - Status: %d\n",
					time.Now().Format("2006-01-02 15:04:05"), resp.StatusCode))
			}
		}
	}()
}

// sendMetrics sends system metrics to the backend metrics endpoint
func sendMetrics(cfg *config.Config, metrics *metrics.Metrics) {
	if cfg.AuthToken == "" || cfg.ServerID == "" {
		return // Don't send if token or server_id is missing
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Current timestamp in ISO format for BaseMetric (UTC)
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// Convert to BaseMetric format - each metric is a separate object
	baseMetrics := []map[string]interface{}{
		// CPU metric
		{
			"timestamp":    timestamp,
			"server_id":    cfg.ServerID,
			"metric_type":  "system",
			"metric_name":  "cpu_usage",
			"metric_value": metrics.CPUUsage,
			"metric_unit":  "percent",
			"tags": map[string]string{
				"hostname": hostname,
				"os_name":  metrics.OSName,
			},
			"metadata": fmt.Sprintf(`{"cores_total": %d, "cores_used": %d}`,
				metrics.CPUDetails.Total, metrics.CPUDetails.Used),
		},
		// Memory metric
		{
			"timestamp":    timestamp,
			"server_id":    cfg.ServerID,
			"metric_type":  "system",
			"metric_name":  "memory_usage",
			"metric_value": metrics.MemoryUsage,
			"metric_unit":  "percent",
			"tags": map[string]string{
				"hostname": hostname,
				"os_name":  metrics.OSName,
			},
			"metadata": fmt.Sprintf(`{"total_kb": %d, "used_kb": %d, "available_kb": %d}`,
				metrics.MemoryDetails.Total, metrics.MemoryDetails.Used, metrics.MemoryDetails.Available),
		},
		// Disk metric
		{
			"timestamp":    timestamp,
			"server_id":    cfg.ServerID,
			"metric_type":  "system",
			"metric_name":  "disk_usage",
			"metric_value": metrics.DiskUsage,
			"metric_unit":  "percent",
			"tags": map[string]string{
				"hostname": hostname,
				"os_name":  metrics.OSName,
			},
			"metadata": fmt.Sprintf(`{"total_kb": %d, "used_kb": %d, "available_kb": %d}`,
				metrics.DiskDetails.Total, metrics.DiskDetails.Used, metrics.DiskDetails.Available),
		},
		// HTTPS connections metric
		{
			"timestamp":    timestamp,
			"server_id":    cfg.ServerID,
			"metric_type":  "network",
			"metric_name":  "https_connections",
			"metric_value": float64(metrics.HTTPSRequests),
			"metric_unit":  "count",
			"tags": map[string]string{
				"hostname": hostname,
				"os_name":  metrics.OSName,
			},
			"metadata": `{}`,
		},
		// IOPS metric
		{
			"timestamp":    timestamp,
			"server_id":    cfg.ServerID,
			"metric_type":  "storage",
			"metric_name":  "iops",
			"metric_value": float64(metrics.IOPS),
			"metric_unit":  "operations_per_sec",
			"tags": map[string]string{
				"hostname": hostname,
				"os_name":  metrics.OSName,
			},
			"metadata": `{}`,
		},
		// IO Wait metric
		{
			"timestamp":    timestamp,
			"server_id":    cfg.ServerID,
			"metric_type":  "storage",
			"metric_name":  "io_wait",
			"metric_value": metrics.IOWait,
			"metric_unit":  "percent",
			"tags": map[string]string{
				"hostname": hostname,
				"os_name":  metrics.OSName,
			},
			"metadata": `{}`,
		},
	}

	// Create the request payload according to MetricsBatchRequest schema
	requestData := map[string]interface{}{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().Unix()), // Unix timestamp for request
		"user_token": cfg.AuthToken,
		"metrics":    baseMetrics, // Array of BaseMetric objects
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return
	}

	// Send metrics to backend asynchronously
	go func() {
		// Log metrics request start
		if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer logFile.Close()
			logFile.WriteString(fmt.Sprintf("[%s] INFO: Metrics request started - URL: %s\n",
				time.Now().Format("2006-01-02 15:04:05"), constants.METRICS_URL))
		}

		req, err := utils.CreateCLIRequest("POST", constants.METRICS_URL, bytes.NewBuffer(jsonData), getCurrentVersion())
		if err != nil {
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] ERROR: Failed to create metrics request: %v\n",
					time.Now().Format("2006-01-02 15:04:05"), err))
			}
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// Log metrics send error
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] ERROR: Failed to send metrics: %v\n",
					time.Now().Format("2006-01-02 15:04:05"), err))
			}
			return
		}
		defer resp.Body.Close()

		// Log metrics success
		if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer logFile.Close()
			if resp.StatusCode == 200 {
				logFile.WriteString(fmt.Sprintf("[%s] INFO: Metrics sent successfully - Status: %d\n",
					time.Now().Format("2006-01-02 15:04:05"), resp.StatusCode))
			} else {
				logFile.WriteString(fmt.Sprintf("[%s] WARNING: Metrics sent with non-200 status - Status: %d\n",
					time.Now().Format("2006-01-02 15:04:05"), resp.StatusCode))
			}
		}
	}()
}

// updateServerVersion updates server version in database after update
func updateServerVersion(userToken string, cfg *config.Config) bool {
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
			"catops_version": getCurrentVersion(),
		},
		"cpu_cores":     serverSpecs["cpu_cores"],
		"total_memory":  serverSpecs["total_memory"],
		"total_storage": serverSpecs["total_storage"],
	}

	jsonData, _ := json.Marshal(serverData)

	req, err := utils.CreateCLIRequest("POST", constants.INSTALL_URL, bytes.NewBuffer(jsonData), getCurrentVersion())
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		defer logFile.Close()
		if resp.StatusCode == 200 {
			logFile.WriteString(fmt.Sprintf("[%s] INFO: Server version updated to %s\n",
				time.Now().Format("2006-01-02 15:04:05"), getCurrentVersion()))
		} else {
			logFile.WriteString(fmt.Sprintf("[%s] WARNING: Failed to update server version - Status: %d\n",
				time.Now().Format("2006-01-02 15:04:05"), resp.StatusCode))
		}
	}

	return resp.StatusCode == 200
}

// Регистрация сервера
func registerServer(userToken string, cfg *config.Config) bool {
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
		if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer f.Close()
			f.WriteString(fmt.Sprintf("[%s] WARNING: Could not get server specs: %v\n",
				time.Now().Format("2006-01-02 15:04:05"), err))
		}
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
			"catops_version": getCurrentVersion(),
		},
		// Add server specifications
		"cpu_cores":     serverSpecs["cpu_cores"],
		"total_memory":  serverSpecs["total_memory"],
		"total_storage": serverSpecs["total_storage"],
	}

	jsonData, _ := json.Marshal(serverData)

	// Debug: Log what we're sending
	if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] DEBUG: JSON data: %s\n",
			time.Now().Format("2006-01-02 15:04:05"), string(jsonData)))
	}

	// Debug: Log pretty JSON for better readability
	prettyJSON, _ := json.MarshalIndent(serverData, "", "  ")
	if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] DEBUG: Pretty JSON:\n%s\n",
			time.Now().Format("2006-01-02 15:04:05"), string(prettyJSON)))
	}

	// Debug: Log HTTP request details
	if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] DEBUG: Sending to URL: %s\n",
			time.Now().Format("2006-01-02 15:04:05"), constants.INSTALL_URL))
		f.WriteString(fmt.Sprintf("[%s] DEBUG: Request method: POST\n",
			time.Now().Format("2006-01-02 15:04:05")))
	}

	req, err := utils.CreateCLIRequest("POST", constants.INSTALL_URL, bytes.NewBuffer(jsonData), getCurrentVersion())
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
		// New backend returns: data.user_token and data.id

		if data, ok := result["data"].(map[string]interface{}); ok {
			// Extract permanent user_token (replaces server_token)
			if userToken, ok := data["user_token"].(string); ok && userToken != "" {
				cfg.AuthToken = userToken // Store permanent user_token as AuthToken
				if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
					defer f.Close()
					f.WriteString(fmt.Sprintf("[%s] INFO: Permanent user_token received and saved\n",
						time.Now().Format("2006-01-02 15:04:05")))
				}
			}

			// Extract server_id (MongoDB ObjectId)
			if serverID, ok := data["id"].(string); ok && serverID != "" {
				cfg.ServerID = serverID
				if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
					defer f.Close()
					f.WriteString(fmt.Sprintf("[%s] INFO: Server ID received and saved: %s\n",
						time.Now().Format("2006-01-02 15:04:05"), serverID))
				}
			}

			// Log successful registration
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] SUCCESS: Server registration completed\n",
					time.Now().Format("2006-01-02 15:04:05")))
			}
		} else {
			// log that data section not found
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] ERROR: data section not found in response\n",
					time.Now().Format("2006-01-02 15:04:05")))
			}
		}
		return true
	}

	return false
}

// Helper functions for process analytics
func countProcessesByStatus(processes []metrics.ProcessInfo, status string) int {
	count := 0
	for _, proc := range processes {
		if proc.Status == status {
			count++
		}
	}
	return count
}

func getTopProcessesByCPU(processes []metrics.ProcessInfo, limit int) []metrics.ProcessInfo {
	sorted := make([]metrics.ProcessInfo, len(processes))
	copy(sorted, processes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CPUUsage > sorted[j].CPUUsage
	})

	if limit < len(sorted) {
		return sorted[:limit]
	}
	return sorted
}

func getTopProcessesByMemory(processes []metrics.ProcessInfo, limit int) []metrics.ProcessInfo {
	sorted := make([]metrics.ProcessInfo, len(processes))
	copy(sorted, processes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].MemoryUsage > sorted[j].MemoryUsage
	})

	if limit < len(sorted) {
		return sorted[:limit]
	}
	return sorted
}

// send uninstall notification to backend
func sendUninstallNotification(authToken, serverID string) bool {
	// ServerUninstallRequest format - only needs timestamp and user_token
	uninstallData := map[string]interface{}{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().Unix()),
		"user_token": authToken,
	}

	jsonData, _ := json.Marshal(uninstallData)

	// Debug logging
	if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] DEBUG: Uninstall request data: %s\n",
			time.Now().Format("2006-01-02 15:04:05"), string(jsonData)))
		f.WriteString(fmt.Sprintf("[%s] DEBUG: Uninstall URL: %s\n",
			time.Now().Format("2006-01-02 15:04:05"), constants.UNINSTALL_URL))
	}

	// create request
	req, err := utils.CreateCLIRequest("POST", constants.UNINSTALL_URL, bytes.NewBuffer(jsonData), getCurrentVersion())
	if err != nil {
		// Debug logging for error
		if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer f.Close()
			f.WriteString(fmt.Sprintf("[%s] ERROR: Failed to create uninstall request: %v\n",
				time.Now().Format("2006-01-02 15:04:05"), err))
		}
		return false
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Debug logging for HTTP error
		if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer f.Close()
			f.WriteString(fmt.Sprintf("[%s] ERROR: HTTP request failed: %v\n",
				time.Now().Format("2006-01-02 15:04:05"), err))
		}
		return false
	}
	defer resp.Body.Close()

	// log result
	if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		defer f.Close()
		if resp.StatusCode == 200 {
			f.WriteString(fmt.Sprintf("[%s] INFO: Uninstall notification sent successfully\n",
				time.Now().Format("2006-01-02 15:04:05")))
		} else {
			f.WriteString(fmt.Sprintf("[%s] WARNING: Uninstall notification failed with status %d\n",
				time.Now().Format("2006-01-02 15:04:05"), resp.StatusCode))
		}
	}

	return resp.StatusCode == 200
}

// transfer server ownership
func transferServerOwnership(oldToken, newToken, serverID string) bool {
	// ServerOwnerChangeRequest format - no server_id needed, backend finds it via token
	changeData := map[string]interface{}{
		"timestamp":      fmt.Sprintf("%d", time.Now().Unix()),
		"old_user_token": oldToken,
		"new_user_token": newToken,
	}

	jsonData, _ := json.Marshal(changeData)

	req, err := utils.CreateCLIRequest("POST", constants.SERVERS_URL, bytes.NewBuffer(jsonData), getCurrentVersion())
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

func main() {
	// load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// create root command
	rootCmd := &cobra.Command{
		Use:                "catops",
		Short:              "Professional CatOps Tool",
		DisableSuggestions: true,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			// check if --version flag is set
			if cmd.Flags().Lookup("version").Changed {
				version := VERSION
				if version == "" {
					// read version from version.txt if VERSION is not set
					if versionData, err := os.ReadFile("version.txt"); err == nil {
						version = strings.TrimSpace(string(versionData))
					}
				}
				fmt.Printf("v%s\n", version)
				return nil
			}

			ui.PrintHeader()

			ui.PrintSection("Core Features")

			// features section
			featuresData := map[string]string{
				"System Monitoring": "CPU, Memory, Disk metrics",
				"Alert System":      "Telegram notifications",
				"Remote Control":    "Telegram bot commands",
				"Open Source":       "Free monitoring solution",
				"Lightweight":       "Minimal resource usage",
			}
			fmt.Print(ui.CreateBeautifulList(featuresData))
			ui.PrintSectionEnd()

			// quick start section
			ui.PrintSection("Quick Start")
			quickStartData := map[string]string{
				"Start Service":  "catops start",
				"Set Thresholds": "catops set cpu=90",
				"Apply Changes":  "catops restart",
				"Check Status":   "catops status",
				"Telegram Bot":   "Auto-configured",
				"Cloud Mode":     "catops auth login <token>",
			}
			fmt.Print(ui.CreateBeautifulList(quickStartData))
			ui.PrintSectionEnd()

			// available commands section
			ui.PrintSection("Commands")
			commandsData := map[string]string{
				"status":  "Show system metrics",
				"start":   "Start monitoring service",
				"restart": "Restart monitoring service",
				"set":     "Set alert thresholds",
				"update":  "Update to latest version",
				"auth":    "Manage Cloud Mode authentication",
			}
			fmt.Print(ui.CreateBeautifulList(commandsData))
			ui.PrintSectionEnd()

			ui.PrintStatus("info", "Use 'catops [command] --help' for detailed help")
			return nil
		},
	}

	// add version flag
	rootCmd.Flags().BoolP("version", "v", false, "Show version information")

	// status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Display current system metrics and alert thresholds",
		Long: `Display real-time system information including:
  • System Information (Hostname, OS, IP, Uptime)
  • Current Metrics (CPU, Memory, Disk, HTTPS Connections)
  • Alert Thresholds (configured limits for alerts)

Examples:
  catops status          # Show all system information`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Using default thresholds")
				cfg = &config.Config{
					CPUThreshold:  constants.DEFAULT_CPU_THRESHOLD,
					MemThreshold:  constants.DEFAULT_MEMORY_THRESHOLD,
					DiskThreshold: constants.DEFAULT_DISK_THRESHOLD,
				}
			}

			// get system information
			hostname, _ := os.Hostname()
			metrics, err := metrics.GetMetrics()
			if err != nil {
				ui.PrintStatus("error", fmt.Sprintf("Error getting metrics: %v", err))
				return
			}

			// system information section
			ui.PrintSection("System Information")
			systemData := map[string]string{
				"Hostname": hostname,
				"OS":       metrics.OSName,
				"IP":       metrics.IPAddress,
				"Uptime":   metrics.Uptime,
			}
			fmt.Print(ui.CreateBeautifulList(systemData))
			ui.PrintSectionEnd()

			// timestamp section
			ui.PrintSection("Timestamp")
			timestampData := map[string]string{
				"Current Time": metrics.Timestamp,
			}
			fmt.Print(ui.CreateBeautifulList(timestampData))
			ui.PrintSectionEnd()

			// metrics section
			ui.PrintSection("Current Metrics")
			metricsData := map[string]string{
				"CPU Usage":         fmt.Sprintf("%s (%d cores, %d active)", utils.FormatPercentage(metrics.CPUUsage), metrics.CPUDetails.Total, metrics.CPUDetails.Used),
				"Memory Usage":      fmt.Sprintf("%s (%s / %s)", utils.FormatPercentage(metrics.MemoryUsage), utils.FormatBytes(metrics.MemoryDetails.Used*1024), utils.FormatBytes(metrics.MemoryDetails.Total*1024)),
				"Disk Usage":        fmt.Sprintf("%s (%s / %s)", utils.FormatPercentage(metrics.DiskUsage), utils.FormatBytes(metrics.DiskDetails.Used*1024), utils.FormatBytes(metrics.DiskDetails.Total*1024)),
				"HTTPS Connections": utils.FormatNumber(metrics.HTTPSRequests),
				"IOPS":              utils.FormatNumber(metrics.IOPS),
				"I/O Wait":          utils.FormatPercentage(metrics.IOWait),
			}
			fmt.Print(ui.CreateBeautifulList(metricsData))
			ui.PrintSectionEnd()

			// thresholds section
			ui.PrintSection("Alert Thresholds")
			thresholdData := map[string]string{
				"CPU Threshold":    utils.FormatPercentage(cfg.CPUThreshold),
				"Memory Threshold": utils.FormatPercentage(cfg.MemThreshold),
				"Disk Threshold":   utils.FormatPercentage(cfg.DiskThreshold),
			}
			fmt.Print(ui.CreateBeautifulList(thresholdData))
			ui.PrintSectionEnd()

			// daemon status
			ui.PrintSection("Daemon Status")
			if process.IsRunning() {
				ui.PrintStatus("success", "Monitoring daemon is running")
			} else {
				ui.PrintStatus("warning", "Monitoring daemon is not running")
			}
			ui.PrintSectionEnd()
		},
	}

	// processes command
	processesCmd := &cobra.Command{
		Use:   "processes",
		Short: "Show detailed information about running processes",
		Long: `Display detailed information about system processes including:
  • Top processes by CPU usage
  • Top processes by memory usage
  • Process details (PID, user, command, resource usage)

Examples:
  catops processes        # Show all process information
  catops processes -n 20 # Show top 20 processes`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Process Information")

			// get metrics with process information
			currentMetrics, err := metrics.GetMetrics()
			if err != nil {
				ui.PrintStatus("error", fmt.Sprintf("Error getting metrics: %v", err))
				ui.PrintSectionEnd()
				return
			}

			// get limit from flags
			limit, _ := cmd.Flags().GetInt("limit")

			// show top processes by CPU
			ui.PrintSection("Top Processes by CPU Usage")
			if len(currentMetrics.TopProcesses) > 0 {
				// sort by CPU usage
				sortedProcesses := make([]metrics.ProcessInfo, len(currentMetrics.TopProcesses))
				copy(sortedProcesses, currentMetrics.TopProcesses)
				sort.Slice(sortedProcesses, func(i, j int) bool {
					return sortedProcesses[i].CPUUsage > sortedProcesses[j].CPUUsage
				})

				// show top N processes
				if limit < len(sortedProcesses) {
					sortedProcesses = sortedProcesses[:limit]
				}

				fmt.Print(ui.CreateProcessTable(sortedProcesses))
			} else {
				ui.PrintStatus("warning", "No process information available")
			}
			ui.PrintTableSectionEnd()

			// show top processes by memory
			ui.PrintSection("Top Processes by Memory Usage")
			if len(currentMetrics.TopProcesses) > 0 {
				// sort by memory usage
				sortedProcesses := make([]metrics.ProcessInfo, len(currentMetrics.TopProcesses))
				copy(sortedProcesses, currentMetrics.TopProcesses)
				sort.Slice(sortedProcesses, func(i, j int) bool {
					return sortedProcesses[i].MemoryUsage > sortedProcesses[j].MemoryUsage
				})

				// show top N processes
				if limit < len(sortedProcesses) {
					sortedProcesses = sortedProcesses[:limit]
				}

				fmt.Print(ui.CreateProcessTableByMemory(sortedProcesses))
			} else {
				ui.PrintStatus("warning", "No process information available")
			}
			ui.PrintTableSectionEnd()
		},
	}
	processesCmd.Flags().IntP("limit", "n", 10, "Number of processes to show")

	// restart command
	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "Stop and restart the monitoring service",
		Long: `Stop the current monitoring process and start a new one.
This ensures the monitoring service uses the latest configuration.
The Telegram bot will also be restarted if configured.

Examples:
  catops restart         # Restart monitoring with current config`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Restarting Monitoring Service")

			// stop current process
			if process.IsRunning() {
				err := process.StopProcess()
				if err != nil {
					ui.PrintStatus("error", fmt.Sprintf("Failed to stop: %v", err))
					ui.PrintSectionEnd()
					return
				}
				ui.PrintStatus("success", "Monitoring service stopped")
			}

			// start new process
			err := process.StartProcess()
			if err != nil {
				ui.PrintStatus("error", fmt.Sprintf("Failed to start: %v", err))
				ui.PrintSectionEnd()
				return
			}

			ui.PrintStatus("success", "Monitoring service restarted successfully")

			// Send service_restart event
			if cfg.AuthToken != "" && cfg.ServerID != "" {
				if currentMetrics, err := metrics.GetMetrics(); err == nil {
					sendAllAnalytics(cfg, "service_restart", currentMetrics)
				}
			}

			// check if Telegram is configured
			if cfg.TelegramToken != "" && cfg.ChatID != 0 {
				ui.PrintStatus("info", "Telegram notifications enabled")
			}

			ui.PrintSectionEnd()
		},
	}

	// update command
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Download and install the latest version",
		Long: `Check for and install the latest version of CatOps.
This will check if updates are available and install them if found.
The update process is handled by the official update script.

Examples:
  catops update          # Check and install updates`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Checking for Updates")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Continuing with update check without server version")
				cfg = &config.Config{}
			}

			// Check if we have authentication
			if cfg.AuthToken == "" {
				ui.PrintStatus("warning", "No authentication token found")
				ui.PrintStatus("info", "Run 'catops auth login <token>' to authenticate")
				ui.PrintStatus("info", "Continuing with basic update check...")

				// Fallback to basic update check
				checkBasicUpdate()
				return
			}

			// Check server version against latest
			ui.PrintStatus("info", "Checking server version...")
			serverVersion, latestVersion, _, err := checkServerVersion(cfg.AuthToken)
			if err != nil {
				ui.PrintStatus("warning", fmt.Sprintf("Failed to check server version: %v", err))
				ui.PrintStatus("info", "Falling back to basic update check...")
				checkBasicUpdate()
				return
			}

			// Show CLI binary version as current version (not database version)
			currentVersion := getCurrentVersion()
			ui.PrintStatus("info", fmt.Sprintf("Current version: %s", currentVersion))
			ui.PrintStatus("info", fmt.Sprintf("Latest version: %s", latestVersion))

			// Log if database version differs from binary version (for debugging)
			if serverVersion != "" && serverVersion != currentVersion {
				if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
					defer logFile.Close()
					logFile.WriteString(fmt.Sprintf("[%s] WARNING: Version mismatch - Binary: %s, Database: %s\n",
						time.Now().Format("2006-01-02 15:04:05"), currentVersion, serverVersion))
				}
			}

			// Check if binary needs update (compare binary version with latest)
			if currentVersion == latestVersion {
				ui.PrintStatus("success", "Server is up to date!")
				ui.PrintSectionEnd()
				return
			}

			ui.PrintStatus("info", "Update available! Installing...")
			ui.PrintSectionEnd()

			// Execute the update script
			executeUpdateScript()
		},
	}

	// start command
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start background monitoring service",
		Long: `Start the monitoring service in the background.
The service will continuously check system metrics and send Telegram alerts
when thresholds are exceeded.

To run in background (recommended):
  nohup catops start > /dev/null 2>&1 &

To run in foreground (for testing):
  catops start

Examples:
  catops start           # Start monitoring service (foreground)
  nohup catops start &   # Start monitoring service (background)`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Starting Monitoring Service")

			if process.IsRunning() {
				ui.PrintStatus("warning", "Monitoring service is already running")
				ui.PrintSectionEnd()
				return
			}

			err := process.StartProcess()
			if err != nil {
				ui.PrintStatus("error", fmt.Sprintf("Failed to start: %v", err))
				ui.PrintSectionEnd()
				return
			}

			ui.PrintStatus("success", "Monitoring service started successfully")
			ui.PrintSectionEnd()
		},
	}

	// set command
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Configure alert thresholds for CPU, Memory, and Disk",
		Long: `Set individual alert thresholds for system metrics.
After changing thresholds, run 'catops restart' to apply changes to the running service.

Supported metrics:
  • cpu    - CPU usage percentage (0-100)
  • mem    - Memory usage percentage (0-100)  
  • disk   - Disk usage percentage (0-100)

Examples:
  catops set cpu=90              # Set CPU threshold to 90%
  catops set mem=80 disk=85      # Set Memory to 80%, Disk to 85%
  catops set cpu=70 mem=75 disk=90  # Set all thresholds at once`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Configuring Alert Thresholds")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Using default values")
				cfg = &config.Config{
					CPUThreshold:  constants.DEFAULT_CPU_THRESHOLD,
					MemThreshold:  constants.DEFAULT_MEMORY_THRESHOLD,
					DiskThreshold: constants.DEFAULT_DISK_THRESHOLD,
				}
			}

			if len(args) == 0 {
				ui.PrintStatus("error", "Usage: catops set cpu=90 mem=90 disk=90")
				ui.PrintStatus("info", "Supported: cpu, mem, disk")
				ui.PrintSectionEnd()
				return
			}

			// parse arguments and update config
			for _, arg := range args {
				parts := strings.Split(arg, "=")
				if len(parts) != 2 {
					ui.PrintStatus("error", fmt.Sprintf("Invalid format: %s", arg))
					continue
				}

				metric := parts[0]
				value, err := utils.ParseFloat(parts[1])
				if err != nil {
					ui.PrintStatus("error", fmt.Sprintf("Invalid value for %s: %s", metric, parts[1]))
					continue
				}

				if !utils.IsValidThreshold(value) {
					ui.PrintStatus("error", fmt.Sprintf("Invalid threshold for %s: %.1f%% (must be 0-100)", metric, value))
					continue
				}

				switch metric {
				case "cpu":
					cfg.CPUThreshold = value
				case "mem":
					cfg.MemThreshold = value
				case "disk":
					cfg.DiskThreshold = value
				default:
					ui.PrintStatus("error", fmt.Sprintf("Unknown metric: %s", metric))
					continue
				}

				ui.PrintStatus("success", fmt.Sprintf("Set %s threshold to %.1f%%", metric, value))
			}

			// save configuration
			err = config.SaveConfig(cfg)
			if err != nil {
				ui.PrintStatus("error", fmt.Sprintf("Failed to save config: %v", err))
				ui.PrintSectionEnd()
				return
			}

			ui.PrintStatus("success", "Configuration saved successfully")

			// Send config_change event
			if cfg.AuthToken != "" && cfg.ServerID != "" {
				if currentMetrics, err := metrics.GetMetrics(); err == nil {
					sendAllAnalytics(cfg, "config_change", currentMetrics)
				}
			}

			ui.PrintStatus("info", "Run 'catops restart' to apply changes")
			ui.PrintSectionEnd()
		},
	}

	// daemon command (hidden)
	daemonCmd := &cobra.Command{
		Use:    "daemon",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			// this is the actual monitoring daemon
			// write PID file
			pid := os.Getpid()
			if f, err := os.Create(constants.PID_FILE); err == nil {
				f.WriteString(fmt.Sprintf("%d", pid))
				f.Close()

				// log service start
				if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
					defer logFile.Close()
					logFile.WriteString(fmt.Sprintf("[%s] INFO: Service started - PID: %d\n",
						time.Now().Format("2006-01-02 15:04:05"), pid))
				}
			}

			// Prepare startup message (always prepare, Telegram is optional)
			hostname, _ := os.Hostname()
			ipAddress, _ := metrics.GetIPAddress()
			osName, _ := metrics.GetOSName()
			uptime, _ := metrics.GetUptime()

			startupMessage := fmt.Sprintf(`🚀 <b>CatOps Monitoring Started</b>

📊 <b>Server Information:</b>
• Hostname: %s
• OS: %s
• IP: %s
• Uptime: %s

⏰ <b>Startup Time:</b> %s

🔧 <b>Status:</b> Monitoring service is now active

📡 <b>Monitoring Active:</b>
• CPU, Memory, Disk usage
• System connections monitoring
• Real-time alerts

🔔 <b>Alert Thresholds:</b>
• CPU: %.1f%% (will trigger alert if exceeded)
• Memory: %.1f%% (will trigger alert if exceeded)
• Disk: %.1f%% (will trigger alert if exceeded)`, hostname, osName, ipAddress, uptime, time.Now().Format("2006-01-02 15:04:05"), cfg.CPUThreshold, cfg.MemThreshold, cfg.DiskThreshold)

			// Send Telegram notification if configured
			if cfg.TelegramToken != "" && cfg.ChatID != 0 {
				telegram.SendToTelegram(cfg.TelegramToken, cfg.ChatID, startupMessage)
			}

			// send service start analytics (always if in cloud mode)
			if currentMetrics, err := metrics.GetMetrics(); err == nil {
				sendAllAnalytics(cfg, "service_start", currentMetrics)
			}

			// Update server version in database if in cloud mode
			// This ensures version is updated after CLI updates
			if cfg.IsCloudMode() && cfg.AuthToken != "" && cfg.ServerID != "" {
				updateServerVersion(cfg.AuthToken, cfg)
			}

			// start Telegram bot in background if configured
			if cfg.TelegramToken != "" && cfg.ChatID != 0 {
				go telegram.StartBotInBackground(cfg)
			}

			// setup signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

			// start monitoring loop
			ticker := time.NewTicker(60 * time.Second)     // Changed from 30 to 60 seconds
			updateTicker := time.NewTicker(24 * time.Hour) // Check updates every minute (for testing)
			defer ticker.Stop()
			defer updateTicker.Stop()

			for {
				select {
				case <-ticker.C:
					// reload config to get latest changes
					currentCfg, err := config.LoadConfig()
					if err != nil {
						// if config reload fails, use cached config
						currentCfg = cfg
					}

					// get current metrics
					currentMetrics, err := metrics.GetMetrics()
					if err != nil {
						continue
					}

					// check for alerts
					alerts := []string{}
					if utils.CheckCPUAlert(currentMetrics.CPUUsage, currentCfg.CPUThreshold) {
						alerts = append(alerts, fmt.Sprintf("CPU: %.1f%% (limit: %.1f%%)", currentMetrics.CPUUsage, currentCfg.CPUThreshold))
					}
					if utils.CheckMemoryAlert(currentMetrics.MemoryUsage, currentCfg.MemThreshold) {
						alerts = append(alerts, fmt.Sprintf("Memory: %.1f%% (limit: %.1f%%)", currentMetrics.MemoryUsage, currentCfg.MemThreshold))
					}
					if utils.CheckDiskAlert(currentMetrics.DiskUsage, currentCfg.DiskThreshold) {
						alerts = append(alerts, fmt.Sprintf("Disk: %.1f%% (limit: %.1f%%)", currentMetrics.DiskUsage, currentCfg.DiskThreshold))
					}

					// send alert if any thresholds exceeded
					if len(alerts) > 0 {
						hostname, _ := os.Hostname()

						// log alert
						if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
							defer logFile.Close()
							logFile.WriteString(fmt.Sprintf("[%s] ALERT: Thresholds exceeded - %s\n",
								time.Now().Format("2006-01-02 15:04:05"), strings.Join(alerts, ", ")))
						}

						// Send Telegram notification if configured
						if currentCfg.TelegramToken != "" && currentCfg.ChatID != 0 {
							alertMessage := fmt.Sprintf(`⚠️ <b>ALERT: System Thresholds Exceeded</b>

📊 <b>Server:</b> %s
⏰ <b>Time:</b> %s

🚨 <b>Alerts:</b>
%s`, hostname, time.Now().Format("2006-01-02 15:04:05"), strings.Join(alerts, "\n"))

							telegram.SendToTelegram(currentCfg.TelegramToken, currentCfg.ChatID, alertMessage)
						}

						// send analytics to backend only if in cloud mode
						if currentCfg.IsCloudMode() {
							sendAlertAnalytics(currentCfg, alerts, currentMetrics)
						}
					} else {
						// if thresholds are not exceeded, send regular analytics
						if currentCfg.IsCloudMode() {
							sendAllAnalytics(currentCfg, "system_monitoring", currentMetrics)
						}
					}

				case <-updateTicker.C:
					// check for updates once per day (always check, Telegram is optional)
					// get current version
					cmd := exec.Command("catops", "--version")
					output, err := cmd.Output()
					if err == nil {
						currentVersion := strings.TrimSpace(string(output))
						currentVersion = strings.TrimPrefix(currentVersion, "v")

						// check API for latest version
						// log version check request start
						if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
							defer logFile.Close()
							logFile.WriteString(fmt.Sprintf("[%s] INFO: Version check request started - URL: %s\n",
								time.Now().Format("2006-01-02 15:04:05"), constants.VERSIONS_URL))
						}

						req, err := utils.CreateCLIRequest("GET", constants.VERSIONS_URL, nil, getCurrentVersion())
						if err != nil {
							continue
						}

						client := &http.Client{Timeout: 10 * time.Second}
						resp, err := client.Do(req)
						if err == nil {
							defer resp.Body.Close()
							var result map[string]interface{}
							if json.NewDecoder(resp.Body).Decode(&result) == nil {
								if latestVersion, ok := result["latest_version"].(string); ok {
									if latestVersion != currentVersion {
										hostname, _ := os.Hostname()
										updateMessage := fmt.Sprintf(`🔄 <b>New Update Available!</b>

📦 <b>Current:</b> v%s
🆕 <b>Latest:</b> v%s

💡 <b>To update, run this command on your server:</b>
<code>catops update</code>

📊 <b>Server:</b> %s
⏰ <b>Check Time:</b> %s`, currentVersion, latestVersion, hostname, time.Now().Format("2006-01-02 15:04:05"))

										// Send Telegram notification if configured
										if cfg.TelegramToken != "" && cfg.ChatID != 0 {
											telegram.SendToTelegram(cfg.TelegramToken, cfg.ChatID, updateMessage)
										}
									}
								}
							}
						}
					}

				case <-sigChan:
					// Graceful shutdown
					hostname, _ := os.Hostname()
					ipAddress, _ := metrics.GetIPAddress()
					osName, _ := metrics.GetOSName()
					uptime, _ := metrics.GetUptime()

					// Send Telegram notification if configured
					if cfg.TelegramToken != "" && cfg.ChatID != 0 {
						shutdownMessage := fmt.Sprintf(`🛑 <b>CatOps Monitoring Stopped</b>

📊 <b>Server Information:</b>
• Hostname: %s
• OS: %s
• IP: %s
• Uptime: %s

⏰ <b>Shutdown Time:</b> %s

🔧 <b>Status:</b> Monitoring service stopped gracefully`, hostname, osName, ipAddress, uptime, time.Now().Format("2006-01-02 15:04:05"))

						telegram.SendToTelegram(cfg.TelegramToken, cfg.ChatID, shutdownMessage)
					}

					// send service stop analytics (always if in cloud mode)
					if currentMetrics, err := metrics.GetMetrics(); err == nil {
						sendAllAnalytics(cfg, "service_stop", currentMetrics)
					}

					// log service stop
					if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
						defer logFile.Close()
						logFile.WriteString(fmt.Sprintf("[%s] INFO: Service stopped - PID: %d\n",
							time.Now().Format("2006-01-02 15:04:05"), pid))
					}

					// remove PID file
					if err := os.Remove(constants.PID_FILE); err != nil && !os.IsNotExist(err) {
						if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
							defer logFile.Close()
							logFile.WriteString(fmt.Sprintf("[%s] WARNING: Failed to remove PID file: %v\n",
								time.Now().Format("2006-01-02 15:04:05"), err))
						}
					}
					return
				}
			}
		},
	}

	// uninstall command
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Completely remove CatOps from the system",
		Long: `Completely remove CatOps from the system.

This command will:
• Stop the monitoring service
• Remove the binary from PATH
• Delete configuration files
• Remove autostart services
• Clean up all CatOps-related files

Examples:
  	catops uninstall        # Remove CatOps completely
  catops uninstall --yes  # Skip confirmation prompt`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Uninstall CatOps")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Continuing with uninstall without backend notification")
				cfg = &config.Config{} // Use empty config
			}

			// check if --yes flag is set
			skipConfirm := cmd.Flags().Lookup("yes").Changed

			if !skipConfirm {
				ui.PrintStatus("warning", "This will completely remove CatOps from your system!")
				ui.PrintStatus("warning", "This will completely remove CatOps from your system!")
				ui.PrintStatus("info", "All configuration and data will be lost.")

				fmt.Print("\nAre you sure you want to continue? (y/N): ")
				var response string
				fmt.Scanln(&response)

				if response != "y" && response != "Y" {
					ui.PrintStatus("info", "Uninstall cancelled")
					ui.PrintSectionEnd()
					return
				}
			}

			// send uninstall notification to backend if we have tokens
			ui.PrintStatus("debug", fmt.Sprintf("AuthToken present: %t, ServerID present: %t", cfg.AuthToken != "", cfg.ServerID != ""))
			backendNotified := false
			if cfg.AuthToken != "" && cfg.ServerID != "" {
				ui.PrintStatus("info", "Notifying backend about uninstall...")
				ui.PrintStatus("debug", fmt.Sprintf("Sending to URL: %s", constants.UNINSTALL_URL))
				if sendUninstallNotification(cfg.AuthToken, cfg.ServerID) {
					ui.PrintStatus("success", "Backend notified about uninstall")
					backendNotified = true
				} else {
					ui.PrintStatus("warning", "Could not notify backend (continuing with uninstall)")
				}
			} else {
				ui.PrintStatus("warning", "No auth token or server ID found - skipping backend notification")
			}

			// remove autostart services FIRST (before stopping service)
			switch runtime.GOOS {
			case "linux":
				homeDir, _ := os.UserHomeDir()
				systemdService := homeDir + "/.config/systemd/user/catops.service"
				if _, err := os.Stat(systemdService); err == nil {
					if err := exec.Command("systemctl", "--user", "disable", "catops.service").Run(); err != nil {
						ui.PrintStatus("warning", "Failed to disable systemd service (may already be disabled)")
					}
					if err := exec.Command("systemctl", "--user", "stop", "catops.service").Run(); err != nil {
						ui.PrintStatus("warning", "Failed to stop systemd service (may already be stopped)")
					}
					if err := os.Remove(systemdService); err != nil {
						ui.PrintStatus("warning", fmt.Sprintf("Failed to remove systemd service file: %v", err))
					}
				}
			case "darwin":
				homeDir, _ := os.UserHomeDir()
				launchAgent := homeDir + "/Library/LaunchAgents/com.catops.monitor.plist"
				if _, err := os.Stat(launchAgent); err == nil {
					if err := exec.Command("launchctl", "unload", launchAgent).Run(); err != nil {
						ui.PrintStatus("warning", "Failed to unload launchd service (may already be unloaded)")
					}
					if err := os.Remove(launchAgent); err != nil {
						ui.PrintStatus("warning", fmt.Sprintf("Failed to remove launchd plist: %v", err))
					}
				}
			case "windows":
				// Remove Task Scheduler task for Windows autostart
				taskName := "CatOps Monitor"
				if err := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run(); err != nil {
					ui.PrintStatus("warning", "Failed to remove scheduled task (may not exist)")
				} else {
					ui.PrintStatus("success", "Removed autostart task from Task Scheduler")
				}
			}

			// remove configuration directory
			homeDir, err := os.UserHomeDir()
			if err != nil {
				ui.PrintStatus("error", "Could not determine home directory")
				homeDir = os.Getenv("HOME") // fallback
			}
			configDir := filepath.Join(homeDir, ".catops")
			if err := os.RemoveAll(configDir); err == nil {
				ui.PrintStatus("success", "Configuration directory removed: "+configDir)
			} else {
				ui.PrintStatus("warning", fmt.Sprintf("Could not remove configuration directory: %v", err))
			}

			// remove log files only if backend was notified successfully
			if backendNotified {
				logFiles := []string{
					"/tmp/catops.log",
					"/tmp/catops.pid",
				}

				for _, logFile := range logFiles {
					if _, err := os.Stat(logFile); err == nil {
						if err := os.Remove(logFile); err == nil {
							ui.PrintStatus("success", "Removed log file: "+logFile)
						}
					}
				}
			} else {
				ui.PrintStatus("info", "Keeping log files for debugging (backend not notified)")
			}

			// stop ALL catops processes (after removing config)
			process.KillAllCatOpsProcesses()
			ui.PrintStatus("success", "All processes stopped")

			// remove ALL CatOps binaries from PATH LAST
			binaryPaths := []string{}

			// Platform-specific binary paths
			if runtime.GOOS == "windows" {
				localAppData := os.Getenv("LOCALAPPDATA")
				if localAppData != "" {
					binaryPaths = append(binaryPaths, filepath.Join(localAppData, "catops", "catops.exe"))
				}
				// Windows might also have it in PATH
				programFiles := os.Getenv("PROGRAMFILES")
				if programFiles != "" {
					binaryPaths = append(binaryPaths, filepath.Join(programFiles, "catops", "catops.exe"))
				}
			} else {
				// Unix-like systems
				binaryPaths = append(binaryPaths,
					"/usr/local/bin/catops",
					"/usr/bin/catops",
					filepath.Join(homeDir, ".local", "bin", "catops"),
				)
			}

			// also search for any other catops binaries in PATH
			pathSeparator := ":"
			if runtime.GOOS == "windows" {
				pathSeparator = ";"
			}
			pathDirs := strings.Split(os.Getenv("PATH"), pathSeparator)
			for _, dir := range pathDirs {
				if strings.Contains(dir, "catops") || strings.Contains(dir, ".local") || strings.Contains(dir, "bin") {
					binaryName := "catops"
					if runtime.GOOS == "windows" {
						binaryName = "catops.exe"
					}
					potentialPath := filepath.Join(dir, binaryName)
					if _, err := os.Stat(potentialPath); err == nil {
						binaryPaths = append(binaryPaths, potentialPath)
					}
				}
			}

			// remove all found binaries
			binaryRemoved := false
			for _, path := range binaryPaths {
				if _, err := os.Stat(path); err == nil {
					if err := os.Remove(path); err == nil {
						ui.PrintStatus("success", "Removed binary: "+path)
						binaryRemoved = true
					} else {
						ui.PrintStatus("warning", "Could not remove binary: "+path)
					}
				}
			}

			if !binaryRemoved {
				ui.PrintStatus("warning", "Could not find any CatOps binaries in standard locations")
			}

			ui.PrintStatus("success", "CatOps completely removed from the system")
			ui.PrintSectionEnd()
		},
	}

	// add --yes flag to uninstall command
	uninstallCmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	// cleanup command
	cleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old backup files and duplicate processes",
		Long: `Clean up old backup files created during updates and kill duplicate catops processes.
This will remove specific old backup files, clean up files older than 30 days, and ensure only one catops daemon is running.

Examples:
  catops cleanup          # Clean up old backups and processes`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Cleaning Up Old Backups and Processes")

			// clean up duplicate processes first
			process.KillDuplicateProcesses()
			process.CleanupZombieProcesses()
			ui.PrintStatus("success", "Process cleanup completed")

			// clean up old backup files
			executable, err := os.Executable()
			if err != nil {
				ui.PrintStatus("error", "Could not determine binary location")
				ui.PrintSectionEnd()
				return
			}
			backupDir := executable
			backupDir = backupDir[:len(backupDir)-len("/catops")] // Adjust path
			removedCount := 0
			for i := 3; i <= 10; i++ { // Remove specific old backups
				backupFile := fmt.Sprintf("%s/catops.backup.%d", backupDir, i)
				if _, err := os.Stat(backupFile); err == nil {
					os.Remove(backupFile)
					removedCount++
				}
			}
			cmd2 := exec.Command("find", backupDir, "-name", "catops.backup.*", "-mtime", "+30", "-delete")
			cmd2.Run() // Ignore errors
			ui.PrintStatus("success", fmt.Sprintf("Cleanup completed. Removed %d old backup files", removedCount))
			ui.PrintSectionEnd()
		},
	}

	// force cleanup command
	forceCleanupCmd := &cobra.Command{
		Use:   "force-cleanup",
		Short: "Force cleanup of all duplicate processes and zombie processes",
		Long: `Force cleanup of all duplicate catops processes and zombie processes.
This command will kill ALL catops daemon processes and clean up any zombie processes.
Use this when you have multiple processes running and need a fresh start.

Examples:
  catops force-cleanup    # Kill all processes and start fresh`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Force Cleanup of All Processes")

			ui.PrintStatus("warning", "This will kill ALL catops daemon processes!")

			// kill all catops daemon processes
			process.KillAllCatOpsProcesses()

			ui.PrintStatus("success", "Force cleanup completed. All processes killed.")
			ui.PrintStatus("info", "Run 'catops start' to start fresh monitoring service.")
			ui.PrintSectionEnd()
		},
	}

	// config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configure Telegram bot and backend analytics",
		Long: `Configure Telegram bot token and group ID.
This allows you to set up or change your configuration for notifications.

Use 'catops config show' to see current settings.`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Configuration")

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				ui.PrintStatus("info", "Using default values")
				cfg = &config.Config{}
			}

			if len(args) == 0 {
				ui.PrintStatus("error", "Usage: catops config [token=...|group=...|show]")
				ui.PrintStatus("info", "Run 'catops config show' to see current settings")
				ui.PrintSectionEnd()
				return
			}

			arg := args[0]
			if arg == "show" {
				// show current configuration
				ui.PrintSection("Telegram Bot Configuration")
				if cfg.TelegramToken != "" {
					ui.PrintStatus("success", fmt.Sprintf("Bot Token: %s...%s", cfg.TelegramToken[:10], cfg.TelegramToken[len(cfg.TelegramToken)-10:]))
				} else {
					ui.PrintStatus("warning", "Bot Token: Not configured")
				}
				if cfg.ChatID != 0 {
					ui.PrintStatus("success", fmt.Sprintf("Group ID: %d", cfg.ChatID))
				} else {
					ui.PrintStatus("warning", "Group ID: Not configured")
				}
				ui.PrintSectionEnd()

				ui.PrintSection("Backend Analytics Configuration")
				if cfg.AuthToken != "" {
					ui.PrintStatus("success", fmt.Sprintf("Auth Token: %s...%s", cfg.AuthToken[:10], cfg.AuthToken[len(cfg.AuthToken)-10:]))
					ui.PrintStatus("info", "Analytics will be sent to backend")
				} else {
					ui.PrintStatus("warning", "Auth Token: Not configured")
					ui.PrintStatus("info", "Analytics won't be sent to backend")
				}
				ui.PrintSectionEnd()

				ui.PrintSection("Alert Thresholds")
				ui.PrintStatus("success", fmt.Sprintf("CPU Threshold: %.1f%%", cfg.CPUThreshold))
				ui.PrintStatus("success", fmt.Sprintf("Memory Threshold: %.1f%%", cfg.MemThreshold))
				ui.PrintStatus("success", fmt.Sprintf("Disk Threshold: %.1f%%", cfg.DiskThreshold))
				ui.PrintSectionEnd()
				return
			}

			// parse argument
			parts := strings.Split(arg, "=")
			if len(parts) != 2 {
				ui.PrintStatus("error", "Invalid format. Use: token=..., group=..., or auth=...")
				ui.PrintSectionEnd()
				return
			}

			key := parts[0]
			value := parts[1]

			switch key {
			case "token":
				if len(value) < 20 {
					ui.PrintStatus("error", "Invalid token format. Token should be longer")
					ui.PrintSectionEnd()
					return
				}
				cfg.TelegramToken = value
				ui.PrintStatus("success", "Bot token updated successfully")

			case "group":
				groupID, err := utils.ParseInt(value)
				if err != nil {
					ui.PrintStatus("error", "Invalid group ID. Must be a number")
					ui.PrintSectionEnd()
					return
				}
				cfg.ChatID = groupID
				ui.PrintStatus("success", fmt.Sprintf("Group ID updated to: %d", groupID))

			case "auth":
				ui.PrintStatus("error", "Use 'catops auth login <token>' instead")
				ui.PrintSectionEnd()
				return

			default:
				ui.PrintStatus("error", "Unknown setting. Use: token, group, or show")
				ui.PrintSectionEnd()
				return
			}

			// save configuration
			{
				err = config.SaveConfig(cfg)
				if err != nil {
					ui.PrintStatus("error", fmt.Sprintf("Failed to save config: %v", err))
					return
				}
			}

			ui.PrintStatus("success", "Configuration saved successfully")

			// Send config_change event
			if cfg.AuthToken != "" && cfg.ServerID != "" {
				if currentMetrics, err := metrics.GetMetrics(); err == nil {
					sendAllAnalytics(cfg, "config_change", currentMetrics)
				}
			}

			ui.PrintStatus("info", "Run 'catops restart' to apply changes to the monitoring service")
			ui.PrintSectionEnd()
		},
	}

	// autostart command
	autostartCmd := &cobra.Command{
		Use:   "autostart",
		Short: "Enable or disable autostart on boot",
		Long: `Enable or disable autostart on boot.
This creates systemd service (Linux) or launchd service (macOS) to start catops automatically.
Examples:
  catops autostart enable   # Enable autostart
  catops autostart disable  # Disable autostart
  catops autostart status   # Check autostart status`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Autostart Management")

			if len(args) == 0 {
				ui.PrintStatus("error", "Please specify: enable, disable, or status")
				ui.PrintSectionEnd()
				return
			}

			action := args[0]
			executable, err := os.Executable()
			if err != nil {
				ui.PrintStatus("error", "Could not determine binary location")
				ui.PrintSectionEnd()
				return
			}

			switch action {
			case "enable":
				enableAutostart(executable)
			case "disable":
				disableAutostart()
			case "status":
				checkAutostartStatus()
			default:
				ui.PrintStatus("error", "Invalid action. Use: enable, disable, or status")
			}

			ui.PrintSectionEnd()
		},
	}

	// auth command
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
		Long: `Manage authentication for CatOps.

Commands:
  login    Login with authentication token
  logout   Logout and clear authentication
  status   Show authentication status`,
	}

	// login subcommand
	loginCmd := &cobra.Command{
		Use:   "login [token]",
		Short: "Login with authentication token",
		Long: `Login to CatOps with your authentication token.

Examples:
  catops auth login your_token_here
  catops auth login eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Authentication")

			newToken := args[0]

			// Load current config
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load config")
				return
			}

			// if we already have server_id, transfer ownership
			if cfg.ServerID != "" && cfg.AuthToken != "" {
				ui.PrintStatus("info", "Server is already registered, transferring ownership...")

				if !transferServerOwnership(cfg.AuthToken, newToken, cfg.ServerID) {
					ui.PrintStatus("error", "Failed to transfer server ownership")
					ui.PrintStatus("info", "Please check your token and try again")
					ui.PrintSectionEnd()
					return
				}

				ui.PrintStatus("success", "Server ownership transferred successfully")
			} else {
				// first time logging in - register server
				ui.PrintStatus("info", "Registering server with your account...")

				if !registerServer(newToken, cfg) {
					ui.PrintStatus("error", "Failed to register server")
					ui.PrintStatus("info", "Please check your token and try again")
					ui.PrintSectionEnd()
					return
				}

				ui.PrintStatus("success", "Server registered successfully")
			}

			// update auth_token (server_id is already saved in registerServer)
			cfg.AuthToken = newToken
			if err := config.SaveConfig(cfg); err != nil {
				ui.PrintStatus("error", "Failed to save authentication token")
				return
			}

			ui.PrintStatus("success", "Authentication successful")
			ui.PrintSectionEnd()
		},
	}

	// logout subcommand
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout and clear authentication",
		Long:  `Logout from CatOps and clear authentication token.`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Authentication")

			// load current config
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load config")
				return
			}

			// clear auth token
			cfg.AuthToken = ""

			// save config
			if err := config.SaveConfig(cfg); err != nil {
				ui.PrintStatus("error", "Failed to clear authentication token")
				return
			}

			ui.PrintStatus("success", "Logged out successfully")
			ui.PrintStatus("info", "Authentication token cleared")
			ui.PrintSectionEnd()
		},
	}

	// status subcommand
	statusAuthCmd := &cobra.Command{
		Use:   "info",
		Short: "Show authentication status",
		Long:  `Display current authentication status and server registration status.`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Authentication Status")

			// load current config
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load config")
				return
			}

			if cfg.AuthToken != "" {
				ui.PrintStatus("success", "Authenticated")
				ui.PrintStatus("info", "User ID: "+cfg.AuthToken)
				ui.PrintStatus("info", "Server registered: "+func() string {
					if cfg.ServerID != "" {
						return "Yes"
					}
					return "No"
				}())
			} else {
				ui.PrintStatus("warning", "Not authenticated")
				ui.PrintStatus("info", "Run 'catops auth login <token>' to authenticate")
			}

			ui.PrintSectionEnd()
		},
	}

	// token subcommand
	tokenCmd := &cobra.Command{
		Use:   "token",
		Short: "Show current authentication token",
		Long: `Display the current authentication token.

This command shows the full token that is currently stored in the configuration.`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Authentication Token")

			// load current config
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load config")
				return
			}

			if cfg.AuthToken != "" {
				ui.PrintStatus("success", "Current token:")
				fmt.Printf("  %s\n", cfg.AuthToken)
			} else {
				ui.PrintStatus("warning", "No authentication token found")
				ui.PrintStatus("info", "Run 'catops auth login <token>' to set a token")
			}

			ui.PrintSectionEnd()
		},
	}

	// add auth subcommands
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(statusAuthCmd)
	authCmd.AddCommand(tokenCmd)

	// add commands to root
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(processesCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(forceCleanupCmd)
	rootCmd.AddCommand(autostartCmd)
	rootCmd.AddCommand(authCmd)

	// execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// autostart functions
func enableAutostart(executable string) {
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
Type=simple
ExecStart=%s daemon
Restart=always
RestartSec=10
Environment=PATH=%s:/usr/local/bin:/usr/bin:/bin

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
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
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

	case "windows":
		// create Windows Task Scheduler task
		if err := createWindowsAutostart(executable); err != nil {
			ui.PrintStatus("error", fmt.Sprintf("Failed to create Windows task: %v", err))
			return
		}

		ui.PrintStatus("success", "Windows Task Scheduler task created")
		ui.PrintStatus("info", "CatOps will start automatically on boot")

	default:
		ui.PrintStatus("error", "Autostart not supported on this operating system")
	}
}

func disableAutostart() {
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

	case "windows":
		// remove Windows Task Scheduler task
		if err := removeWindowsAutostart(); err != nil {
			ui.PrintStatus("error", fmt.Sprintf("Failed to remove Windows task: %v", err))
			return
		}

		ui.PrintStatus("success", "Windows Task Scheduler task removed")

	default:
		ui.PrintStatus("error", "Autostart not supported on this operating system")
	}
}

func checkAutostartStatus() {
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

	case "windows":
		// check Windows Task Scheduler task status
		cmd := exec.Command("schtasks", "/Query", "/TN", "CatOpsMonitor")
		if err := cmd.Run(); err == nil {
			ui.PrintStatus("success", "Autostart is enabled (Task Scheduler)")
		} else {
			ui.PrintStatus("info", "Autostart is disabled (Task Scheduler)")
		}

	default:
		ui.PrintStatus("error", "Autostart not supported on this operating system")
	}
}

// Windows autostart functions
func createWindowsAutostart(executable string) error {
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

func removeWindowsAutostart() error {
	taskName := "CatOpsMonitor"

	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete task: %s - %w", string(output), err)
	}

	return nil
}
