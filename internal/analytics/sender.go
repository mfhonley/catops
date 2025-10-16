package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	constants "catops/config"
	"catops/internal/alerts"
	"catops/internal/config"
	"catops/internal/logger"
	"catops/internal/metrics"
	"catops/pkg/utils"
)

// Sender handles sending analytics data to the backend
type Sender struct {
	cfg     *config.Config
	version string
}

// NewSender creates a new analytics sender
func NewSender(cfg *config.Config, version string) *Sender {
	return &Sender{
		cfg:     cfg,
		version: version,
	}
}

// SendAll sends all analytics (metrics, processes, events) synchronously with WaitGroup
func (s *Sender) SendAll(eventType string, currentMetrics *metrics.Metrics) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
		return // Skip if not in cloud mode
	}

	var wg sync.WaitGroup

	// Send service analytics
	if eventType != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.SendServiceEvent(eventType, currentMetrics)
		}()
	}

	// Send metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.SendSystemMetrics(currentMetrics)
	}()

	// Send process metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.SendProcessMetrics(currentMetrics)
	}()

	// Send network metrics (Phase 1 - Network Observability)
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.SendNetworkMetrics(currentMetrics)
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
		logger.Warning("Analytics send timeout after 10 seconds")
	}
}

// ProcessAlert sends spike-based alert to backend with new Phase 2 format
func (s *Sender) ProcessAlert(alert *alerts.Alert, metrics *metrics.Metrics) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
		return // Don't send if token or server_id is missing
	}

	// Use alert's severity directly
	severity := string(alert.Severity)
	if severity == "" {
		severity = "warning"
	}

	// Map alert type to metric name
	metricName := getMetricName(alert.Type)

	// Get alert subtype (spike detection type)
	alertSubtype := string(alert.SubType)
	if alertSubtype == "" {
		alertSubtype = "threshold" // default
	}

	// Build alert details from existing Details field
	details := alert.Details
	if details == nil {
		details = make(map[string]interface{})
	}

	// Add timestamp and fingerprint
	details["detection_time"] = alert.Timestamp.Format(time.RFC3339)
	details["fingerprint"] = alert.Fingerprint

	// Create AlertProcessRequest payload
	alertData := map[string]interface{}{
		"user_token":    s.cfg.AuthToken,
		"server_id":     s.cfg.ServerID,
		"alert_type":    metricName,
		"alert_subtype": alertSubtype,
		"severity":      severity,
		"title":         alert.Title,
		"message":       alert.Message,
		"value":         alert.Value,
		"threshold":     alert.Threshold,
		"metric_name":   metricName,
		"details":       details,
	}

	jsonData, err := json.Marshal(alertData)
	if err != nil {
		logger.Error("Failed to marshal alert data: %v", err)
		return
	}

	// Send alert to backend asynchronously
	go func() {
		logger.Info("Processing alert - Type: %s, Subtype: %s, URL: %s", metricName, alertSubtype, constants.ALERTS_PROCESS_URL)

		req, err := utils.CreateCLIRequest("POST", constants.ALERTS_PROCESS_URL, bytes.NewBuffer(jsonData), s.version)
		if err != nil {
			logger.Error("Failed to create alert request: %v", err)
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			logger.Error("Failed to send alert: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			logger.Info("Alert processed successfully - Fingerprint: %s", alert.Fingerprint)
		} else {
			logger.Warning("Alert processed with non-200 status - Status: %d", resp.StatusCode)
		}
	}()
}

