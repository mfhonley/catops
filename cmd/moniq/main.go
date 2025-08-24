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
	"syscall"
	"time"

	"github.com/spf13/cobra"

	constants "moniq/config"
	"moniq/internal/config"
	"moniq/internal/metrics"
	"moniq/internal/process"
	"moniq/internal/telegram"
	"moniq/internal/ui"
	"moniq/pkg/utils"
)

// VERSION is set during build via ldflags
var VERSION string

// Helper functions for server operations

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

// sendAlertAnalytics sends alert data to the backend for monitoring and analytics
func sendAlertAnalytics(cfg *config.Config, alerts []string, metrics *metrics.Metrics) {

	if cfg.AuthToken == "" || cfg.ServerToken == "" {
		return // Don't send if tokens are missing
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	alertData := map[string]interface{}{
		"user_token":   cfg.AuthToken,
		"server_token": cfg.ServerToken,
		"alert_type":   "threshold_exceeded",
		"timestamp":    time.Now().Unix(),
		"server_info": map[string]interface{}{
			"hostname":      hostname,
			"os_type":       metrics.OSName,
			"moniq_version": getCurrentVersion(),
		},
		"metrics": map[string]interface{}{
			"cpu_usage":      metrics.CPUUsage,
			"memory_usage":   metrics.MemoryUsage,
			"disk_usage":     metrics.DiskUsage,
			"https_requests": metrics.HTTPSRequests,
			"iops":           metrics.IOPS,
			"io_wait":        metrics.IOWait,
		},
		"thresholds": map[string]interface{}{
			"cpu_threshold":    cfg.CPUThreshold,
			"memory_threshold": cfg.MemThreshold,
			"disk_threshold":   cfg.DiskThreshold,
		},
		"alerts": alerts,

		// Include detailed process analytics for backend monitoring
		"process_analytics": map[string]interface{}{
			"top_cpu_processes":    getTopProcessesByCPU(metrics.TopProcesses, 10),
			"top_memory_processes": getTopProcessesByMemory(metrics.TopProcesses, 10),
			"process_summary": map[string]interface{}{
				"total_processes":    len(metrics.TopProcesses),
				"running_processes":  countProcessesByStatus(metrics.TopProcesses, "R"),
				"sleeping_processes": countProcessesByStatus(metrics.TopProcesses, "S"),
				"zombie_processes":   countProcessesByStatus(metrics.TopProcesses, "Z"),
			},
		},
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

		req, err := http.NewRequest("POST", constants.ANALYTICS_URL, bytes.NewBuffer(jsonData))
		if err != nil {
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", constants.USER_AGENT)
		req.Header.Set("X-Platform", runtime.GOOS)
		req.Header.Set("X-Version", getCurrentVersion())

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

// Отправка аналитики событий сервиса на бэк
func sendServiceAnalytics(cfg *config.Config, eventType string, metrics *metrics.Metrics) {
	if cfg.AuthToken == "" || cfg.ServerToken == "" {
		return // Не отправляем если нет токенов
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Получаем PID процесса
	pid := os.Getpid()

	serviceData := map[string]interface{}{
		"user_token":   cfg.AuthToken,
		"server_token": cfg.ServerToken,
		"event_type":   eventType, // "service_start", "service_stop"
		"timestamp":    time.Now().Unix(),
		"server_info": map[string]interface{}{
			"hostname":      hostname,
			"os_type":       metrics.OSName,
			"moniq_version": getCurrentVersion(),
		},
		"metrics": map[string]interface{}{
			"cpu_usage":      metrics.CPUUsage,
			"memory_usage":   metrics.MemoryUsage,
			"disk_usage":     metrics.DiskUsage,
			"https_requests": metrics.HTTPSRequests,
			"iops":           metrics.IOPS,
			"io_wait":        metrics.IOWait,
		},
		"service_name": "moniq",
		"service_status": func() string {
			if eventType == "service_start" || eventType == "system_monitoring" {
				return "running"
			}
			return "stopped"
		}(),
		"service_pid": pid,
		"service_uptime": func() int64 {
			if eventType == "service_start" || eventType == "service_stop" {
				return 0
			}
			// could try to get uptime from PID file for stop events
			return 0
		}(),

		// add process analytics data for service events
		"process_analytics": map[string]interface{}{
			"top_cpu_processes":    getTopProcessesByCPU(metrics.TopProcesses, 5),
			"top_memory_processes": getTopProcessesByMemory(metrics.TopProcesses, 5),
			"process_summary": map[string]interface{}{
				"total_processes":    len(metrics.TopProcesses),
				"running_processes":  countProcessesByStatus(metrics.TopProcesses, "R"),
				"sleeping_processes": countProcessesByStatus(metrics.TopProcesses, "S"),
				"zombie_processes":   countProcessesByStatus(metrics.TopProcesses, "Z"),
			},
		},
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

		req, err := http.NewRequest("POST", constants.EVENTS_URL, bytes.NewBuffer(jsonData))
		if err != nil {
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", constants.USER_AGENT)
		req.Header.Set("X-Platform", runtime.GOOS)
		req.Header.Set("X-Version", getCurrentVersion())

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
			"hostname":      hostname,
			"os_type":       osName,
			"moniq_version": getCurrentVersion(),
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

	req, err := http.NewRequest("POST", constants.INSTALL_URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return false
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", constants.USER_AGENT)
	req.Header.Set("X-Platform", platform)
	req.Header.Set("X-Version", getCurrentVersion())

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
		// save server_token if it exists in response
		// server_token is in data.server_token

		if data, ok := result["data"].(map[string]interface{}); ok {
			if serverToken, ok := data["server_token"].(string); ok && serverToken != "" {
				// log found server_token

				// set server_token in passed cfg object
				cfg.ServerToken = serverToken
			} else {
				// log that server_token not found

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
func sendUninstallNotification(authToken, serverToken string) bool {
	uninstallData := map[string]interface{}{
		"auth_token":   authToken,
		"server_token": serverToken,
		"action":       "uninstall",
		"timestamp":    fmt.Sprintf("%d", time.Now().Unix()),
	}

	jsonData, _ := json.Marshal(uninstallData)

	// create request
	req, err := http.NewRequest("POST", constants.UNINSTALL_URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return false
	}

	// set headers (as required by backend)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", constants.USER_AGENT)
	req.Header.Set("X-Platform", runtime.GOOS) // linux/darwin/windows
	req.Header.Set("X-Version", getCurrentVersion())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
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
func transferServerOwnership(oldToken, newToken, serverToken string) bool {
	changeData := map[string]interface{}{
		"old_user_token": oldToken,
		"new_user_token": newToken,
		"server_token":   serverToken,
	}

	jsonData, _ := json.Marshal(changeData)

	resp, err := http.Post(constants.SERVERS_URL,
		"application/json", bytes.NewBuffer(jsonData))

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
		Use:                "moniq",
		Short:              "Professional Moniq CLI Tool",
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
				"Start Service":  "moniq start",
				"Set Thresholds": "moniq set cpu=90",
				"Apply Changes":  "moniq restart",
				"Check Status":   "moniq status",
				"Telegram Bot":   "Auto-configured",
				"Cloud Mode":     "moniq auth login <token>",
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

			ui.PrintStatus("info", "Use 'moniq [command] --help' for detailed help")
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
  moniq status          # Show all system information`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()

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
  moniq processes        # Show all process information
  moniq processes -n 20 # Show top 20 processes`,
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
			ui.PrintSectionEnd()

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

				fmt.Print(ui.CreateProcessTable(sortedProcesses))
			} else {
				ui.PrintStatus("warning", "No process information available")
			}
			ui.PrintSectionEnd()
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
  moniq restart         # Restart monitoring with current config`,
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
		Long: `Check for and install the latest version of Moniq CLI.
This will check if updates are available and install them if found.
The update process is handled by the official update script.

Examples:
  moniq update          # Check and install updates`,
		Run: func(cmd *cobra.Command, args []string) {
			// execute the update script directly
			updateCmd := exec.Command("bash", "-c", "curl -sfL "+constants.GET_MONIQ_URL+"/update.sh | bash")
			updateCmd.Stdout = os.Stdout
			updateCmd.Stderr = os.Stderr

			if err := updateCmd.Run(); err != nil {
				// don't treat any exit code as error (update.sh handles its own exit codes)
				return
			}
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
  nohup moniq start > /dev/null 2>&1 &

To run in foreground (for testing):
  moniq start

Examples:
  moniq start           # Start monitoring service (foreground)
  nohup moniq start &   # Start monitoring service (background)`,
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
After changing thresholds, run 'moniq restart' to apply changes to the running service.

Supported metrics:
  • cpu    - CPU usage percentage (0-100)
  • mem    - Memory usage percentage (0-100)  
  • disk   - Disk usage percentage (0-100)

Examples:
  moniq set cpu=90              # Set CPU threshold to 90%
  moniq set mem=80 disk=85      # Set Memory to 80%, Disk to 85%
  moniq set cpu=70 mem=75 disk=90  # Set all thresholds at once`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Configuring Alert Thresholds")

			if len(args) == 0 {
				ui.PrintStatus("error", "Usage: moniq set cpu=90 mem=90 disk=90")
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
			err := config.SaveConfig(cfg)
			if err != nil {
				ui.PrintStatus("error", fmt.Sprintf("Failed to save config: %v", err))
				ui.PrintSectionEnd()
				return
			}

			ui.PrintStatus("success", "Configuration saved successfully")
			ui.PrintStatus("info", "Run 'moniq restart' to apply changes")
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

			// send startup notification
			if cfg.TelegramToken != "" && cfg.ChatID != 0 {
				hostname, _ := os.Hostname()
				ipAddress, _ := metrics.GetIPAddress()
				osName, _ := metrics.GetOSName()
				uptime, _ := metrics.GetUptime()

				startupMessage := fmt.Sprintf(`🚀 <b>Moniq Monitoring Started</b>

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

				telegram.SendToTelegram(cfg.TelegramToken, cfg.ChatID, startupMessage)

				// send service start analytics
				if currentMetrics, err := metrics.GetMetrics(); err == nil {
					sendServiceAnalytics(cfg, "service_start", currentMetrics)
				}
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
					if len(alerts) > 0 && currentCfg.TelegramToken != "" && currentCfg.ChatID != 0 {
						hostname, _ := os.Hostname()

						// log alert
						if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
							defer logFile.Close()
							logFile.WriteString(fmt.Sprintf("[%s] ALERT: Thresholds exceeded - %s\n",
								time.Now().Format("2006-01-02 15:04:05"), strings.Join(alerts, ", ")))
						}

						alertMessage := fmt.Sprintf(`⚠️ <b>ALERT: System Thresholds Exceeded</b>

📊 <b>Server:</b> %s
⏰ <b>Time:</b> %s

🚨 <b>Alerts:</b>
%s`, hostname, time.Now().Format("2006-01-02 15:04:05"), strings.Join(alerts, "\n"))

						telegram.SendToTelegram(currentCfg.TelegramToken, currentCfg.ChatID, alertMessage)

						// send analytics to backend only if in cloud mode
						if currentCfg.IsCloudMode() {
							sendAlertAnalytics(currentCfg, alerts, currentMetrics)
						}
					} else {
						// if thresholds are not exceeded, send regular analytics
						if currentCfg.IsCloudMode() {
							sendServiceAnalytics(currentCfg, "system_monitoring", currentMetrics)
						}
					}

				case <-updateTicker.C:
					// check for updates once per day
					if cfg.TelegramToken != "" && cfg.ChatID != 0 {
						// get current version
						cmd := exec.Command("moniq", "--version")
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

							resp, err := http.Get(constants.VERSIONS_URL)
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
<code>moniq update</code>

📊 <b>Server:</b> %s
⏰ <b>Check Time:</b> %s`, currentVersion, latestVersion, hostname, time.Now().Format("2006-01-02 15:04:05"))

											telegram.SendToTelegram(cfg.TelegramToken, cfg.ChatID, updateMessage)
										}
									}
								}
							}
						}
					}

				case <-sigChan:
					// Graceful shutdown
					if cfg.TelegramToken != "" && cfg.ChatID != 0 {
						hostname, _ := os.Hostname()
						ipAddress, _ := metrics.GetIPAddress()
						osName, _ := metrics.GetOSName()
						uptime, _ := metrics.GetUptime()

						shutdownMessage := fmt.Sprintf(`🛑 <b>Moniq Monitoring Stopped</b>

📊 <b>Server Information:</b>
• Hostname: %s
• OS: %s
• IP: %s
• Uptime: %s

⏰ <b>Shutdown Time:</b> %s

🔧 <b>Status:</b> Monitoring service stopped gracefully`, hostname, osName, ipAddress, uptime, time.Now().Format("2006-01-02 15:04:05"))

						telegram.SendToTelegram(cfg.TelegramToken, cfg.ChatID, shutdownMessage)

						// send service stop analytics
						if currentMetrics, err := metrics.GetMetrics(); err == nil {
							sendServiceAnalytics(cfg, "service_stop", currentMetrics)
						}
					}

					// log service stop
					if logFile, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
						defer logFile.Close()
						logFile.WriteString(fmt.Sprintf("[%s] INFO: Service stopped - PID: %d\n",
							time.Now().Format("2006-01-02 15:04:05"), pid))
					}

					// remove PID file
					os.Remove(constants.PID_FILE)
					return
				}
			}
		},
	}

	// uninstall command
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Completely remove Moniq CLI from the system",
		Long: `Completely remove Moniq CLI from the system.

This command will:
• Stop the monitoring service
• Remove the binary from PATH
• Delete configuration files
• Remove autostart services
• Clean up all Moniq-related files

Examples:
  moniq uninstall        # Remove Moniq CLI completely
  moniq uninstall --yes  # Skip confirmation prompt`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Uninstall Moniq CLI")

			// check if --yes flag is set
			skipConfirm := cmd.Flags().Lookup("yes").Changed

			if !skipConfirm {
				ui.PrintStatus("warning", "This will completely remove Moniq CLI from your system!")
				ui.PrintStatus("warning", "This will completely remove Moniq CLI from your system!")
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
			if cfg.AuthToken != "" && cfg.ServerToken != "" {
				ui.PrintStatus("info", "Notifying backend about uninstall...")
				if sendUninstallNotification(cfg.AuthToken, cfg.ServerToken) {
					ui.PrintStatus("success", "Backend notified about uninstall")
				} else {
					ui.PrintStatus("warning", "Could not notify backend (continuing with uninstall)")
				}
			}

			// remove autostart services FIRST (before stopping service)
			switch runtime.GOOS {
			case "linux":
				homeDir, _ := os.UserHomeDir()
				systemdService := homeDir + "/.config/systemd/user/moniq.service"
				if _, err := os.Stat(systemdService); err == nil {
					exec.Command("systemctl", "--user", "disable", "moniq.service").Run()
					exec.Command("systemctl", "--user", "stop", "moniq.service").Run()
					os.Remove(systemdService)
				}
			case "darwin":
				homeDir, _ := os.UserHomeDir()
				launchAgent := homeDir + "/Library/LaunchAgents/com.moniq.monitor.plist"
				if _, err := os.Stat(launchAgent); err == nil {
					exec.Command("launchctl", "unload", launchAgent).Run()
					os.Remove(launchAgent)
				}
			}

			// remove configuration directory
			configDir := os.Getenv("HOME") + "/.moniq"
			if err := os.RemoveAll(configDir); err == nil {
				ui.PrintStatus("success", "Configuration directory removed: "+configDir)
			} else {
				ui.PrintStatus("warning", "Could not remove configuration directory")
			}

			// remove log files
			logFiles := []string{
				"/tmp/moniq.log",
				"/tmp/moniq.pid",
			}

			for _, logFile := range logFiles {
				if _, err := os.Stat(logFile); err == nil {
					if err := os.Remove(logFile); err == nil {
						ui.PrintStatus("success", "Removed log file: "+logFile)
					}
				}
			}

			// stop ALL moniq processes (after removing config)
			process.KillAllMoniqProcesses()
			ui.PrintStatus("success", "All processes stopped")

			// remove ALL Moniq binaries from PATH LAST
			binaryPaths := []string{
				"/usr/local/bin/moniq",
				"/usr/bin/moniq",
				os.Getenv("HOME") + "/.local/bin/moniq",
			}

			// also search for any other moniq binaries in PATH
			pathDirs := strings.Split(os.Getenv("PATH"), ":")
			for _, dir := range pathDirs {
				if strings.Contains(dir, "moniq") || strings.Contains(dir, ".local") || strings.Contains(dir, "bin") {
					potentialPath := filepath.Join(dir, "moniq")
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
				ui.PrintStatus("warning", "Could not find any Moniq binaries in standard locations")
			}

			ui.PrintStatus("success", "Moniq CLI completely removed from the system")
			ui.PrintSectionEnd()
		},
	}

	// add --yes flag to uninstall command
	uninstallCmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	// cleanup command
	cleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old backup files and duplicate processes",
		Long: `Clean up old backup files created during updates and kill duplicate moniq processes.
This will remove specific old backup files, clean up files older than 30 days, and ensure only one moniq daemon is running.

Examples:
  moniq cleanup          # Clean up old backups and processes`,
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
			backupDir = backupDir[:len(backupDir)-len("/moniq")] // Adjust path
			removedCount := 0
			for i := 3; i <= 10; i++ { // Remove specific old backups
				backupFile := fmt.Sprintf("%s/moniq.backup.%d", backupDir, i)
				if _, err := os.Stat(backupFile); err == nil {
					os.Remove(backupFile)
					removedCount++
				}
			}
			cmd2 := exec.Command("find", backupDir, "-name", "moniq.backup.*", "-mtime", "+30", "-delete")
			cmd2.Run() // Ignore errors
			ui.PrintStatus("success", fmt.Sprintf("Cleanup completed. Removed %d old backup files", removedCount))
			ui.PrintSectionEnd()
		},
	}

	// force cleanup command
	forceCleanupCmd := &cobra.Command{
		Use:   "force-cleanup",
		Short: "Force cleanup of all duplicate processes and zombie processes",
		Long: `Force cleanup of all duplicate moniq processes and zombie processes.
This command will kill ALL moniq daemon processes and clean up any zombie processes.
Use this when you have multiple processes running and need a fresh start.

Examples:
  moniq force-cleanup    # Kill all processes and start fresh`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Force Cleanup of All Processes")

			ui.PrintStatus("warning", "This will kill ALL moniq daemon processes!")

			// kill all moniq daemon processes
			process.KillAllMoniqProcesses()

			ui.PrintStatus("success", "Force cleanup completed. All processes killed.")
			ui.PrintStatus("info", "Run 'moniq start' to start fresh monitoring service.")
			ui.PrintSectionEnd()
		},
	}

	// config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configure Telegram bot and backend analytics",
		Long: `Configure Telegram bot token and group ID.
This allows you to set up or change your configuration for notifications.

Use 'moniq config show' to see current settings.`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Configuration")

			if len(args) == 0 {
				ui.PrintStatus("error", "Usage: moniq config [token=...|group=...|show]")
				ui.PrintStatus("info", "Run 'moniq config show' to see current settings")
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
				ui.PrintStatus("error", "Use 'moniq auth login <token>' instead")
				ui.PrintSectionEnd()
				return

			default:
				ui.PrintStatus("error", "Unknown setting. Use: token, group, or show")
				ui.PrintSectionEnd()
				return
			}

			// save configuration (only if registerServer was not called)
			if cfg.AuthToken == "" || cfg.ServerToken != "" {
				err := config.SaveConfig(cfg)
				if err != nil {
					ui.PrintStatus("error", fmt.Sprintf("Failed to save config: %v", err))
					return
				}
			}

			ui.PrintStatus("success", "Configuration saved successfully")
			ui.PrintStatus("info", "Run 'moniq restart' to apply changes to the monitoring service")
			ui.PrintSectionEnd()
		},
	}

	// autostart command
	autostartCmd := &cobra.Command{
		Use:   "autostart",
		Short: "Enable or disable autostart on boot",
		Long: `Enable or disable autostart on boot.
This creates systemd service (Linux) or launchd service (macOS) to start moniq automatically.
Examples:
  moniq autostart enable   # Enable autostart
  moniq autostart disable  # Disable autostart
  moniq autostart status   # Check autostart status`,
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
		Long: `Manage authentication for Moniq CLI.

Commands:
  login    Login with authentication token
  logout   Logout and clear authentication
  status   Show authentication status`,
	}

	// login subcommand
	loginCmd := &cobra.Command{
		Use:   "login [token]",
		Short: "Login with authentication token",
		Long: `Login to Moniq CLI with your authentication token.

Examples:
  moniq auth login your_token_here
  moniq auth login eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...`,
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

			// if we already have server_token, transfer ownership
			if cfg.ServerToken != "" && cfg.AuthToken != "" {
				ui.PrintStatus("info", "Server is already registered, transferring ownership...")

				if !transferServerOwnership(cfg.AuthToken, newToken, cfg.ServerToken) {
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

			// update auth_token (server_token is already saved in registerServer)
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
		Long:  `Logout from Moniq CLI and clear authentication token.`,
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
					if cfg.ServerToken != "" {
						return "Yes"
					}
					return "No"
				}())
			} else {
				ui.PrintStatus("warning", "Not authenticated")
				ui.PrintStatus("info", "Run 'moniq auth login <token>' to authenticate")
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
				ui.PrintStatus("info", "Run 'moniq auth login <token>' to set a token")
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
Description=Moniq System Monitor
After=network.target

[Service]
Type=simple
ExecStart=%s daemon
Restart=always
RestartSec=10
Environment=PATH=%s:/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=default.target`, executable, executable[:len(executable)-len("/moniq")])

		serviceFile := systemdDir + "/moniq.service"
		if err := os.WriteFile(serviceFile, []byte(serviceContent), 0644); err != nil {
			ui.PrintStatus("error", "Failed to create systemd service file")
			return
		}

		// enable and start service
		exec.Command("systemctl", "--user", "daemon-reload").Run()
		exec.Command("systemctl", "--user", "enable", "moniq.service").Run()
		exec.Command("systemctl", "--user", "start", "moniq.service").Run()

		ui.PrintStatus("success", "Systemd service created and enabled")
		ui.PrintStatus("info", "Moniq will start automatically on boot")

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
	<string>com.moniq.monitor</string>
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

		plistFile := launchAgentsDir + "/com.moniq.monitor.plist"
		if err := os.WriteFile(plistFile, []byte(plistContent), 0644); err != nil {
			ui.PrintStatus("error", "Failed to create launchd plist file")
			return
		}

		// load the service
		exec.Command("launchctl", "load", plistFile).Run()

		ui.PrintStatus("success", "Launchd service created and enabled")
		ui.PrintStatus("info", "Moniq will start automatically on boot")

	default:
		ui.PrintStatus("error", "Autostart not supported on this operating system")
	}
}

func disableAutostart() {
	switch runtime.GOOS {
	case "linux":
		// disable systemd service (without stopping to avoid duplicate Telegram messages)
		exec.Command("systemctl", "--user", "disable", "moniq.service").Run()

		// remove service file
		homeDir, _ := os.UserHomeDir()
		serviceFile := homeDir + "/.config/systemd/user/moniq.service"
		os.Remove(serviceFile)

		ui.PrintStatus("success", "Systemd service disabled and removed")

	case "darwin":
		// unload launchd service
		exec.Command("launchctl", "unload", "~/Library/LaunchAgents/com.moniq.monitor.plist").Run()

		// remove plist file
		homeDir, _ := os.UserHomeDir()
		plistFile := homeDir + "/Library/LaunchAgents/com.moniq.monitor.plist"
		os.Remove(plistFile)

		ui.PrintStatus("success", "Launchd service disabled and removed")

	default:
		ui.PrintStatus("error", "Autostart not supported on this operating system")
	}
}

func checkAutostartStatus() {
	switch runtime.GOOS {
	case "linux":
		// check systemd service status
		cmd := exec.Command("systemctl", "--user", "is-enabled", "moniq.service")
		if err := cmd.Run(); err == nil {
			ui.PrintStatus("success", "Autostart is enabled (systemd)")
		} else {
			ui.PrintStatus("info", "Autostart is disabled (systemd)")
		}

	case "darwin":
		// check launchd service status
		homeDir, _ := os.UserHomeDir()
		plistFile := homeDir + "/Library/LaunchAgents/com.moniq.monitor.plist"
		if _, err := os.Stat(plistFile); err == nil {
			ui.PrintStatus("success", "Autostart is enabled (launchd)")
		} else {
			ui.PrintStatus("info", "Autostart is disabled (launchd)")
		}

	default:
		ui.PrintStatus("error", "Autostart not supported on this operating system")
	}
}
