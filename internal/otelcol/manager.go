// Package otelcol manages the OpenTelemetry Collector subprocess for host metrics collection.
// This replaces gopsutil-based collection with the official OTel Collector agent.
package otelcol

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"catops/internal/logger"
)

const (
	// OTel Collector version to use
	CollectorVersion = "0.116.0"

	// Download URLs
	BaseDownloadURL = "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download"

	// File paths
	CollectorDir     = "/.catops/otelcol"
	CollectorBinary  = "otelcol-contrib"
	CollectorConfig  = "config.yaml"
	CollectorPIDFile = "otelcol.pid"
)

// Manager handles OTel Collector lifecycle
type Manager struct {
	homeDir    string
	configPath string
	binaryPath string
	pidFile    string
	cmd        *exec.Cmd
	mu         sync.Mutex
	running    bool
}

// Config holds collector configuration parameters
type Config struct {
	Endpoint           string
	AuthToken          string
	ServerID           string
	Hostname           string
	CollectionInterval int // seconds
}

// NewManager creates a new OTel Collector manager
func NewManager() (*Manager, error) {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return nil, fmt.Errorf("HOME environment variable not set")
	}

	collectorDir := filepath.Join(homeDir, ".catops", "otelcol")

	return &Manager{
		homeDir:    homeDir,
		configPath: filepath.Join(collectorDir, CollectorConfig),
		binaryPath: filepath.Join(collectorDir, CollectorBinary),
		pidFile:    filepath.Join(collectorDir, CollectorPIDFile),
	}, nil
}

// EnsureInstalled checks if collector is installed, downloads if not
func (m *Manager) EnsureInstalled() error {
	// Create directory if needed
	collectorDir := filepath.Dir(m.binaryPath)
	if err := os.MkdirAll(collectorDir, 0755); err != nil {
		return fmt.Errorf("failed to create collector directory: %w", err)
	}

	// Check if binary exists and is executable
	if _, err := os.Stat(m.binaryPath); err == nil {
		// Binary exists, verify it works
		cmd := exec.Command(m.binaryPath, "--version")
		if err := cmd.Run(); err == nil {
			logger.Debug("OTel Collector already installed at %s", m.binaryPath)
			return nil
		}
		// Binary broken, remove and re-download
		os.Remove(m.binaryPath)
	}

	// Download collector
	logger.Info("Downloading OTel Collector v%s...", CollectorVersion)
	return m.download()
}

// download fetches the collector binary for current platform
func (m *Manager) download() error {
	// Determine platform
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Map to OTel release naming
	var platform string
	switch goos {
	case "linux":
		platform = "linux"
	case "darwin":
		platform = "darwin"
	default:
		return fmt.Errorf("unsupported OS: %s", goos)
	}

	var arch string
	switch goarch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		return fmt.Errorf("unsupported architecture: %s", goarch)
	}

	// Build download URL
	// Format: otelcol-contrib_0.115.0_darwin_arm64.tar.gz
	filename := fmt.Sprintf("otelcol-contrib_%s_%s_%s.tar.gz", CollectorVersion, platform, arch)
	url := fmt.Sprintf("%s/v%s/%s", BaseDownloadURL, CollectorVersion, filename)

	logger.Info("Downloading from %s", url)

	// Download with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Extract binary from tarball
	return m.extractBinary(resp.Body)
}

// extractBinary extracts otelcol-contrib from tar.gz
func (m *Manager) extractBinary(r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Look for the binary
		if header.Typeflag == tar.TypeReg && strings.HasSuffix(header.Name, "otelcol-contrib") {
			// Create output file
			outFile, err := os.OpenFile(m.binaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("failed to create binary file: %w", err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write binary: %w", err)
			}
			outFile.Close()

			logger.Info("OTel Collector installed at %s", m.binaryPath)
			return nil
		}
	}

	return fmt.Errorf("otelcol-contrib binary not found in archive")
}