// SendHeartbeat sends heartbeat for active alert to keep it alive
func (s *Sender) SendHeartbeat(fingerprint string) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
		return
	}

	// Build heartbeat URL with fingerprint
	heartbeatURL := fmt.Sprintf("%s/%s/heartbeat", constants.ALERTS_HEARTBEAT_URL, fingerprint)

	heartbeatData := map[string]interface{}{
		"user_token": s.cfg.AuthToken,
		"server_id":  s.cfg.ServerID,
	}

	jsonData, err := json.Marshal(heartbeatData)
	if err != nil {
		logger.Error("Failed to marshal heartbeat data: %v", err)
		return
	}

	// Send heartbeat to backend asynchronously
	go func() {
		logger.Debug("Sending heartbeat for alert - Fingerprint: %s", fingerprint)

		req, err := utils.CreateCLIRequest("PUT", heartbeatURL, bytes.NewBuffer(jsonData), s.version)
		if err != nil {
			logger.Error("Failed to create heartbeat request: %v", err)
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			logger.Error("Failed to send heartbeat: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			logger.Debug("Heartbeat sent successfully - Fingerprint: %s", fingerprint)
		} else {
			logger.Warning("Heartbeat sent with non-200 status - Status: %d", resp.StatusCode)
		}
	}()
}

// ResolveAlert notifies backend that alert is resolved
func (s *Sender) ResolveAlert(fingerprint string) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
		return
	}

	resolveData := map[string]interface{}{
		"user_token":  s.cfg.AuthToken,
		"server_id":   s.cfg.ServerID,
		"fingerprint": fingerprint,
	}

	jsonData, err := json.Marshal(resolveData)
	if err != nil {
		logger.Error("Failed to marshal resolve data: %v", err)
		return
	}

	// Send resolve to backend asynchronously
	go func() {
		logger.Info("Resolving alert - Fingerprint: %s, URL: %s", fingerprint, constants.ALERTS_RESOLVE_URL)

		req, err := utils.CreateCLIRequest("POST", constants.ALERTS_RESOLVE_URL, bytes.NewBuffer(jsonData), s.version)
		if err != nil {
			logger.Error("Failed to create resolve request: %v", err)
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			logger.Error("Failed to send resolve: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			logger.Info("Alert resolved successfully - Fingerprint: %s", fingerprint)
		} else {
			logger.Warning("Alert resolved with non-200 status - Status: %d", resp.StatusCode)
		}
	}()
}

// getMetricName converts alert type to metric name for backend
func getMetricName(alertType alerts.AlertType) string {
	switch alertType {
	case alerts.AlertTypeCPU:
		return "cpu"
	case alerts.AlertTypeMemory:
		return "memory"
	case alerts.AlertTypeDisk:
		return "disk"
	case alerts.AlertTypeProcess:
		return "process"
	case alerts.AlertTypeNetwork:
		return "network"
	default:
		return "unknown"
	}
}

// SendServiceEvent sends service event data to the backend for monitoring and analytics
func (s *Sender) SendServiceEvent(eventType string, metrics *metrics.Metrics) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
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
		"server_id":     s.cfg.ServerID,
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
			"catops_version": s.version,
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
		"user_token": s.cfg.AuthToken,
		"events":     []map[string]interface{}{eventModel},
	}

	jsonData, _ := json.Marshal(serviceData)

	// send analytics to backend
	go func() {
		// log service analytics request start
		logger.Info("Analytics request started - Type: service_%s, URL: %s", eventType, constants.EVENTS_URL)

		req, err := utils.CreateCLIRequest("POST", constants.EVENTS_URL, bytes.NewBuffer(jsonData), s.version)
		if err != nil {
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// log analytics send error
			logger.Error("Failed to send service analytics: %v", err)
			return
		}
		defer resp.Body.Close()

		// log analytics success
		logger.Info("Analytics sent - Type: service_%s, Status: success", eventType)
	}()
}

