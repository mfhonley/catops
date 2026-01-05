package commands

import (
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"runtime"
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
	"catops/internal/service"
	"catops/pkg/utils"
)

// NewDaemonCmd creates the daemon command
// The daemon is now a thin wrapper that:
// 1. Manages OTel Collector lifecycle (metrics collection)
// 2. Sends service events (start/stop)
// 3. Checks for updates
// All alerting and metric analysis is done on the backend
func NewDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "daemon",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			runDaemon()
		},
	}
}

func runDaemon() {
	// Log all exits
	defer func() {
		logger.Info("=== DAEMON EXITING - PID: %d ===", os.Getpid())
	}()

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Error("=== PANIC DETECTED ===")
			logger.Error("Panic value: %v", r)
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			logger.Error("Stack trace:\n%s", string(buf[:n]))
			service.NotifyStopping()
			os.Exit(1)
		}
	}()

	logger.Info("========================================")
	logger.Info("=== DAEMON STARTING - PID: %d ===", os.Getpid())
	logger.Info("========================================")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Error loading config: %v", err)
		os.Exit(1)
	}

	hostname, _ := os.Hostname()

	// Send service start event
	if cfg.IsCloudMode() {
		analytics.NewSender(cfg, GetCurrentVersion()).SendEvent("service_start")
		server.UpdateServerVersion(cfg.AuthToken, GetCurrentVersion(), cfg)
	}

	// Start metrics collection (sends catops.* metrics directly to backend)
	var metricsStarted bool
	if cfg.IsCloudMode() && cfg.AuthToken != "" && cfg.ServerID != "" {
		metricsStarted = startMetricsCollection(cfg, hostname)
	}
	defer func() {
		if metricsStarted {
			logger.Info("Stopping metrics collection...")
			if err := metrics.StopOTelCollector(); err != nil {
				logger.Warning("Failed to stop metrics collection: %v", err)
			}
		}
	}()


	logger.Info("Daemon initialized:")
	logger.Info("  Mode: %s", cfg.Mode)
	logger.Info("  Collection interval: %ds", cfg.CollectionInterval)
	if metricsStarted {
		logger.Info("  Metrics: sending via OTLP to %s", constants.OTLP_ENDPOINT)
		logger.Info("  Alerts: processed on backend")
	} else {
		logger.Info("  Metrics: not started (local mode or missing credentials)")
	}

	// Notify systemd that we're ready (for Type=notify services)
	service.NotifyReady()
	service.NotifyStatus("Monitoring active")

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Update check ticker (once per day)
	updateTicker := time.NewTicker(24 * time.Hour)
	defer updateTicker.Stop()

	// Health check ticker (every 5 minutes)
	healthTicker := time.NewTicker(5 * time.Minute)
	defer healthTicker.Stop()

	// Metrics collection ticker - must run BEFORE OTel SDK reads the cache
	// OTel SDK calls callbacks at CollectionInterval, we collect slightly faster
	metricsInterval := time.Duration(cfg.CollectionInterval) * time.Second
	if metricsInterval == 0 {
		metricsInterval = 30 * time.Second
	}
	metricsTicker := time.NewTicker(metricsInterval)
	defer metricsTicker.Stop()

	// Initial metrics collection (so first OTel export has data)
	if metricsStarted {
		if _, err := metrics.CollectAllMetrics(); err != nil {
			logger.Warning("Initial metrics collection failed: %v", err)
		} else {
			logger.Debug("Initial metrics collected successfully")
			// Force immediate export so dashboard shows data right away
			if err := metrics.ForceFlush(); err != nil {
				logger.Warning("Initial metrics flush failed: %v", err)
			} else {
				logger.Info("Initial metrics sent to backend")
			}
		}
	}

	// Main loop
	for {
		select {
		case <-metricsTicker.C:
			// Collect metrics and update cache for OTel callbacks
			if metricsStarted {
				if _, err := metrics.CollectAllMetrics(); err != nil {
					logger.Debug("Metrics collection error: %v", err)
				}
			}

		case <-healthTicker.C:
			// Log health status and notify systemd watchdog
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			logger.Debug("Health check - goroutines: %d, memory: %.1f MB",
				runtime.NumGoroutine(),
				float64(memStats.Alloc)/1024/1024)

			// Ping systemd watchdog (keeps service alive)
			service.NotifyWatchdog()

		case <-updateTicker.C:
			checkForUpdates()

		case sig := <-sigChan:
			logger.Info("========================================")
			logger.Info("=== SIGNAL RECEIVED: %v ===", sig)
			logger.Info("Initiating graceful shutdown...")
			logger.Info("========================================")

			// Notify systemd we're stopping
			service.NotifyStopping()

			// Send service stop event
			if cfg.IsCloudMode() {
				analytics.NewSender(cfg, GetCurrentVersion()).SendEventSync("service_stop")
			}

			return
		}
	}
}

// startMetricsCollection initializes and starts the built-in metrics collection
func startMetricsCollection(cfg *config.Config, hostname string) bool {
	interval := time.Duration(cfg.CollectionInterval) * time.Second
	if interval == 0 {
		interval = 30 * time.Second
	}

	otelCfg := &metrics.OTelConfig{
		Endpoint:           constants.OTLP_ENDPOINT,
		AuthToken:          cfg.AuthToken,
		ServerID:           cfg.ServerID,
		Hostname:           hostname,
		CollectionInterval: interval,
	}

	if err := metrics.StartOTelCollector(otelCfg); err != nil {
		logger.Error("Failed to start metrics collection: %v", err)
		return false
	}

	logger.Info("Metrics collection started successfully")
	return true
}

// checkForUpdates checks for new CLI versions
func checkForUpdates() {
	currentVersion := strings.TrimPrefix(GetCurrentVersion(), "v")
	logger.Info("Checking for updates...")

	req, err := utils.CreateCLIRequest("GET", constants.VERSIONS_URL, nil, GetCurrentVersion())
	if err != nil {
		logger.Error("Failed to create update check request: %v", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to check for updates: %v", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Error("Failed to parse version API response: %v", err)
		return
	}

	if latestVersion, ok := result["version"].(string); ok {
		latestVersion = strings.TrimPrefix(latestVersion, "v")
		if latestVersion != currentVersion {
			logger.Info("New version available: v%s (current: v%s)", latestVersion, currentVersion)
		} else {
			logger.Info("Already running latest version: v%s", currentVersion)
		}
	}
}
