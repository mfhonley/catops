package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	constants "catops/config"
	"catops/internal/alerts"
	"catops/internal/analytics"
	"catops/internal/config"
	"catops/internal/logger"
	"catops/internal/metrics"
	"catops/internal/process"
	"catops/internal/server"
	"catops/pkg/utils"
)

// NewDaemonCmd creates the daemon command
func NewDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "daemon",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			// CRITICAL: Acquire lock file FIRST (before any other operations)
			// This prevents duplicate daemon processes from starting
			_, err := process.AcquireLock()
			if err != nil {
				logger.Error("Failed to start daemon: %v", err)
				logger.Error("Another CatOps instance may already be running")
				os.Exit(1)
			}
			defer process.ReleaseLock()

			logger.Info("Service started - PID: %d (lock acquired)", os.Getpid())

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				logger.Error("Error loading config: %v", err)
				os.Exit(1)
			}

			// Prepare startup message (always prepare, Telegram is optional)
			hostname, _ := os.Hostname()
			ipAddress, _ := metrics.GetIPAddress()
			osName, _ := metrics.GetOSName()
			uptime, _ := metrics.GetUptime()

			_ = fmt.Sprintf(`🚀 <b>CatOps Monitoring Started</b>

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

			// Telegram notifications removed - Backend handles all notifications

			// send service start analytics (always if in cloud mode)
			if currentMetrics, err := metrics.GetMetrics(); err == nil {
				analytics.NewSender(cfg, GetCurrentVersion()).SendAll("service_start", currentMetrics)
			}

			// Update server version in database if in cloud mode
			// This ensures version is updated after CLI updates
			if cfg.IsCloudMode() && cfg.AuthToken != "" && cfg.ServerID != "" {
				server.UpdateServerVersion(cfg.AuthToken, GetCurrentVersion(), cfg)
			}


			// Initialize metrics buffer and alert manager
			metricsBuffer := metrics.NewMetricsBuffer(cfg.BufferSize)
			alertManager := alerts.NewAlertManager(
				time.Duration(cfg.AlertRenotifyInterval)*time.Minute,
				time.Duration(cfg.AlertResolutionTimeout)*time.Minute,
			)

			logger.Info("Monitoring system initialized:")
			logger.Info("  Collection interval: %ds", cfg.CollectionInterval)
			logger.Info("  Buffer size: %d points", cfg.BufferSize)
			logger.Info("  Spike detection: enabled (sudden: %.1f%%, gradual: %.1f%%)",
				cfg.SuddenSpikeThreshold, cfg.GradualRiseThreshold)
			logger.Info("  Alert deduplication: %v", cfg.AlertDeduplication)

			// Start periodic cleanup of resolved alerts (runs every 5 minutes)
			if cfg.AlertDeduplication {
				go func() {
					cleanupTicker := time.NewTicker(5 * time.Minute)
					defer cleanupTicker.Stop()
					for range cleanupTicker.C {
						removed := alertManager.ClearResolved()
						if removed > 0 {
							logger.Info("Alert cleanup: removed %d resolved alerts from memory", removed)
						}
					}
				}()
				logger.Info("  Alert cleanup: enabled (every 5 minutes)")
			}

			// Start heartbeat sender for active alerts (Phase 2B - every 2 minutes)
			if cfg.IsCloudMode() && cfg.AlertDeduplication {
				go func() {
					heartbeatTicker := time.NewTicker(2 * time.Minute)
					defer heartbeatTicker.Stop()
					for range heartbeatTicker.C {
						currentCfg, err := config.LoadConfig()
						if err != nil {
							currentCfg = cfg
						}

						// Get active alerts from alert manager
						activeAlerts := alertManager.GetActiveAlerts()
						if len(activeAlerts) > 0 {
							sender := analytics.NewSender(currentCfg, GetCurrentVersion())
							for _, activeAlert := range activeAlerts {
								// Send heartbeat for each active alert
								sender.SendHeartbeat(activeAlert.Alert.Fingerprint)
							}
							logger.Debug("Heartbeat sent for %d active alerts", len(activeAlerts))
						}
					}
				}()
				logger.Info("  Alert heartbeat: enabled (every 2 minutes)")
			}

			// setup signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

			// start monitoring loop with configurable interval
			collectionInterval := time.Duration(cfg.CollectionInterval) * time.Second
			ticker := time.NewTicker(collectionInterval)
			updateTicker := time.NewTicker(24 * time.Hour)
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

					// Add metrics to buffer
					metricsBuffer.AddCPUPoint(currentMetrics.CPUUsage)
					metricsBuffer.AddMemoryPoint(currentMetrics.MemoryUsage)
					metricsBuffer.AddDiskPoint(currentMetrics.DiskUsage)

					// Detect alerts with spike detection
					alertsToSend := []alerts.Alert{}
					hostname, _ := os.Hostname()

					// CPU alerts
					cpuAlerts := checkCPUAlerts(currentMetrics.CPUUsage, currentCfg, metricsBuffer)
					alertsToSend = append(alertsToSend, cpuAlerts...)

					// Memory alerts
					memoryAlerts := checkMemoryAlerts(currentMetrics.MemoryUsage, currentCfg, metricsBuffer)
					alertsToSend = append(alertsToSend, memoryAlerts...)

					// Disk alerts
					diskAlerts := checkDiskAlerts(currentMetrics.DiskUsage, currentCfg, metricsBuffer)
					alertsToSend = append(alertsToSend, diskAlerts...)

					// Process alerts through alert manager (deduplication enabled)
					if currentCfg.AlertDeduplication && len(alertsToSend) > 0 {
						for _, alert := range alertsToSend {
							decision := alertManager.ProcessAlert(alert)

							if decision.ShouldNotify {
								// Log alert
								logger.Warning("ALERT [%s]: %s", decision.Reason, alert.Title)


								// Send to backend if in cloud mode (Phase 2B)
								// IMPORTANT: Use decision.Alert.Alert (has fingerprint set by AlertManager)
								if currentCfg.IsCloudMode() {
									analytics.NewSender(currentCfg, GetCurrentVersion()).ProcessAlert(&decision.Alert.Alert, currentMetrics)
								}
							}
						}

						// Check for resolved alerts
						checkResolvedAlerts(currentMetrics, currentCfg, alertManager, hostname)
					} else if len(alertsToSend) > 0 {
						// No deduplication - send all alerts immediately (legacy mode)
						logger.Warning("ALERT: %d alerts detected", len(alertsToSend))

						for _, alert := range alertsToSend {
							// Generate fingerprint even without deduplication (needed for backend)
							decision := alertManager.ProcessAlert(alert)

							// Send to backend if in cloud mode (Phase 2B)
							if currentCfg.IsCloudMode() {
								analytics.NewSender(currentCfg, GetCurrentVersion()).ProcessAlert(&decision.Alert.Alert, currentMetrics)
							}
						}
					}

					// Send regular metrics to backend if no alerts
					if len(alertsToSend) == 0 && currentCfg.IsCloudMode() {
						analytics.NewSender(currentCfg, GetCurrentVersion()).SendAll("system_monitoring", currentMetrics)
					}

				case <-updateTicker.C:
					// check for updates once per day (always check, Telegram is optional)
					// get current version
					currentVersion := GetCurrentVersion()
					currentVersion = strings.TrimPrefix(currentVersion, "v")

					// check API for latest version
					logger.Info("Daily update check started - URL: %s", constants.VERSIONS_URL)

					req, err := utils.CreateCLIRequest("GET", constants.VERSIONS_URL, nil, GetCurrentVersion())
					if err != nil {
						logger.Error("Failed to create update check request: %s", err.Error())
						continue
					}

					client := &http.Client{Timeout: 10 * time.Second}
					resp, err := client.Do(req)
					if err != nil {
						logger.Error("Failed to check for updates: %s", err.Error())
						continue
					}

					defer resp.Body.Close()
					var result map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&result) == nil {
						// API returns "version" field, not "latest_version"
						if latestVersion, ok := result["version"].(string); ok {
							latestVersion = strings.TrimPrefix(latestVersion, "v")

							if latestVersion != currentVersion {
							logger.Info("New version available: v%s (current: v%s)", latestVersion, currentVersion)
							} else {
								logger.Info("Already running latest version: v%s", currentVersion)
							}
						} else {
							logger.Error("Invalid response from version API: missing 'version' field")
						}
					} else {
						logger.Error("Failed to parse version API response")
					}

				case <-sigChan:
					// Graceful shutdown
					// log service stop
					logger.Info("Service stopped - PID: %d", os.Getpid())

					// Lock will be automatically released by defer process.ReleaseLock()
					return
				}
			}
		},
	}
}

// checkCPUAlerts checks for CPU-related alerts with spike detection
func checkCPUAlerts(cpuUsage float64, cfg *config.Config, buffer *metrics.MetricsBuffer) []alerts.Alert {
	var cpuAlerts []alerts.Alert

	// Detect spikes
	spikeResult := buffer.DetectCPUSpike(cfg.SuddenSpikeThreshold, cfg.GradualRiseThreshold)

	// Sudden spike (highest priority)
	if spikeResult.HasSuddenSpike {
		cpuAlerts = append(cpuAlerts, alerts.CreateAlert(
			alerts.AlertTypeCPU,
			alerts.SubTypeSuddenSpike,
			alerts.SeverityCritical,
			"CPU Spike Detected",
			fmt.Sprintf("CPU jumped from %.1f%% to %.1f%% (+%.1f%% in %ds)",
				spikeResult.PreviousValue,
				spikeResult.CurrentValue,
				spikeResult.PercentChange,
				cfg.CollectionInterval),
			cpuUsage,
			cfg.CPUThreshold,
			map[string]interface{}{
				"previous_value": spikeResult.PreviousValue,
				"change_percent": spikeResult.PercentChange,
				"avg_5min":       spikeResult.Stats.Avg,
				"p95_5min":       spikeResult.Stats.P95,
			},
		))
	}

	// Gradual rise (medium priority)
	if spikeResult.HasGradualRise {
		cpuAlerts = append(cpuAlerts, alerts.CreateAlert(
			alerts.AlertTypeCPU,
			alerts.SubTypeGradualRise,
			alerts.SeverityWarning,
			"CPU Trending Up",
			fmt.Sprintf("CPU increased by %.1f%% over last 5 minutes (current: %.1f%%, avg: %.1f%%)",
				spikeResult.ChangeOverWindow,
				spikeResult.CurrentValue,
				spikeResult.Stats.Avg),
			cpuUsage,
			cfg.CPUThreshold,
			map[string]interface{}{
				"change_over_window": spikeResult.ChangeOverWindow,
				"avg_5min":           spikeResult.Stats.Avg,
				"p95_5min":           spikeResult.Stats.P95,
			},
		))
	}

	// Statistical anomaly
	if spikeResult.HasAnomalousValue && !spikeResult.HasSuddenSpike {
		cpuAlerts = append(cpuAlerts, alerts.CreateAlert(
			alerts.AlertTypeCPU,
			alerts.SubTypeAnomaly,
			alerts.SeverityWarning,
			"CPU Anomaly Detected",
			fmt.Sprintf("CPU at %.1f%% (%.1f stddev from 5min avg of %.1f%%)",
				spikeResult.CurrentValue,
				spikeResult.DeviationFromAvg,
				spikeResult.Stats.Avg),
			cpuUsage,
			cfg.CPUThreshold,
			map[string]interface{}{
				"deviation":  spikeResult.DeviationFromAvg,
				"avg_5min":   spikeResult.Stats.Avg,
				"stddev":     spikeResult.Stats.StdDev,
				"p95_5min":   spikeResult.Stats.P95,
			},
		))
	}

	// Simple threshold (lowest priority, only if no spikes detected)
	if cpuUsage >= cfg.CPUThreshold && !spikeResult.HasSuddenSpike && !spikeResult.HasGradualRise {
		cpuAlerts = append(cpuAlerts, alerts.CreateAlert(
			alerts.AlertTypeCPU,
			alerts.SubTypeThreshold,
			alerts.SeverityWarning,
			"CPU High",
			fmt.Sprintf("CPU at %.1f%% (threshold: %.1f%%, 5min avg: %.1f%%, p95: %.1f%%)",
				cpuUsage,
				cfg.CPUThreshold,
				spikeResult.Stats.Avg,
				spikeResult.Stats.P95),
			cpuUsage,
			cfg.CPUThreshold,
			map[string]interface{}{
				"avg_5min": spikeResult.Stats.Avg,
				"p95_5min": spikeResult.Stats.P95,
			},
		))
	}

	return cpuAlerts
}

// checkMemoryAlerts checks for Memory-related alerts with spike detection
func checkMemoryAlerts(memUsage float64, cfg *config.Config, buffer *metrics.MetricsBuffer) []alerts.Alert {
	var memAlerts []alerts.Alert

	// Detect spikes
	spikeResult := buffer.DetectMemorySpike(cfg.SuddenSpikeThreshold, cfg.GradualRiseThreshold)

	// Sudden spike (critical - possible memory leak or attack)
	if spikeResult.HasSuddenSpike {
		memAlerts = append(memAlerts, alerts.CreateAlert(
			alerts.AlertTypeMemory,
			alerts.SubTypeSuddenSpike,
			alerts.SeverityCritical,
			"Memory Spike Detected",
			fmt.Sprintf("Memory jumped from %.1f%% to %.1f%% (+%.1f%% in %ds)",
				spikeResult.PreviousValue,
				spikeResult.CurrentValue,
				spikeResult.PercentChange,
				cfg.CollectionInterval),
			memUsage,
			cfg.MemThreshold,
			map[string]interface{}{
				"previous_value": spikeResult.PreviousValue,
				"change_percent": spikeResult.PercentChange,
				"avg_5min":       spikeResult.Stats.Avg,
				"p95_5min":       spikeResult.Stats.P95,
			},
		))
	}

	// Gradual rise (warning - likely memory leak)
	if spikeResult.HasGradualRise {
		memAlerts = append(memAlerts, alerts.CreateAlert(
			alerts.AlertTypeMemory,
			alerts.SubTypeGradualRise,
			alerts.SeverityWarning,
			"Memory Leak Suspected",
			fmt.Sprintf("Memory increased by %.1f%% over last 5 minutes (current: %.1f%%, avg: %.1f%%). Possible memory leak.",
				spikeResult.ChangeOverWindow,
				spikeResult.CurrentValue,
				spikeResult.Stats.Avg),
			memUsage,
			cfg.MemThreshold,
			map[string]interface{}{
				"change_over_window": spikeResult.ChangeOverWindow,
				"avg_5min":           spikeResult.Stats.Avg,
				"p95_5min":           spikeResult.Stats.P95,
			},
		))
	}

	// Statistical anomaly
	if spikeResult.HasAnomalousValue && !spikeResult.HasSuddenSpike {
		memAlerts = append(memAlerts, alerts.CreateAlert(
			alerts.AlertTypeMemory,
			alerts.SubTypeAnomaly,
			alerts.SeverityWarning,
			"Memory Anomaly Detected",
			fmt.Sprintf("Memory at %.1f%% (%.1f stddev from 5min avg of %.1f%%)",
				spikeResult.CurrentValue,
				spikeResult.DeviationFromAvg,
				spikeResult.Stats.Avg),
			memUsage,
			cfg.MemThreshold,
			map[string]interface{}{
				"deviation":  spikeResult.DeviationFromAvg,
				"avg_5min":   spikeResult.Stats.Avg,
				"stddev":     spikeResult.Stats.StdDev,
				"p95_5min":   spikeResult.Stats.P95,
			},
		))
	}

	// Simple threshold
	if memUsage >= cfg.MemThreshold && !spikeResult.HasSuddenSpike && !spikeResult.HasGradualRise {
		memAlerts = append(memAlerts, alerts.CreateAlert(
			alerts.AlertTypeMemory,
			alerts.SubTypeThreshold,
			alerts.SeverityWarning,
			"Memory High",
			fmt.Sprintf("Memory at %.1f%% (threshold: %.1f%%, 5min avg: %.1f%%, p95: %.1f%%)",
				memUsage,
				cfg.MemThreshold,
				spikeResult.Stats.Avg,
				spikeResult.Stats.P95),
			memUsage,
			cfg.MemThreshold,
			map[string]interface{}{
				"avg_5min": spikeResult.Stats.Avg,
				"p95_5min": spikeResult.Stats.P95,
			},
		))
	}

	return memAlerts
}

// checkDiskAlerts checks for Disk-related alerts with spike detection
func checkDiskAlerts(diskUsage float64, cfg *config.Config, buffer *metrics.MetricsBuffer) []alerts.Alert {
	var diskAlerts []alerts.Alert

	// Detect spikes
	spikeResult := buffer.DetectDiskSpike(cfg.SuddenSpikeThreshold, cfg.GradualRiseThreshold)

	// Sudden spike (critical - disk filling rapidly)
	if spikeResult.HasSuddenSpike {
		diskAlerts = append(diskAlerts, alerts.CreateAlert(
			alerts.AlertTypeDisk,
			alerts.SubTypeSuddenSpike,
			alerts.SeverityCritical,
			"Disk Filling Rapidly",
			fmt.Sprintf("Disk usage jumped from %.1f%% to %.1f%% (+%.1f%% in %ds)",
				spikeResult.PreviousValue,
				spikeResult.CurrentValue,
				spikeResult.PercentChange,
				cfg.CollectionInterval),
			diskUsage,
			cfg.DiskThreshold,
			map[string]interface{}{
				"previous_value": spikeResult.PreviousValue,
				"change_percent": spikeResult.PercentChange,
				"avg_5min":       spikeResult.Stats.Avg,
				"p95_5min":       spikeResult.Stats.P95,
			},
		))
	}

	// Gradual rise (warning - disk filling steadily)
	if spikeResult.HasGradualRise {
		diskAlerts = append(diskAlerts, alerts.CreateAlert(
			alerts.AlertTypeDisk,
			alerts.SubTypeGradualRise,
			alerts.SeverityWarning,
			"Disk Filling Steadily",
			fmt.Sprintf("Disk usage increased by %.1f%% over last 5 minutes (current: %.1f%%, avg: %.1f%%)",
				spikeResult.ChangeOverWindow,
				spikeResult.CurrentValue,
				spikeResult.Stats.Avg),
			diskUsage,
			cfg.DiskThreshold,
			map[string]interface{}{
				"change_over_window": spikeResult.ChangeOverWindow,
				"avg_5min":           spikeResult.Stats.Avg,
				"p95_5min":           spikeResult.Stats.P95,
			},
		))
	}

	// Simple threshold
	if diskUsage >= cfg.DiskThreshold && !spikeResult.HasSuddenSpike && !spikeResult.HasGradualRise {
		diskAlerts = append(diskAlerts, alerts.CreateAlert(
			alerts.AlertTypeDisk,
			alerts.SubTypeThreshold,
			alerts.SeverityWarning,
			"Disk Space High",
			fmt.Sprintf("Disk at %.1f%% (threshold: %.1f%%, 5min avg: %.1f%%, p95: %.1f%%)",
				diskUsage,
				cfg.DiskThreshold,
				spikeResult.Stats.Avg,
				spikeResult.Stats.P95),
			diskUsage,
			cfg.DiskThreshold,
			map[string]interface{}{
				"avg_5min": spikeResult.Stats.Avg,
				"p95_5min": spikeResult.Stats.P95,
			},
		))
	}

	return diskAlerts
}

// checkResolvedAlerts checks if any active alerts have been resolved
func checkResolvedAlerts(currentMetrics *metrics.Metrics, cfg *config.Config, alertMgr *alerts.AlertManager, hostname string) {
	// Check if CPU alerts resolved
	if currentMetrics.CPUUsage < cfg.CPUThreshold {
		for _, subType := range []alerts.AlertSubType{
			alerts.SubTypeThreshold,
			alerts.SubTypeSuddenSpike,
			alerts.SubTypeGradualRise,
			alerts.SubTypeAnomaly,
		} {
			if decision := alertMgr.CheckResolved(alerts.AlertTypeCPU, subType); decision != nil {
				logger.Info("RESOLVED: CPU alert (fingerprint: %s)", decision.Alert.Alert.Fingerprint)

				// Send resolve notification to backend (Phase 2B)
				if cfg.IsCloudMode() {
					analytics.NewSender(cfg, GetCurrentVersion()).ResolveAlert(decision.Alert.Alert.Fingerprint)
				}
			}
		}
	}

	// Check if Memory alerts resolved
	if currentMetrics.MemoryUsage < cfg.MemThreshold {
		for _, subType := range []alerts.AlertSubType{
			alerts.SubTypeThreshold,
			alerts.SubTypeSuddenSpike,
			alerts.SubTypeGradualRise,
			alerts.SubTypeAnomaly,
		} {
			if decision := alertMgr.CheckResolved(alerts.AlertTypeMemory, subType); decision != nil {
				logger.Info("RESOLVED: Memory alert (fingerprint: %s)", decision.Alert.Alert.Fingerprint)

				// Send resolve notification to backend (Phase 2B)
				if cfg.IsCloudMode() {
					analytics.NewSender(cfg, GetCurrentVersion()).ResolveAlert(decision.Alert.Alert.Fingerprint)
				}
			}
		}
	}

	// Check if Disk alerts resolved
	if currentMetrics.DiskUsage < cfg.DiskThreshold {
		for _, subType := range []alerts.AlertSubType{
			alerts.SubTypeThreshold,
			alerts.SubTypeSuddenSpike,
			alerts.SubTypeGradualRise,
		} {
			if decision := alertMgr.CheckResolved(alerts.AlertTypeDisk, subType); decision != nil {
				logger.Info("RESOLVED: Disk alert (fingerprint: %s)", decision.Alert.Alert.Fingerprint)

				// Send resolve notification to backend (Phase 2B)
				if cfg.IsCloudMode() {
					analytics.NewSender(cfg, GetCurrentVersion()).ResolveAlert(decision.Alert.Alert.Fingerprint)
				}
			}
		}
	}
}

// getSeverityEmoji returns emoji for alert severity
func getSeverityEmoji(severity alerts.AlertSeverity) string {
	switch severity {
	case alerts.SeverityCritical:
		return "🔴"
	case alerts.SeverityWarning:
		return "🟡"
	case alerts.SeverityInfo:
		return "🔵"
	default:
		return "⚪"
	}
}
