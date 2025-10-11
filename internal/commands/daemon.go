package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	constants "catops/config"
	"catops/internal/analytics"
	"catops/internal/config"
	"catops/internal/logger"
	"catops/internal/metrics"
	"catops/internal/server"
	"catops/internal/telegram"
	"catops/pkg/utils"
)

// NewDaemonCmd creates the daemon command
func NewDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "daemon",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				logger.Error("Error loading config: %v", err)
				os.Exit(1)
			}

			// this is the actual monitoring daemon
			// write PID file
			pid := os.Getpid()
			if f, err := os.Create(constants.PID_FILE); err == nil {
				f.WriteString(fmt.Sprintf("%d", pid))
				f.Close()

				// log service start
				logger.Info("Service started - PID: %d", pid)
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
				analytics.NewSender(cfg, GetCurrentVersion()).SendAll("service_start", currentMetrics)
			}

			// Update server version in database if in cloud mode
			// This ensures version is updated after CLI updates
			if cfg.IsCloudMode() && cfg.AuthToken != "" && cfg.ServerID != "" {
				server.UpdateServerVersion(cfg.AuthToken, GetCurrentVersion(), cfg)
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
						logger.Warning("ALERT: Thresholds exceeded - %s", strings.Join(alerts, ", "))

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
							analytics.NewSender(currentCfg, GetCurrentVersion()).SendAlert(alerts, currentMetrics)
						}
					} else {
						// if thresholds are not exceeded, send regular analytics
						if currentCfg.IsCloudMode() {
							analytics.NewSender(currentCfg, GetCurrentVersion()).SendAll("system_monitoring", currentMetrics)
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
						logger.Info("Version check request started - URL: %s", constants.VERSIONS_URL)

						req, err := utils.CreateCLIRequest("GET", constants.VERSIONS_URL, nil, GetCurrentVersion())
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
						analytics.NewSender(cfg, GetCurrentVersion()).SendAll("service_stop", currentMetrics)
					}

					// log service stop
					logger.Info("Service stopped - PID: %d", pid)

					// remove PID file
					if err := os.Remove(constants.PID_FILE); err != nil && !os.IsNotExist(err) {
						logger.Warning("Failed to remove PID file: %v", err)
					}
					return
				}
			}
		},
	}
}