// GenerateConfig creates collector config file
func (m *Manager) GenerateConfig(cfg *Config) error {
	interval := cfg.CollectionInterval
	if interval == 0 {
		interval = 15
	}

	config := fmt.Sprintf(`# CatOps OTel Collector Configuration
# Auto-generated - do not edit manually

receivers:
  hostmetrics:
    collection_interval: %ds
    scrapers:
      cpu:
        metrics:
          system.cpu.utilization:
            enabled: true
      memory:
        metrics:
          system.memory.utilization:
            enabled: true
      disk:
        metrics:
          system.disk.io:
            enabled: true
          system.disk.operations:
            enabled: true
      filesystem:
        metrics:
          system.filesystem.utilization:
            enabled: true
      load:
        metrics:
          system.cpu.load_average.1m:
            enabled: true
          system.cpu.load_average.5m:
            enabled: true
          system.cpu.load_average.15m:
            enabled: true
      network:
        metrics:
          system.network.io:
            enabled: true
          system.network.connections:
            enabled: true
          system.network.packets:
            enabled: true
      processes:
        metrics:
          system.processes.count:
            enabled: true
          system.processes.created:
            enabled: true
      process:
        mute_process_exe_error: true
        mute_process_io_error: true
        mute_process_user_error: true
        metrics:
          process.cpu.utilization:
            enabled: true
          process.memory.utilization:
            enabled: true
          process.disk.io:
            enabled: true

processors:
  batch:
    timeout: 10s
    send_batch_size: 100

  resource:
    attributes:
      - key: catops.server.id
        value: "%s"
        action: upsert
      - key: host.name
        value: "%s"
        action: upsert

  filter/processes:
    metrics:
      include:
        match_type: regexp
        metric_names:
          - process\..*
      # Only include processes using >0.1%% CPU or memory
      datapoint:
        - 'value > 0.001'

exporters:
  otlphttp:
    endpoint: "https://%s/api"
    encoding: json
    headers:
      Authorization: "Bearer %s"
      X-CatOps-Server-ID: "%s"
    compression: gzip
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

  debug:
    verbosity: basic
    sampling_initial: 5
    sampling_thereafter: 200

service:
  pipelines:
    metrics:
      receivers: [hostmetrics]
      processors: [batch, resource]
      exporters: [otlphttp]

  telemetry:
    logs:
      level: warn
    metrics:
      level: none
`, interval, cfg.ServerID, cfg.Hostname, cfg.Endpoint, cfg.AuthToken, cfg.ServerID)

	// Write config file
	if err := os.WriteFile(m.configPath, []byte(config), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	logger.Debug("Generated OTel Collector config at %s", m.configPath)
	return nil
}

// Start launches the collector process
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil // Already running
	}

	// Check for existing process
	if m.isRunning() {
		m.running = true
		return nil
	}

	// Verify binary exists
	if _, err := os.Stat(m.binaryPath); err != nil {
		return fmt.Errorf("collector binary not found: %w", err)
	}

	// Verify config exists
	if _, err := os.Stat(m.configPath); err != nil {
		return fmt.Errorf("collector config not found: %w", err)
	}

	// Start collector
	m.cmd = exec.Command(m.binaryPath, "--config", m.configPath)

	// Redirect output to log file
	logFile := filepath.Join(filepath.Dir(m.binaryPath), "otelcol.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logger.Warning("Failed to open log file: %v", err)
	} else {
		m.cmd.Stdout = f
		m.cmd.Stderr = f
	}

	// Start process
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start collector: %w", err)
	}

	// Save PID
	if err := os.WriteFile(m.pidFile, []byte(fmt.Sprintf("%d", m.cmd.Process.Pid)), 0644); err != nil {
		logger.Warning("Failed to write PID file: %v", err)
	}

	m.running = true
	logger.Info("OTel Collector started (PID: %d)", m.cmd.Process.Pid)

	// Monitor process in background
	go m.monitor()

	return nil
}

// Stop gracefully stops the collector
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	// Try graceful shutdown first
	if m.cmd != nil && m.cmd.Process != nil {
		logger.Info("Stopping OTel Collector (PID: %d)...", m.cmd.Process.Pid)

		// Send SIGTERM
		m.cmd.Process.Signal(syscall.SIGTERM)

		// Wait with timeout
		done := make(chan error, 1)
		go func() {
			done <- m.cmd.Wait()
		}()

		select {
		case <-done:
			logger.Info("OTel Collector stopped gracefully")
		case <-time.After(5 * time.Second):
			// Force kill
			m.cmd.Process.Kill()
			logger.Warning("OTel Collector force killed")
		}
	}

	// Clean up PID file
	os.Remove(m.pidFile)
	m.running = false
	m.cmd = nil

	return nil
}

// isRunning checks if collector is already running
func (m *Manager) isRunning() bool {
	// Check PID file
	data, err := os.ReadFile(m.pidFile)
	if err != nil {
		return false
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	if err := process.Signal(syscall.Signal(0)); err != nil {
		// Process doesn't exist, clean up PID file
		os.Remove(m.pidFile)
		return false
	}

	return true
}

// monitor watches the collector process and restarts if needed
func (m *Manager) monitor() {
	if m.cmd == nil {
		return
	}

	err := m.cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		// Unexpected exit
		logger.Error("OTel Collector exited unexpectedly: %v", err)
		m.running = false

		// Auto-restart after delay
		go func() {
			time.Sleep(5 * time.Second)
			if err := m.Start(); err != nil {
				logger.Error("Failed to restart OTel Collector: %v", err)
			}
		}()
	}
}

// Status returns collector status info
func (m *Manager) Status() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := map[string]interface{}{
		"installed": false,
		"running":   false,
		"version":   CollectorVersion,
	}

	if _, err := os.Stat(m.binaryPath); err == nil {
		status["installed"] = true
		status["binary_path"] = m.binaryPath
	}

	if m.isRunning() {
		status["running"] = true
		if data, err := os.ReadFile(m.pidFile); err == nil {
			status["pid"] = strings.TrimSpace(string(data))
		}
	}

	return status
}
