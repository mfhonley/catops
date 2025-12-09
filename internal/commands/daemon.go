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
	"catops/internal/otelcol"
	"catops/internal/process"
	"catops/internal/server"
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
			process.ReleaseLock()
			os.Exit(1)
		}
	}()

	// Acquire lock file
	if _, err := process.AcquireLock(); err != nil {
		logger.Error("Failed to start daemon: %v", err)
		logger.Error("Another CatOps instance may already be running")
		os.Exit(1)
	}
	defer func() {
		logger.Info("Releasing lock file")
		process.ReleaseLock()
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

	// Start OTel Collector (handles all metrics collection and export)
	var otelManager *otelcol.Manager
	if cfg.IsCloudMode() && cfg.AuthToken != "" && cfg.ServerID != "" {
		otelManager = startOTelCollector(cfg, hostname)
	}
	defer func() {
		if otelManager != nil {
			logger.Info("Stopping OTel Collector...")
			if err := otelManager.Stop(); err != nil {
				logger.Warning("Failed to stop OTel Collector: %v", err)
			}
		}
	}()

	logger.Info("Daemon initialized:")
	logger.Info("  Mode: %s", cfg.Mode)
	logger.Info("  Collection interval: %ds", cfg.CollectionInterval)
	if otelManager != nil {
		logger.Info("  OTel Collector: running")
		logger.Info("  Metrics: sent via OTLP to %s", constants.OTLP_ENDPOINT)
		logger.Info("  Alerts: processed on backend")
	} else {
		logger.Info("  OTel Collector: not started (local mode or missing credentials)")
	}

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Update check ticker (once per day)
	updateTicker := time.NewTicker(24 * time.Hour)
	defer updateTicker.Stop()

	// Health check ticker (every 5 minutes)
	healthTicker := time.NewTicker(5 * time.Minute)
	defer healthTicker.Stop()

	// Main loop
	for {
		select {
		case <-healthTicker.C:
			// Log health status
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			logger.Debug("Health check - goroutines: %d, memory: %.1f MB",
				runtime.NumGoroutine(),
				float64(memStats.Alloc)/1024/1024)

			// Check if OTel Collector is still running
			if otelManager != nil {
				status := otelManager.Status()
				if running, ok := status["running"].(bool); ok && !running {
					logger.Warning("OTel Collector not running, attempting restart...")
					if err := otelManager.Start(); err != nil {
						logger.Error("Failed to restart OTel Collector: %v", err)
					}
				}
			}

		case <-updateTicker.C:
			checkForUpdates()

		case sig := <-sigChan:
			logger.Info("========================================")
			logger.Info("=== SIGNAL RECEIVED: %v ===", sig)
			logger.Info("Initiating graceful shutdown...")
			logger.Info("========================================")

			// Send service stop event
			if cfg.IsCloudMode() {
				analytics.NewSender(cfg, GetCurrentVersion()).SendEventSync("service_stop")
			}

			return
		}
	}
}

// startOTelCollector initializes and starts the OTel Collector
func startOTelCollector(cfg *config.Config, hostname string) *otelcol.Manager {
	manager, err := otelcol.NewManager()
	if err != nil {
		logger.Error("Failed to create OTel Collector manager: %v", err)
		return nil
	}

	// Ensure collector is installed
	if err := manager.EnsureInstalled(); err != nil {
		logger.Error("Failed to install OTel Collector: %v", err)
		return nil
	}

	// Generate config
	otelCfg := &otelcol.Config{
		Endpoint:           constants.OTLP_ENDPOINT,
		AuthToken:          cfg.AuthToken,
		ServerID:           cfg.ServerID,
		Hostname:           hostname,
		CollectionInterval: cfg.CollectionInterval,
	}
	if err := manager.GenerateConfig(otelCfg); err != nil {
		logger.Error("Failed to generate OTel Collector config: %v", err)
		return nil
	}

	// Start collector
	if err := manager.Start(); err != nil {
		logger.Error("Failed to start OTel Collector: %v", err)
		return nil
	}

	logger.Info("OTel Collector started successfully")
	return manager
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