// SendProcessMetrics sends process analytics to the backend processes endpoint
func (s *Sender) SendProcessMetrics(metrics *metrics.Metrics) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
		return // Don't send if token or server_id is missing
	}

	if len(metrics.TopProcesses) == 0 {
		return // No processes to send
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Get top 30 CPU processes (increased from 10 to capture more resource usage)
	topCPU := getTopProcessesByCPU(metrics.TopProcesses, 30)
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

	// Get top 30 Memory processes (increased from 10 to capture more resource usage)
	topMemory := getTopProcessesByMemory(metrics.TopProcesses, 30)
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

	// Calculate aggregated resource usage for ALL processes
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

	// Calculate "other" processes (not in top-30 CPU or top-30 Memory)
	includedPIDs := make(map[int]bool)
	for _, proc := range topCPU {
		includedPIDs[proc.PID] = true
	}
	for _, proc := range topMemory {
		includedPIDs[proc.PID] = true
	}

	// Aggregate "other" processes
	var otherCount int
	var otherCPU float64
	var otherMemoryKB int64
	otherByUser := make(map[string]map[string]interface{})

	for _, proc := range metrics.TopProcesses {
		if !includedPIDs[proc.PID] {
			otherCount++
			otherCPU += proc.CPUUsage
			otherMemoryKB += proc.MemoryKB

			// Aggregate by user
			if _, exists := otherByUser[proc.User]; !exists {
				otherByUser[proc.User] = map[string]interface{}{
					"count":     0,
					"cpu":       float64(0),
					"memory_kb": int64(0),
				}
			}
			userData := otherByUser[proc.User]
			userData["count"] = userData["count"].(int) + 1
			userData["cpu"] = userData["cpu"].(float64) + proc.CPUUsage
			userData["memory_kb"] = userData["memory_kb"].(int64) + proc.MemoryKB
		}
	}

	// Calculate "other" memory percentage (based on system total memory)
	otherMemoryPercent := float64(0)
	if metrics.MemoryDetails.Total > 0 {
		// Convert KB to bytes for accurate percentage
		otherMemoryPercent = (float64(otherMemoryKB) * 1024 / float64(metrics.MemoryDetails.Total*1024)) * 100
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
		// New fields for "other" processes
		"other_processes_count":   otherCount,
		"other_cpu_usage":         otherCPU,
		"other_memory_kb":         otherMemoryKB,
		"other_memory_percent":    otherMemoryPercent,
		"other_breakdown_by_user": otherByUser,
		"tags": map[string]string{
			"hostname": hostname,
			"os_name":  metrics.OSName,
		},
	}

	// Create the request payload according to ProcessesBatchRequest schema
	requestData := map[string]interface{}{
		"timestamp":            fmt.Sprintf("%d", time.Now().Unix()),
		"user_token":           s.cfg.AuthToken,
		"server_id":            s.cfg.ServerID,
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
		logger.Info("Process metrics request started - URL: %s", constants.PROCESSES_URL)

		req, err := utils.CreateCLIRequest("POST", constants.PROCESSES_URL, bytes.NewBuffer(jsonData), s.version)
		if err != nil {
			logger.Error("Failed to create process metrics request: %v", err)
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// Log process metrics send error
			logger.Error("Failed to send process metrics: %v", err)
			return
		}
		defer resp.Body.Close()

		// Log process metrics success
		if resp.StatusCode == 200 {
			logger.Info("Process metrics sent successfully - Status: %d", resp.StatusCode)
		} else {
			logger.Warning("Process metrics sent with non-200 status - Status: %d", resp.StatusCode)
		}
	}()
}

// SendSystemMetrics sends system metrics to the backend metrics endpoint
func (s *Sender) SendSystemMetrics(metrics *metrics.Metrics) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
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
			"server_id":    s.cfg.ServerID,
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
			"server_id":    s.cfg.ServerID,
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
			"server_id":    s.cfg.ServerID,
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
			"server_id":    s.cfg.ServerID,
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
			"server_id":    s.cfg.ServerID,
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
			"server_id":    s.cfg.ServerID,
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
		"user_token": s.cfg.AuthToken,
		"metrics":    baseMetrics, // Array of BaseMetric objects
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return
	}

	// Send metrics to backend asynchronously
	go func() {
		// Log metrics request start
		logger.Info("Metrics request started - URL: %s", constants.METRICS_URL)

		req, err := utils.CreateCLIRequest("POST", constants.METRICS_URL, bytes.NewBuffer(jsonData), s.version)
		if err != nil {
			logger.Error("Failed to create metrics request: %v", err)
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// Log metrics send error
			logger.Error("Failed to send metrics: %v", err)
			return
		}
		defer resp.Body.Close()

		// Log metrics success
		if resp.StatusCode == 200 {
			logger.Info("Metrics sent successfully - Status: %d", resp.StatusCode)
		} else {
			logger.Warning("Metrics sent with non-200 status - Status: %d", resp.StatusCode)
		}
	}()
}

