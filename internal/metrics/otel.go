// Package metrics provides OpenTelemetry-based system metrics collection for CatOps CLI.
package metrics

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	constants "catops/config"
)

// =============================================================================
// Global OTel State
// =============================================================================

var (
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter
	otelMu        sync.Mutex
	otelStarted   bool

	// Current OTel config for health checks
	currentOTelConfig *OTelConfig

	// Cached metrics for OTel callbacks
	cachedMetrics *AllMetrics
	cacheMu       sync.RWMutex
)

// =============================================================================
// OTel Setup Functions
// =============================================================================

// StartOTelCollector initializes and starts the OpenTelemetry metrics exporter
func StartOTelCollector(cfg *OTelConfig) error {
	otelMu.Lock()
	defer otelMu.Unlock()

	if otelStarted {
		return nil
	}

	if cfg.Endpoint == "" || cfg.AuthToken == "" || cfg.ServerID == "" {
		return fmt.Errorf("OTLP config incomplete: endpoint, auth_token, and server_id required")
	}

	ctx := context.Background()

	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(cfg.Endpoint),
		otlpmetrichttp.WithURLPath(constants.OTLP_PATH),
		otlpmetrichttp.WithHeaders(map[string]string{
			"Authorization":      "Bearer " + cfg.AuthToken,
			"X-CatOps-Server-ID": cfg.ServerID,
		}),
		// Retry configuration for resilience
		otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  2 * time.Minute,
		}),
		otlpmetrichttp.WithTimeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Store config for health checks
	currentOTelConfig = cfg

	hostname := cfg.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	// Create resource without merging with Default() to avoid schema URL conflicts
	// (resource.Default() uses schema v1.26.0, semconv uses v1.24.0)
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("catops-cli"),
		semconv.ServiceVersion("1.0.0"),
		semconv.HostName(hostname),
		attribute.String("catops.server.id", cfg.ServerID),
		attribute.String("os.type", runtime.GOOS),
	)

	interval := cfg.CollectionInterval
	if interval == 0 {
		interval = 30 * time.Second
	}

	meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter,
				sdkmetric.WithInterval(interval),
			),
		),
	)

	otel.SetMeterProvider(meterProvider)

	meter = meterProvider.Meter("catops.io/cli",
		metric.WithInstrumentationVersion("1.0.0"),
	)

	if err := registerAllMetrics(); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	otelStarted = true
	return nil
}

// StopOTelCollector gracefully shuts down the OTel exporter
func StopOTelCollector() error {
	otelMu.Lock()
	defer otelMu.Unlock()

	if !otelStarted || meterProvider == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := meterProvider.Shutdown(ctx)
	otelStarted = false
	meterProvider = nil
	meter = nil

	return err
}

// ForceFlush forces immediate export of all pending metrics
// Call this after initial metrics collection to send data immediately
func ForceFlush() error {
	otelMu.Lock()
	defer otelMu.Unlock()

	if !otelStarted || meterProvider == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return meterProvider.ForceFlush(ctx)
}

// FlushWithLogging flushes metrics and logs the result
// Returns true if flush was successful
func FlushWithLogging() bool {
	start := time.Now()
	err := ForceFlush()
	elapsed := time.Since(start)

	if err != nil {
		// Log error but don't crash - will retry on next interval
		fmt.Fprintf(os.Stderr, "[%s] [OTLP] SEND FAILED (%v): %v\n",
			time.Now().Format("2006-01-02 15:04:05"), elapsed, err)
		return false
	}

	fmt.Fprintf(os.Stderr, "[%s] [OTLP] SEND OK (%v)\n",
		time.Now().Format("2006-01-02 15:04:05"), elapsed)
	return true
}

// CheckOTelHealth verifies the OTLP exporter can reach the backend
// Returns nil if healthy, error otherwise
func CheckOTelHealth() error {
	otelMu.Lock()
	defer otelMu.Unlock()

	if !otelStarted || meterProvider == nil {
		return fmt.Errorf("OTLP exporter not started")
	}

	if currentOTelConfig == nil {
		return fmt.Errorf("OTLP config not available")
	}

	// Try to flush - if it fails, connection is unhealthy
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return meterProvider.ForceFlush(ctx)
}

// IsOTelStarted returns true if OTel collector is running
func IsOTelStarted() bool {
	otelMu.Lock()
	defer otelMu.Unlock()
	return otelStarted
}

// GetCachedMetrics returns the current cached metrics (thread-safe)
func GetCachedMetrics() *AllMetrics {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return cachedMetrics
}

// SetCachedMetrics updates the cached metrics (thread-safe)
func SetCachedMetrics(m *AllMetrics) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachedMetrics = m
}