// Helper functions

// countProcessesByStatus counts processes by their status
func countProcessesByStatus(processes []metrics.ProcessInfo, status string) int {
	count := 0
	for _, proc := range processes {
		if proc.Status == status {
			count++
		}
	}
	return count
}

// getTopProcessesByCPU returns the top N processes sorted by CPU usage
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

// getTopProcessesByMemory returns the top N processes sorted by memory usage
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

// SendNetworkMetrics sends network metrics to the backend network endpoint
func (s *Sender) SendNetworkMetrics(metricsData *metrics.Metrics) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
		return // Don't send if token or server_id is missing
	}

	// Skip if no network metrics collected
	if metricsData.NetworkMetrics == nil {
		return
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	networkMetrics := metricsData.NetworkMetrics

	// Convert NetworkConnection slice to []map[string]interface{}
	topConnectionsData := make([]map[string]interface{}, 0, len(networkMetrics.TopConnections))
	for _, conn := range networkMetrics.TopConnections {
		topConnectionsData = append(topConnectionsData, map[string]interface{}{
			"remote_ip":        conn.RemoteIP,
			"remote_port":      conn.RemotePort,
			"local_port":       conn.LocalPort,
			"protocol":         conn.Protocol,
			"state":            conn.State,
			"pid":              conn.PID,
			"family":           conn.Family,
			"importance_score": conn.ImportanceScore,
		})
	}

	// Create the request payload according to NetworkMetricsBatchRequest schema
	requestData := map[string]interface{}{
		"timestamp":                 fmt.Sprintf("%d", time.Now().Unix()),
		"user_token":                s.cfg.AuthToken,
		"server_id":                 s.cfg.ServerID,
		"inbound_bytes_per_sec":     networkMetrics.InboundBytesPerSec,
		"outbound_bytes_per_sec":    networkMetrics.OutboundBytesPerSec,
		"connections_established":   networkMetrics.ConnectionsEstablished,
		"connections_time_wait":     networkMetrics.ConnectionsTimeWait,
		"connections_close_wait":    networkMetrics.ConnectionsCloseWait,
		"connections_syn_sent":      networkMetrics.ConnectionsSynSent,
		"connections_syn_recv":      networkMetrics.ConnectionsSynRecv,
		"connections_fin_wait1":     networkMetrics.ConnectionsFinWait1,
		"connections_fin_wait2":     networkMetrics.ConnectionsFinWait2,
		"connections_listen":        networkMetrics.ConnectionsListen,
		"total_connections":         networkMetrics.TotalConnections,
		"top_connections":           topConnectionsData,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return
	}

	// Send network metrics to backend asynchronously
	go func() {
		// Log network metrics request start
		logger.Info("Network metrics request started - URL: %s", constants.NETWORK_METRICS_URL)

		req, err := utils.CreateCLIRequest("POST", constants.NETWORK_METRICS_URL, bytes.NewBuffer(jsonData), s.version)
		if err != nil {
			logger.Error("Failed to create network metrics request: %v", err)
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// Log network metrics send error
			logger.Error("Failed to send network metrics: %v", err)
			return
		}
		defer resp.Body.Close()

		// Log network metrics success
		if resp.StatusCode == 200 {
			logger.Info("Network metrics sent successfully - Status: %d", resp.StatusCode)
		} else {
			logger.Warning("Network metrics sent with non-200 status - Status: %d", resp.StatusCode)
		}
	}()
}
