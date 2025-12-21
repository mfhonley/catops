// Package metrics provides OpenTelemetry-based system metrics collection for CatOps CLI.
//
// This package uses OpenTelemetry instrumentation to collect comprehensive system metrics:
// - System summary (CPU, memory, disk, network aggregated)
// - CPU per-core with user/system/idle/iowait breakdown
// - Memory detailed (used/free/cached/buffers/swap)
// - Disk per-mount with IOPS and throughput
// - Network per-interface with packets and errors
// - Processes (top N by resource usage)
// - Services (auto-detected: nginx, redis, postgres, etc.)
// - Containers (Docker/Podman metrics)
package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	constants "catops/config"
)

// =============================================================================
// Types
// =============================================================================

// ServiceType represents the type of detected service
type ServiceType string

const (
	ServiceTypeNginx      ServiceType = "nginx"
	ServiceTypeApache     ServiceType = "apache"
	ServiceTypeRedis      ServiceType = "redis"
	ServiceTypePostgres   ServiceType = "postgres"
	ServiceTypeMySQL      ServiceType = "mysql"
	ServiceTypeMongoDB    ServiceType = "mongodb"
	ServiceTypePythonApp  ServiceType = "python_app"
	ServiceTypeNodeApp    ServiceType = "node_app"
	ServiceTypeGoApp      ServiceType = "go_app"
	ServiceTypeJavaApp    ServiceType = "java_app"
	ServiceTypeDocker     ServiceType = "docker"
	ServiceTypeKubernetes ServiceType = "kubernetes"
	ServiceTypeUnknown    ServiceType = "unknown"
)

// CPUCoreMetrics contains per-core CPU metrics
type CPUCoreMetrics struct {
	CoreID  int     `json:"core_id"`
	Usage   float64 `json:"usage"`
	User    float64 `json:"user"`
	System  float64 `json:"system"`
	Idle    float64 `json:"idle"`
	IOWait  float64 `json:"iowait"`
	IRQ     float64 `json:"irq"`
	SoftIRQ float64 `json:"softirq"`
	Steal   float64 `json:"steal"`
	Guest   float64 `json:"guest"`
	Nice    float64 `json:"nice"`
	FreqMHz uint32  `json:"frequency_mhz"`
}

// MemoryMetrics contains detailed memory breakdown
type MemoryMetrics struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	Available    uint64  `json:"available"`
	Cached       uint64  `json:"cached"`
	Buffers      uint64  `json:"buffers"`
	Shared       uint64  `json:"shared"`
	Slab         uint64  `json:"slab"`
	SwapTotal    uint64  `json:"swap_total"`
	SwapUsed     uint64  `json:"swap_used"`
	SwapFree     uint64  `json:"swap_free"`
	SwapCached   uint64  `json:"swap_cached"`
	UsagePercent float64 `json:"usage_percent"`
	SwapPercent  float64 `json:"swap_percent"`
}

// DiskMetrics contains per-mount disk metrics
type DiskMetrics struct {
	Device          string  `json:"device"`
	MountPoint      string  `json:"mount_point"`
	FSType          string  `json:"fs_type"`
	Total           uint64  `json:"total"`
	Used            uint64  `json:"used"`
	Free            uint64  `json:"free"`
	UsagePercent    float64 `json:"usage_percent"`
	InodesTotal     uint64  `json:"inodes_total"`
	InodesUsed      uint64  `json:"inodes_used"`
	InodesFree      uint64  `json:"inodes_free"`
	InodesPercent   float64 `json:"inodes_percent"`
	IOPSRead        uint32  `json:"iops_read"`
	IOPSWrite       uint32  `json:"iops_write"`
	ThroughputRead  uint64  `json:"throughput_read"`
	ThroughputWrite uint64  `json:"throughput_write"`
}

// NetworkInterfaceMetrics contains per-interface network metrics
type NetworkInterfaceMetrics struct {
	Interface       string   `json:"interface"`
	MACAddress      string   `json:"mac_address"`
	IPAddresses     []string `json:"ip_addresses"`
	IsUp            bool     `json:"is_up"`
	SpeedMbps       uint32   `json:"speed_mbps"`
	MTU             uint16   `json:"mtu"`
	BytesRecv       uint64   `json:"bytes_recv"`
	BytesSent       uint64   `json:"bytes_sent"`
	PacketsRecv     uint64   `json:"packets_recv"`
	PacketsSent     uint64   `json:"packets_sent"`
	ErrorsIn        uint32   `json:"errors_in"`
	ErrorsOut       uint32   `json:"errors_out"`
	DropsIn         uint32   `json:"drops_in"`
	DropsOut        uint32   `json:"drops_out"`
	BytesRecvRate   uint64   `json:"bytes_recv_rate"`
	BytesSentRate   uint64   `json:"bytes_sent_rate"`
	PacketsRecvRate uint32   `json:"packets_recv_rate"`
	PacketsSentRate uint32   `json:"packets_sent_rate"`
}

// ProcessInfo contains detailed process information
type ProcessInfo struct {
	PID           int     `json:"pid"`
	PPID          int     `json:"ppid"`
	Name          string  `json:"name"`
	Command       string  `json:"command"`
	Exe           string  `json:"exe"`
	User          string  `json:"user"`
	UID           int32   `json:"uid"`
	GID           int32   `json:"gid"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryRSS     uint64  `json:"memory_rss"`
	MemoryVMS     uint64  `json:"memory_vms"`
	MemoryShared  uint64  `json:"memory_shared"`
	Status        string  `json:"status"`
	NumThreads    uint16  `json:"num_threads"`
	NumFDs        uint32  `json:"num_fds"`
	IOReadBytes   uint64  `json:"io_read_bytes"`
	IOWriteBytes  uint64  `json:"io_write_bytes"`
	CreateTime    int64   `json:"create_time"`
	CPUTimeUser   float64 `json:"cpu_time_user"`
	CPUTimeSystem float64 `json:"cpu_time_system"`
	Nice          int8    `json:"nice"`
	Priority      int16   `json:"priority"`

	// Legacy fields for UI compatibility
	CPUUsage    float64 `json:"cpu_usage"`    // Alias for CPUPercent
	MemoryUsage float64 `json:"memory_usage"` // Alias for MemoryPercent
	MemoryKB    int64   `json:"memory_kb"`    // MemoryRSS in KB
	TTY         string  `json:"tty"`
	Threads     int     `json:"threads"` // Alias for NumThreads
}

// ServiceInfo contains information about a detected service
type ServiceInfo struct {
	ServiceType       ServiceType `json:"service_type"`
	ServiceName       string      `json:"service_name"`
	PID               int         `json:"pid"`
	PIDs              []int       `json:"pids"`
	Ports             []uint16    `json:"ports"`
	Protocol          string      `json:"protocol"`
	BindAddress       string      `json:"bind_address"`
	CPUPercent        float64     `json:"cpu_percent"`
	MemoryPercent     float64     `json:"memory_percent"`
	MemoryBytes       uint64      `json:"memory_bytes"`
	Version           string      `json:"version"`
	ConfigPath        string      `json:"config_path"`
	Status            string      `json:"status"`
	IsContainer       bool        `json:"is_container"`
	ContainerID       string      `json:"container_id"`
	ContainerName     string      `json:"container_name"`
	HealthStatus      string      `json:"health_status"`
	ConnectionsActive uint32      `json:"connections_active"`
	RecentLogs        []string    `json:"recent_logs"`
	LogSource         string      `json:"log_source"`
}

// ContainerMetrics contains Docker/Podman container metrics
type ContainerMetrics struct {
	ContainerID      string  `json:"container_id"`
	ContainerName    string  `json:"container_name"`
	ImageName        string  `json:"image_name"`
	ImageTag         string  `json:"image_tag"`
	Runtime          string  `json:"runtime"`
	Status           string  `json:"status"`
	Health           string  `json:"health"`
	StartedAt        int64   `json:"started_at"`
	ExitCode         *int16  `json:"exit_code"`
	CPUPercent       float64 `json:"cpu_percent"`
	CPUSystemPercent float64 `json:"cpu_system_percent"`
	MemoryUsage      uint64  `json:"memory_usage"`
	MemoryLimit      uint64  `json:"memory_limit"`
	MemoryPercent    float64 `json:"memory_percent"`
	NetRxBytes       uint64  `json:"net_rx_bytes"`
	NetTxBytes       uint64  `json:"net_tx_bytes"`
	BlockReadBytes   uint64  `json:"block_read_bytes"`
	BlockWriteBytes  uint64  `json:"block_write_bytes"`
	PIDsCurrent      uint32  `json:"pids_current"`
	PIDsLimit        uint32  `json:"pids_limit"`
	Ports            string  `json:"ports"`
	Labels           string  `json:"labels"`
}

// SystemSummary contains aggregated system metrics for the main dashboard
type SystemSummary struct {
	// CPU
	CPUUsage  float64 `json:"cpu_usage"`
	CPUUser   float64 `json:"cpu_user"`
	CPUSystem float64 `json:"cpu_system"`
	CPUIdle   float64 `json:"cpu_idle"`
	CPUIOWait float64 `json:"cpu_iowait"`
	CPUSteal  float64 `json:"cpu_steal"`
	CPUCores  uint16  `json:"cpu_cores"`

	// Load
	Load1m  float64 `json:"load_1m"`
	Load5m  float64 `json:"load_5m"`
	Load15m float64 `json:"load_15m"`

	// Memory
	MemoryUsage     float64 `json:"memory_usage"`
	MemoryTotal     uint64  `json:"memory_total"`
	MemoryUsed      uint64  `json:"memory_used"`
	MemoryFree      uint64  `json:"memory_free"`
	MemoryAvailable uint64  `json:"memory_available"`
	MemoryCached    uint64  `json:"memory_cached"`
	MemoryBuffers   uint64  `json:"memory_buffers"`

	// Swap
	SwapUsage float64 `json:"swap_usage"`
	SwapTotal uint64  `json:"swap_total"`
	SwapUsed  uint64  `json:"swap_used"`
	SwapFree  uint64  `json:"swap_free"`

	// Disk (aggregated)
	DiskUsage           float64 `json:"disk_usage"`
	DiskTotal           uint64  `json:"disk_total"`
	DiskUsed            uint64  `json:"disk_used"`
	DiskFree            uint64  `json:"disk_free"`
	DiskIOPSRead        uint32  `json:"disk_iops_read"`
	DiskIOPSWrite       uint32  `json:"disk_iops_write"`
	DiskThroughputRead  uint64  `json:"disk_throughput_read"`
	DiskThroughputWrite uint64  `json:"disk_throughput_write"`

	// Network (aggregated)
	NetBytesRecv   uint64 `json:"net_bytes_recv"`
	NetBytesSent   uint64 `json:"net_bytes_sent"`
	NetPacketsRecv uint64 `json:"net_packets_recv"`
	NetPacketsSent uint64 `json:"net_packets_sent"`
	NetErrorsIn    uint32 `json:"net_errors_in"`
	NetErrorsOut   uint32 `json:"net_errors_out"`
	NetDropsIn     uint32 `json:"net_drops_in"`
	NetDropsOut    uint32 `json:"net_drops_out"`
	NetConnections uint32 `json:"net_connections"`

	// Connection states
	NetConnectionsEstablished uint32 `json:"net_connections_established"`
	NetConnectionsTimeWait    uint32 `json:"net_connections_time_wait"`
	NetConnectionsCloseWait   uint32 `json:"net_connections_close_wait"`
	NetConnectionsListen      uint32 `json:"net_connections_listen"`
	NetConnectionsSynSent     uint32 `json:"net_connections_syn_sent"`
	NetConnectionsSynRecv     uint32 `json:"net_connections_syn_recv"`
	NetConnectionsFinWait1    uint32 `json:"net_connections_fin_wait1"`
	NetConnectionsFinWait2    uint32 `json:"net_connections_fin_wait2"`

	// Processes
	ProcessesTotal    uint32 `json:"processes_total"`
	ProcessesRunning  uint32 `json:"processes_running"`
	ProcessesSleeping uint32 `json:"processes_sleeping"`
	ProcessesZombie   uint32 `json:"processes_zombie"`

	// System
	UptimeSeconds uint64 `json:"uptime_seconds"`
	BootTime      int64  `json:"boot_time"`
}

// AllMetrics contains all collected metrics
type AllMetrics struct {
	Timestamp  time.Time                 `json:"timestamp"`
	Summary    *SystemSummary            `json:"summary"`
	CPUCores   []CPUCoreMetrics          `json:"cpu_cores"`
	Memory     *MemoryMetrics            `json:"memory"`
	Disks      []DiskMetrics             `json:"disks"`
	Networks   []NetworkInterfaceMetrics `json:"networks"`
	Processes  []ProcessInfo             `json:"processes"`
	Services   []ServiceInfo             `json:"services"`
	Containers []ContainerMetrics        `json:"containers"`
}

// =============================================================================
// Global State
// =============================================================================

var (
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter
	otelMu        sync.Mutex
	otelStarted   bool

	// Cached metrics for OTel callbacks
	cachedMetrics *AllMetrics
	cacheMu       sync.RWMutex

	// Previous values for rate calculations
	prevNetStats     map[string]net.IOCountersStat
	prevDiskStats    map[string]disk.IOCountersStat
	prevStatsTime    time.Time
	prevStatsMu      sync.RWMutex

	// Delta tracking - для оптимизации отправки метрик
	lastSentMetrics *AllMetrics
	lastSentTime    time.Time
	deltaTrackingMu sync.RWMutex

	// Per-cycle cache for expensive operations (reused within single collection cycle)
	cycleProcesses    []*process.Process
	cycleConnections  []net.ConnectionStat
	cycleCacheMu      sync.RWMutex
	cycleCacheTime    time.Time

	// Process CPU tracking for delta-based calculation (like htop does)
	prevProcCPUTimes map[int32]float64 // PID -> total CPU time (user + system)
	prevProcCPUTime  time.Time
	prevProcCPUMu    sync.RWMutex
)

// OTelConfig holds configuration for OpenTelemetry exporter
type OTelConfig struct {
	Endpoint           string
	AuthToken          string
	ServerID           string
	Hostname           string
	CollectionInterval time.Duration
}

// =============================================================================
// OTel Setup
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
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

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

// =============================================================================
// OTel Metrics Registration
// =============================================================================

func registerAllMetrics() error {
	// System Summary Metrics
	if err := registerSystemSummaryMetrics(); err != nil {
		return err
	}

	// Per-core CPU Metrics
	if err := registerCPUCoreMetrics(); err != nil {
		return err
	}

	// Memory Metrics
	if err := registerMemoryMetrics(); err != nil {
		return err
	}

	// Per-mount Disk Metrics
	if err := registerDiskMetrics(); err != nil {
		return err
	}

	// Per-interface Network Metrics
	if err := registerNetworkMetrics(); err != nil {
		return err
	}

	// Process Metrics
	if err := registerProcessMetrics(); err != nil {
		return err
	}

	// Service Metrics
	if err := registerServiceMetrics(); err != nil {
		return err
	}

	// Container Metrics
	if err := registerContainerMetrics(); err != nil {
		return err
	}

	// Log Metrics
	if err := registerLogMetrics(); err != nil {
		return err
	}

	return nil
}

func registerSystemSummaryMetrics() error {
	// catops.system.* - Main dashboard metrics
	_, err := meter.Float64ObservableGauge(
		"catops.system.cpu",
		metric.WithDescription("System CPU metrics"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(s.CPUUsage, metric.WithAttributes(attribute.String("type", "usage")))
			o.Observe(s.CPUUser, metric.WithAttributes(attribute.String("type", "user")))
			o.Observe(s.CPUSystem, metric.WithAttributes(attribute.String("type", "system")))
			o.Observe(s.CPUIdle, metric.WithAttributes(attribute.String("type", "idle")))
			o.Observe(s.CPUIOWait, metric.WithAttributes(attribute.String("type", "iowait")))
			// Removed: cpu_steal - rarely used, only relevant in virtualization environments
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Float64ObservableGauge(
		"catops.system.load",
		metric.WithDescription("System load averages"),
		metric.WithUnit("1"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(s.Load1m, metric.WithAttributes(attribute.String("period", "1m")))
			o.Observe(s.Load5m, metric.WithAttributes(attribute.String("period", "5m")))
			o.Observe(s.Load15m, metric.WithAttributes(attribute.String("period", "15m")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.memory",
		metric.WithDescription("System memory in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.MemoryTotal), metric.WithAttributes(attribute.String("type", "total")))
			o.Observe(int64(s.MemoryUsed), metric.WithAttributes(attribute.String("type", "used")))
			// Removed: memory_free - duplicate, can be calculated as (total - used)
			o.Observe(int64(s.MemoryAvailable), metric.WithAttributes(attribute.String("type", "available")))
			o.Observe(int64(s.MemoryCached), metric.WithAttributes(attribute.String("type", "cached")))
			o.Observe(int64(s.MemoryBuffers), metric.WithAttributes(attribute.String("type", "buffers")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Float64ObservableGauge(
		"catops.system.memory.usage",
		metric.WithDescription("System memory usage percent"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			o.Observe(m.Summary.MemoryUsage)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.swap",
		metric.WithDescription("System swap in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.SwapTotal), metric.WithAttributes(attribute.String("type", "total")))
			o.Observe(int64(s.SwapUsed), metric.WithAttributes(attribute.String("type", "used")))
			o.Observe(int64(s.SwapFree), metric.WithAttributes(attribute.String("type", "free")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.disk",
		metric.WithDescription("System disk aggregated in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.DiskTotal), metric.WithAttributes(attribute.String("type", "total")))
			o.Observe(int64(s.DiskUsed), metric.WithAttributes(attribute.String("type", "used")))
			// Removed: disk_free - duplicate, can be calculated as (total - used)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Float64ObservableGauge(
		"catops.system.disk.usage",
		metric.WithDescription("System disk usage percent"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			o.Observe(m.Summary.DiskUsage)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.disk.iops",
		metric.WithDescription("System disk IOPS"),
		metric.WithUnit("{operations}/s"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.DiskIOPSRead), metric.WithAttributes(attribute.String("direction", "read")))
			o.Observe(int64(s.DiskIOPSWrite), metric.WithAttributes(attribute.String("direction", "write")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.network",
		metric.WithDescription("System network aggregated bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.NetBytesRecv), metric.WithAttributes(attribute.String("direction", "recv")))
			o.Observe(int64(s.NetBytesSent), metric.WithAttributes(attribute.String("direction", "sent")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Network connection states
	_, err = meter.Int64ObservableGauge(
		"catops.system.network.connections",
		metric.WithDescription("Network connection states"),
		metric.WithUnit("{connections}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.NetConnections), metric.WithAttributes(attribute.String("state", "total")))
			o.Observe(int64(s.NetConnectionsEstablished), metric.WithAttributes(attribute.String("state", "established")))
			o.Observe(int64(s.NetConnectionsTimeWait), metric.WithAttributes(attribute.String("state", "time_wait")))
			o.Observe(int64(s.NetConnectionsCloseWait), metric.WithAttributes(attribute.String("state", "close_wait")))
			o.Observe(int64(s.NetConnectionsListen), metric.WithAttributes(attribute.String("state", "listen")))
			o.Observe(int64(s.NetConnectionsSynSent), metric.WithAttributes(attribute.String("state", "syn_sent")))
			o.Observe(int64(s.NetConnectionsSynRecv), metric.WithAttributes(attribute.String("state", "syn_recv")))
			o.Observe(int64(s.NetConnectionsFinWait1), metric.WithAttributes(attribute.String("state", "fin_wait1")))
			o.Observe(int64(s.NetConnectionsFinWait2), metric.WithAttributes(attribute.String("state", "fin_wait2")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.processes",
		metric.WithDescription("System process counts"),
		metric.WithUnit("{processes}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.ProcessesTotal), metric.WithAttributes(attribute.String("state", "total")))
			o.Observe(int64(s.ProcessesRunning), metric.WithAttributes(attribute.String("state", "running")))
			o.Observe(int64(s.ProcessesSleeping), metric.WithAttributes(attribute.String("state", "sleeping")))
			// Removed: processes_zombie - rarely > 0, not useful for monitoring
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.uptime",
		metric.WithDescription("System uptime in seconds"),
		metric.WithUnit("s"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Summary == nil {
				return nil
			}
			o.Observe(int64(m.Summary.UptimeSeconds))
			return nil
		}),
	)
	return err
}

func registerCPUCoreMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.cpu.core",
		metric.WithDescription("Per-core CPU metrics"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, core := range m.CPUCores {
				attrs := []attribute.KeyValue{
					attribute.Int("core_id", core.CoreID),
				}
				o.Observe(core.Usage, metric.WithAttributes(append(attrs, attribute.String("type", "usage"))...))
				o.Observe(core.User, metric.WithAttributes(append(attrs, attribute.String("type", "user"))...))
				o.Observe(core.System, metric.WithAttributes(append(attrs, attribute.String("type", "system"))...))
				o.Observe(core.Idle, metric.WithAttributes(append(attrs, attribute.String("type", "idle"))...))
				o.Observe(core.IOWait, metric.WithAttributes(append(attrs, attribute.String("type", "iowait"))...))
				o.Observe(core.IRQ, metric.WithAttributes(append(attrs, attribute.String("type", "irq"))...))
				o.Observe(core.SoftIRQ, metric.WithAttributes(append(attrs, attribute.String("type", "softirq"))...))
				o.Observe(core.Steal, metric.WithAttributes(append(attrs, attribute.String("type", "steal"))...))
				o.Observe(core.Nice, metric.WithAttributes(append(attrs, attribute.String("type", "nice"))...))
			}
			return nil
		}),
	)
	return err
}

func registerMemoryMetrics() error {
	_, err := meter.Int64ObservableGauge(
		"catops.memory.detailed",
		metric.WithDescription("Detailed memory metrics in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil || m.Memory == nil {
				return nil
			}
			mem := m.Memory
			o.Observe(int64(mem.Total), metric.WithAttributes(attribute.String("type", "total")))
			o.Observe(int64(mem.Used), metric.WithAttributes(attribute.String("type", "used")))
			o.Observe(int64(mem.Free), metric.WithAttributes(attribute.String("type", "free")))
			o.Observe(int64(mem.Available), metric.WithAttributes(attribute.String("type", "available")))
			o.Observe(int64(mem.Cached), metric.WithAttributes(attribute.String("type", "cached")))
			o.Observe(int64(mem.Buffers), metric.WithAttributes(attribute.String("type", "buffers")))
			o.Observe(int64(mem.Shared), metric.WithAttributes(attribute.String("type", "shared")))
			o.Observe(int64(mem.Slab), metric.WithAttributes(attribute.String("type", "slab")))
			o.Observe(int64(mem.SwapTotal), metric.WithAttributes(attribute.String("type", "swap_total")))
			o.Observe(int64(mem.SwapUsed), metric.WithAttributes(attribute.String("type", "swap_used")))
			o.Observe(int64(mem.SwapFree), metric.WithAttributes(attribute.String("type", "swap_free")))
			o.Observe(int64(mem.SwapCached), metric.WithAttributes(attribute.String("type", "swap_cached")))
			return nil
		}),
	)
	return err
}

func registerDiskMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.disk.mount",
		metric.WithDescription("Per-mount disk metrics"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, d := range m.Disks {
				attrs := []attribute.KeyValue{
					attribute.String("device", d.Device),
					attribute.String("mount_point", d.MountPoint),
					attribute.String("fs_type", d.FSType),
				}
				o.Observe(d.UsagePercent, metric.WithAttributes(append(attrs, attribute.String("metric", "usage_percent"))...))
				o.Observe(d.InodesPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "inodes_percent"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.disk.mount.bytes",
		metric.WithDescription("Per-mount disk bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, d := range m.Disks {
				attrs := []attribute.KeyValue{
					attribute.String("device", d.Device),
					attribute.String("mount_point", d.MountPoint),
					attribute.String("fs_type", d.FSType),
				}
				o.Observe(int64(d.Total), metric.WithAttributes(append(attrs, attribute.String("type", "total"))...))
				o.Observe(int64(d.Used), metric.WithAttributes(append(attrs, attribute.String("type", "used"))...))
				o.Observe(int64(d.Free), metric.WithAttributes(append(attrs, attribute.String("type", "free"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.disk.mount.iops",
		metric.WithDescription("Per-mount disk IOPS"),
		metric.WithUnit("{operations}/s"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, d := range m.Disks {
				attrs := []attribute.KeyValue{
					attribute.String("device", d.Device),
					attribute.String("mount_point", d.MountPoint),
				}
				o.Observe(int64(d.IOPSRead), metric.WithAttributes(append(attrs, attribute.String("direction", "read"))...))
				o.Observe(int64(d.IOPSWrite), metric.WithAttributes(append(attrs, attribute.String("direction", "write"))...))
			}
			return nil
		}),
	)
	return err
}

func registerNetworkMetrics() error {
	_, err := meter.Int64ObservableGauge(
		"catops.network.interface.bytes",
		metric.WithDescription("Per-interface network bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, n := range m.Networks {
				attrs := []attribute.KeyValue{
					attribute.String("interface", n.Interface),
				}
				o.Observe(int64(n.BytesRecv), metric.WithAttributes(append(attrs, attribute.String("direction", "recv"))...))
				o.Observe(int64(n.BytesSent), metric.WithAttributes(append(attrs, attribute.String("direction", "sent"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.network.interface.packets",
		metric.WithDescription("Per-interface network packets"),
		metric.WithUnit("{packets}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, n := range m.Networks {
				attrs := []attribute.KeyValue{
					attribute.String("interface", n.Interface),
				}
				o.Observe(int64(n.PacketsRecv), metric.WithAttributes(append(attrs, attribute.String("direction", "recv"))...))
				o.Observe(int64(n.PacketsSent), metric.WithAttributes(append(attrs, attribute.String("direction", "sent"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.network.interface.errors",
		metric.WithDescription("Per-interface network errors"),
		metric.WithUnit("{errors}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, n := range m.Networks {
				attrs := []attribute.KeyValue{
					attribute.String("interface", n.Interface),
				}
				o.Observe(int64(n.ErrorsIn), metric.WithAttributes(append(attrs, attribute.String("direction", "in"))...))
				o.Observe(int64(n.ErrorsOut), metric.WithAttributes(append(attrs, attribute.String("direction", "out"))...))
				o.Observe(int64(n.DropsIn), metric.WithAttributes(append(attrs, attribute.String("type", "drops_in"))...))
				o.Observe(int64(n.DropsOut), metric.WithAttributes(append(attrs, attribute.String("type", "drops_out"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.network.interface.rate",
		metric.WithDescription("Per-interface network rate bytes/sec"),
		metric.WithUnit("By/s"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, n := range m.Networks {
				attrs := []attribute.KeyValue{
					attribute.String("interface", n.Interface),
				}
				o.Observe(int64(n.BytesRecvRate), metric.WithAttributes(append(attrs, attribute.String("direction", "recv"))...))
				o.Observe(int64(n.BytesSentRate), metric.WithAttributes(append(attrs, attribute.String("direction", "sent"))...))
			}
			return nil
		}),
	)
	return err
}

func registerProcessMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.process",
		metric.WithDescription("Process CPU/Memory usage"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			limit := 20
			if len(m.Processes) < limit {
				limit = len(m.Processes)
			}

			for i := 0; i < limit; i++ {
				p := m.Processes[i]
				attrs := []attribute.KeyValue{
					attribute.Int("pid", p.PID),
					attribute.Int("ppid", p.PPID),
					attribute.String("name", p.Name),
					attribute.String("command", truncateString(p.Command, 200)),
					attribute.String("exe", p.Exe),
					attribute.String("user", p.User),
					attribute.Int("uid", int(p.UID)),
					attribute.Int("gid", int(p.GID)),
					attribute.String("status", p.Status),
					attribute.Int("num_threads", int(p.NumThreads)),
					attribute.Int("num_fds", int(p.NumFDs)),
					attribute.Int64("memory_rss", int64(p.MemoryRSS)),
					attribute.Int64("memory_vms", int64(p.MemoryVMS)),
					attribute.Int64("memory_shared", int64(p.MemoryShared)),
					attribute.Int64("io_read_bytes", int64(p.IOReadBytes)),
					attribute.Int64("io_write_bytes", int64(p.IOWriteBytes)),
					attribute.Int64("create_time", p.CreateTime),
					attribute.Float64("cpu_time_user", p.CPUTimeUser),
					attribute.Float64("cpu_time_system", p.CPUTimeSystem),
					attribute.Int("nice", int(p.Nice)),
					attribute.Int("priority", int(p.Priority)),
				}
				o.Observe(p.CPUPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "cpu"))...))
				o.Observe(p.MemoryPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "memory"))...))
			}
			return nil
		}),
	)
	return err
}

func registerServiceMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.service",
		metric.WithDescription("Service CPU/Memory usage"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, s := range m.Services {
				portsJSON, _ := json.Marshal(s.Ports)
				pidsJSON, _ := json.Marshal(s.PIDs)
				logsJSON, _ := json.Marshal(s.RecentLogs)
				attrs := []attribute.KeyValue{
					attribute.String("service_type", string(s.ServiceType)),
					attribute.String("service_name", s.ServiceName),
					attribute.Int("pid", s.PID),
					attribute.String("pids", string(pidsJSON)),
					attribute.String("ports", string(portsJSON)),
					attribute.String("protocol", s.Protocol),
					attribute.String("bind_address", s.BindAddress),
					attribute.String("version", s.Version),
					attribute.String("config_path", s.ConfigPath),
					attribute.String("status", s.Status),
					attribute.Bool("is_container", s.IsContainer),
					attribute.String("container_id", s.ContainerID),
					attribute.String("container_name", s.ContainerName),
					attribute.String("health_status", s.HealthStatus),
					attribute.Int("connections_active", int(s.ConnectionsActive)),
					attribute.Int64("memory_bytes", int64(s.MemoryBytes)),
					attribute.String("recent_logs", string(logsJSON)),
					attribute.String("log_source", s.LogSource),
				}
				o.Observe(s.CPUPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "cpu"))...))
				o.Observe(s.MemoryPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "memory"))...))
			}
			return nil
		}),
	)
	return err
}

func registerContainerMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.container",
		metric.WithDescription("Container CPU/Memory usage"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			for _, c := range m.Containers {
				var exitCode int64 = -1
				if c.ExitCode != nil {
					exitCode = int64(*c.ExitCode)
				}
				attrs := []attribute.KeyValue{
					attribute.String("container_id", c.ContainerID),
					attribute.String("container_name", c.ContainerName),
					attribute.String("image_name", c.ImageName),
					attribute.String("image_tag", c.ImageTag),
					attribute.String("runtime", c.Runtime),
					attribute.String("status", c.Status),
					attribute.String("health", c.Health),
					attribute.Int64("started_at", c.StartedAt),
					attribute.Int64("exit_code", exitCode),
					attribute.Float64("cpu_system_percent", c.CPUSystemPercent),
					attribute.Int64("memory_usage", int64(c.MemoryUsage)),
					attribute.Int64("memory_limit", int64(c.MemoryLimit)),
					attribute.Int64("net_rx_bytes", int64(c.NetRxBytes)),
					attribute.Int64("net_tx_bytes", int64(c.NetTxBytes)),
					attribute.Int64("block_read_bytes", int64(c.BlockReadBytes)),
					attribute.Int64("block_write_bytes", int64(c.BlockWriteBytes)),
					attribute.Int("pids_current", int(c.PIDsCurrent)),
					attribute.Int("pids_limit", int(c.PIDsLimit)),
					attribute.String("ports", c.Ports),
					attribute.String("labels", c.Labels),
				}
				o.Observe(c.CPUPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "cpu"))...))
				o.Observe(c.MemoryPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "memory"))...))
			}
			return nil
		}),
	)
	return err
}

func registerLogMetrics() error {
	// catops.log - Log entries from services (sent as individual metrics)
	_, err := meter.Int64ObservableGauge(
		"catops.log",
		metric.WithDescription("Log entries from services"),
		metric.WithUnit("{entries}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			m := cachedMetrics
			cacheMu.RUnlock()
			if m == nil {
				return nil
			}

			// Send logs from services
			for _, s := range m.Services {
				if len(s.RecentLogs) == 0 {
					continue
				}

				for idx, logLine := range s.RecentLogs {
					level := detectLogLevel(logLine)
					attrs := []attribute.KeyValue{
						attribute.String("source", s.LogSource),
						attribute.String("source_path", s.ServiceName),
						attribute.String("level", level),
						attribute.String("message", truncateString(logLine, 500)),
						attribute.String("service", s.ServiceName),
						attribute.String("container_id", s.ContainerID),
						attribute.Int("pid", s.PID),
					}
					o.Observe(int64(idx), metric.WithAttributes(attrs...))
				}
			}
			return nil
		}),
	)
	return err
}

// detectLogLevel detects log level from log line content
func detectLogLevel(line string) string {
	lineLower := strings.ToLower(line)
	switch {
	case strings.Contains(lineLower, "fatal") || strings.Contains(lineLower, "critical"):
		return "fatal"
	case strings.Contains(lineLower, "error") || strings.Contains(lineLower, "err"):
		return "error"
	case strings.Contains(lineLower, "warn"):
		return "warn"
	case strings.Contains(lineLower, "debug"):
		return "debug"
	default:
		return "info"
	}
}

// =============================================================================
// Metrics Collection
// =============================================================================

// CollectAllMetrics collects all system metrics and updates the cache
// shouldUpdateMetrics проверяет нужно ли собирать и отправлять новые метрики
// Возвращает true если:
// - Это первый сбор метрик
// - Прошло больше 60 секунд с последней отправки
// - CPU, Memory или Disk изменились больше чем на 1%
func shouldUpdateMetrics(current *AllMetrics) bool {
	deltaTrackingMu.RLock()
	defer deltaTrackingMu.RUnlock()

	// Первый сбор - всегда отправляем
	if lastSentMetrics == nil {
		return true
	}

	// Принудительная отправка каждые 60 секунд (даже если нет изменений)
	if time.Since(lastSentTime) > 60*time.Second {
		return true
	}

	// Проверяем изменения в ключевых метриках (> 1%)
	if current.Summary != nil && lastSentMetrics.Summary != nil {
		cpuDelta := absFloat64(current.Summary.CPUUsage - lastSentMetrics.Summary.CPUUsage)
		memDelta := absFloat64(current.Summary.MemoryUsage - lastSentMetrics.Summary.MemoryUsage)
		diskDelta := absFloat64(current.Summary.DiskUsage - lastSentMetrics.Summary.DiskUsage)

		// Отправляем если любая метрика изменилась больше чем на 1%
		return cpuDelta > 1.0 || memDelta > 1.0 || diskDelta > 1.0
	}

	return false
}

// absFloat64 returns absolute value of float64
func absFloat64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func CollectAllMetrics() (*AllMetrics, error) {
	// Clear per-cycle cache at start of each collection
	clearCycleCache()

	m := &AllMetrics{
		Timestamp: time.Now().UTC(),
	}

	// Collect all metrics in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	// System summary
	wg.Add(1)
	go func() {
		defer wg.Done()
		if summary, err := collectSystemSummary(); err == nil {
			mu.Lock()
			m.Summary = summary
			mu.Unlock()
		} else {
			mu.Lock()
			errs = append(errs, fmt.Errorf("summary: %w", err))
			mu.Unlock()
		}
	}()

	// CPU cores
	wg.Add(1)
	go func() {
		defer wg.Done()
		if cores, err := collectCPUCores(); err == nil {
			mu.Lock()
			m.CPUCores = cores
			mu.Unlock()
		}
	}()

	// Memory
	wg.Add(1)
	go func() {
		defer wg.Done()
		if memory, err := collectMemory(); err == nil {
			mu.Lock()
			m.Memory = memory
			mu.Unlock()
		}
	}()

	// Disks
	wg.Add(1)
	go func() {
		defer wg.Done()
		if disks, err := collectDisks(); err == nil {
			mu.Lock()
			m.Disks = disks
			mu.Unlock()
		}
	}()

	// Networks
	wg.Add(1)
	go func() {
		defer wg.Done()
		if networks, err := collectNetworks(); err == nil {
			mu.Lock()
			m.Networks = networks
			mu.Unlock()
		}
	}()

	// Processes
	wg.Add(1)
	go func() {
		defer wg.Done()
		if processes, err := collectProcesses(30); err == nil {
			mu.Lock()
			m.Processes = processes
			mu.Unlock()
		}
	}()

	// Services
	wg.Add(1)
	go func() {
		defer wg.Done()
		if services, err := GetServices(); err == nil {
			mu.Lock()
			m.Services = services
			mu.Unlock()
		}
	}()

	// Containers
	wg.Add(1)
	go func() {
		defer wg.Done()
		if containers, err := collectContainers(); err == nil {
			mu.Lock()
			m.Containers = containers
			mu.Unlock()
		}
	}()

	wg.Wait()

	// Delta tracking: Проверяем нужно ли обновлять кэш
	shouldUpdate := shouldUpdateMetrics(m)

	if shouldUpdate {
		// Обновляем кэш только если есть значительные изменения
		cacheMu.Lock()
		cachedMetrics = m
		cacheMu.Unlock()

		// Сохраняем как последние отправленные метрики
		deltaTrackingMu.Lock()
		lastSentMetrics = m
		lastSentTime = time.Now()
		deltaTrackingMu.Unlock()

		// Debug: для отладки можно включить через env CATOPS_DEBUG_DELTA=1
		if os.Getenv("CATOPS_DEBUG_DELTA") == "1" {
			fmt.Printf("[Delta Tracking] Sending metrics (CPU: %.1f%%, Mem: %.1f%%, Disk: %.1f%%)\n",
				m.Summary.CPUUsage, m.Summary.MemoryUsage, m.Summary.DiskUsage)
		}
	} else {
		// Нет значительных изменений - возвращаем старые метрики из кэша
		// Это предотвратит отправку данных в OpenTelemetry
		cacheMu.RLock()
		m = cachedMetrics
		cacheMu.RUnlock()

		// Debug: для отладки можно включить через env CATOPS_DEBUG_DELTA=1
		if os.Getenv("CATOPS_DEBUG_DELTA") == "1" {
			fmt.Printf("[Delta Tracking] Skipping send - no significant changes\n")
		}
	}

	if len(errs) > 0 {
		return m, errs[0]
	}

	return m, nil
}

// =============================================================================
// Cached Expensive Operations (called once per collection cycle)
// =============================================================================

// getCachedProcesses returns cached process list or fetches new one
// Cache is valid for 5 seconds (covers entire collection cycle)
func getCachedProcesses() ([]*process.Process, error) {
	cycleCacheMu.RLock()
	if cycleProcesses != nil && time.Since(cycleCacheTime) < 5*time.Second {
		procs := cycleProcesses
		cycleCacheMu.RUnlock()
		return procs, nil
	}
	cycleCacheMu.RUnlock()

	// Fetch new
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	cycleCacheMu.Lock()
	cycleProcesses = procs
	cycleCacheTime = time.Now()
	cycleCacheMu.Unlock()

	return procs, nil
}

// getCachedConnections returns cached TCP connections or fetches new ones
// Cache is valid for 5 seconds (covers entire collection cycle)
func getCachedConnections() ([]net.ConnectionStat, error) {
	cycleCacheMu.RLock()
	if cycleConnections != nil && time.Since(cycleCacheTime) < 5*time.Second {
		conns := cycleConnections
		cycleCacheMu.RUnlock()
		return conns, nil
	}
	cycleCacheMu.RUnlock()

	// Fetch new
	conns, err := net.Connections("tcp")
	if err != nil {
		return nil, err
	}

	cycleCacheMu.Lock()
	cycleConnections = conns
	cycleCacheMu.Unlock()

	return conns, nil
}

// clearCycleCache clears the per-cycle cache (call at start of each collection)
func clearCycleCache() {
	cycleCacheMu.Lock()
	cycleProcesses = nil
	cycleConnections = nil
	cycleCacheMu.Unlock()
}

func collectSystemSummary() (*SystemSummary, error) {
	s := &SystemSummary{
		CPUCores: uint16(runtime.NumCPU()),
	}

	// CPU - use delta-based calculation for accurate real-time usage
	// This is non-blocking and returns instant results (unlike cpu.Percent with sleep)
	if cpuMetrics, err := GetCPUMetrics(); err == nil {
		s.CPUUsage = cpuMetrics.Total
		s.CPUUser = cpuMetrics.User
		s.CPUSystem = cpuMetrics.System
		s.CPUIdle = cpuMetrics.Idle
		s.CPUIOWait = cpuMetrics.Iowait
		s.CPUSteal = cpuMetrics.Steal
	}

	// Load
	if loadAvg, err := load.Avg(); err == nil {
		s.Load1m = loadAvg.Load1
		s.Load5m = loadAvg.Load5
		s.Load15m = loadAvg.Load15
	}

	// Memory
	if vm, err := mem.VirtualMemory(); err == nil {
		s.MemoryUsage = vm.UsedPercent
		s.MemoryTotal = vm.Total
		s.MemoryUsed = vm.Used
		s.MemoryFree = vm.Free
		s.MemoryAvailable = vm.Available
		s.MemoryCached = vm.Cached
		s.MemoryBuffers = vm.Buffers
	}

	// Swap
	if swap, err := mem.SwapMemory(); err == nil {
		s.SwapTotal = swap.Total
		s.SwapUsed = swap.Used
		s.SwapFree = swap.Free
		if swap.Total > 0 {
			s.SwapUsage = swap.UsedPercent
		}
	}

	// Disk - aggregate all mounts (filter pseudo filesystems)
	if partitions, err := disk.Partitions(false); err == nil {
		for _, p := range partitions {
			// Skip pseudo filesystems that report 100% or have no real storage
			if shouldSkipPartition(p) {
				continue
			}
			if usage, err := disk.Usage(p.Mountpoint); err == nil {
				s.DiskTotal += usage.Total
				s.DiskUsed += usage.Used
				s.DiskFree += usage.Free
			}
		}
		// Calculate percentage from aggregated values (consistent with Total/Used sums)
		if s.DiskTotal > 0 {
			s.DiskUsage = float64(s.DiskUsed) / float64(s.DiskTotal) * 100
		}
	}

	// Disk IOPS
	if ioCounters, err := disk.IOCounters(); err == nil {
		prevStatsMu.Lock()
		if prevDiskStats != nil && !prevStatsTime.IsZero() {
			elapsed := time.Since(prevStatsTime).Seconds()
			if elapsed > 0 {
				for device, current := range ioCounters {
					if prev, ok := prevDiskStats[device]; ok {
						s.DiskIOPSRead += uint32(float64(current.ReadCount-prev.ReadCount) / elapsed)
						s.DiskIOPSWrite += uint32(float64(current.WriteCount-prev.WriteCount) / elapsed)
						s.DiskThroughputRead += uint64(float64(current.ReadBytes-prev.ReadBytes) / elapsed)
						s.DiskThroughputWrite += uint64(float64(current.WriteBytes-prev.WriteBytes) / elapsed)
					}
				}
			}
		}
		prevDiskStats = ioCounters
		prevStatsMu.Unlock()
	}

	// Network - aggregate all interfaces
	if netIO, err := net.IOCounters(false); err == nil && len(netIO) > 0 {
		n := netIO[0]
		s.NetBytesRecv = n.BytesRecv
		s.NetBytesSent = n.BytesSent
		s.NetPacketsRecv = n.PacketsRecv
		s.NetPacketsSent = n.PacketsSent
		s.NetErrorsIn = uint32(n.Errin)
		s.NetErrorsOut = uint32(n.Errout)
		s.NetDropsIn = uint32(n.Dropin)
		s.NetDropsOut = uint32(n.Dropout)
	}

	// Connections count and states (use cached)
	if conns, err := getCachedConnections(); err == nil {
		s.NetConnections = uint32(len(conns))

		// Count connection states
		for _, conn := range conns {
			switch conn.Status {
			case "ESTABLISHED":
				s.NetConnectionsEstablished++
			case "TIME_WAIT":
				s.NetConnectionsTimeWait++
			case "CLOSE_WAIT":
				s.NetConnectionsCloseWait++
			case "LISTEN":
				s.NetConnectionsListen++
			case "SYN_SENT":
				s.NetConnectionsSynSent++
			case "SYN_RECV":
				s.NetConnectionsSynRecv++
			case "FIN_WAIT1":
				s.NetConnectionsFinWait1++
			case "FIN_WAIT2":
				s.NetConnectionsFinWait2++
			}
		}
	}

	// Process counts - just count total from cached list
	// Skip per-process Status() calls - too expensive for 200+ processes
	// Running/sleeping/zombie stats are nice-to-have, not critical
	if procs, err := getCachedProcesses(); err == nil {
		s.ProcessesTotal = uint32(len(procs))
		// Note: ProcessesRunning/Sleeping/Zombie left as 0 for performance
		// These require p.Status() syscall on each process which is expensive
	}

	// Uptime
	if uptime, err := host.Uptime(); err == nil {
		s.UptimeSeconds = uptime
	}

	if bootTime, err := host.BootTime(); err == nil {
		s.BootTime = int64(bootTime)
	}

	// Update prev stats time
	prevStatsMu.Lock()
	prevStatsTime = time.Now()
	prevStatsMu.Unlock()

	return s, nil
}

func collectCPUCores() ([]CPUCoreMetrics, error) {
	// Use delta-based calculation for accurate real-time per-core CPU usage
	// This is non-blocking and returns instant results
	perCoreMetrics, err := GetPerCoreCPUDetailed()
	if err != nil {
		return nil, err
	}

	cores := make([]CPUCoreMetrics, len(perCoreMetrics))

	for i, m := range perCoreMetrics {
		cores[i] = CPUCoreMetrics{
			CoreID:  i,
			Usage:   m.Total,
			User:    m.User,
			System:  m.System,
			Idle:    m.Idle,
			IOWait:  m.Iowait,
			Steal:   m.Steal,
			// Note: IRQ, SoftIRQ, Guest, Nice not available in simplified CPUMetrics
			// These are included in System/User time
		}
	}

	return cores, nil
}

func collectMemory() (*MemoryMetrics, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	m := &MemoryMetrics{
		Total:        vm.Total,
		Used:         vm.Used,
		Free:         vm.Free,
		Available:    vm.Available,
		Cached:       vm.Cached,
		Buffers:      vm.Buffers,
		Shared:       vm.Shared,
		Slab:         vm.Slab,
		UsagePercent: vm.UsedPercent,
	}

	if swap, err := mem.SwapMemory(); err == nil {
		m.SwapTotal = swap.Total
		m.SwapUsed = swap.Used
		m.SwapFree = swap.Free
		m.SwapPercent = swap.UsedPercent
	}

	return m, nil
}

func collectDisks() ([]DiskMetrics, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	ioCounters, _ := disk.IOCounters()

	var disks []DiskMetrics

	prevStatsMu.Lock()
	elapsed := time.Since(prevStatsTime).Seconds()
	prevStatsMu.Unlock()

	for _, p := range partitions {
		// Skip pseudo filesystems
		if shouldSkipPartition(p) {
			continue
		}

		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		d := DiskMetrics{
			Device:        p.Device,
			MountPoint:    p.Mountpoint,
			FSType:        p.Fstype,
			Total:         usage.Total,
			Used:          usage.Used,
			Free:          usage.Free,
			UsagePercent:  usage.UsedPercent,
			InodesTotal:   usage.InodesTotal,
			InodesUsed:    usage.InodesUsed,
			InodesFree:    usage.InodesFree,
			InodesPercent: usage.InodesUsedPercent,
		}

		// Calculate IOPS and throughput
		deviceName := strings.TrimPrefix(p.Device, "/dev/")
		if io, ok := ioCounters[deviceName]; ok {
			prevStatsMu.RLock()
			if prevDiskStats != nil && elapsed > 0 {
				if prevIO, ok := prevDiskStats[deviceName]; ok {
					d.IOPSRead = uint32(float64(io.ReadCount-prevIO.ReadCount) / elapsed)
					d.IOPSWrite = uint32(float64(io.WriteCount-prevIO.WriteCount) / elapsed)
					d.ThroughputRead = uint64(float64(io.ReadBytes-prevIO.ReadBytes) / elapsed)
					d.ThroughputWrite = uint64(float64(io.WriteBytes-prevIO.WriteBytes) / elapsed)
				}
			}
			prevStatsMu.RUnlock()
		}

		disks = append(disks, d)
	}

	return disks, nil
}

func collectNetworks() ([]NetworkInterfaceMetrics, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ioCounters, err := net.IOCounters(true) // per-interface
	if err != nil {
		return nil, err
	}

	ioMap := make(map[string]net.IOCountersStat)
	for _, io := range ioCounters {
		ioMap[io.Name] = io
	}

	prevStatsMu.Lock()
	elapsed := time.Since(prevStatsTime).Seconds()
	prevStatsMu.Unlock()

	var networks []NetworkInterfaceMetrics

	for _, iface := range interfaces {
		// Skip loopback
		if strings.HasPrefix(iface.Name, "lo") || strings.HasPrefix(iface.Name, "veth") {
			continue
		}

		n := NetworkInterfaceMetrics{
			Interface:   iface.Name,
			MACAddress:  iface.HardwareAddr,
			IPAddresses: make([]string, 0),
			MTU:         uint16(iface.MTU),
		}

		// Get IP addresses
		for _, addr := range iface.Addrs {
			n.IPAddresses = append(n.IPAddresses, addr.Addr)
		}

		// Check if up
		for _, flag := range iface.Flags {
			if flag == "up" {
				n.IsUp = true
				break
			}
		}

		// Get IO stats
		if io, ok := ioMap[iface.Name]; ok {
			n.BytesRecv = io.BytesRecv
			n.BytesSent = io.BytesSent
			n.PacketsRecv = io.PacketsRecv
			n.PacketsSent = io.PacketsSent
			n.ErrorsIn = uint32(io.Errin)
			n.ErrorsOut = uint32(io.Errout)
			n.DropsIn = uint32(io.Dropin)
			n.DropsOut = uint32(io.Dropout)

			// Calculate rates
			prevStatsMu.RLock()
			if prevNetStats != nil && elapsed > 0 {
				if prevIO, ok := prevNetStats[iface.Name]; ok {
					n.BytesRecvRate = uint64(float64(io.BytesRecv-prevIO.BytesRecv) / elapsed)
					n.BytesSentRate = uint64(float64(io.BytesSent-prevIO.BytesSent) / elapsed)
					n.PacketsRecvRate = uint32(float64(io.PacketsRecv-prevIO.PacketsRecv) / elapsed)
					n.PacketsSentRate = uint32(float64(io.PacketsSent-prevIO.PacketsSent) / elapsed)
				}
			}
			prevStatsMu.RUnlock()
		}

		networks = append(networks, n)
	}

	// Update prev stats
	prevStatsMu.Lock()
	prevNetStats = ioMap
	prevStatsMu.Unlock()

	return networks, nil
}

func collectProcesses(limit int) ([]ProcessInfo, error) {
	procs, err := getCachedProcesses()
	if err != nil {
		return nil, err
	}

	// Get timing info for CPU delta calculation
	prevProcCPUMu.RLock()
	prevTimes := prevProcCPUTimes
	prevTime := prevProcCPUTime
	prevProcCPUMu.RUnlock()

	elapsed := time.Since(prevTime).Seconds()
	numCPU := float64(runtime.NumCPU())

	// Current CPU times map for next cycle
	currentTimes := make(map[int32]float64)

	var processes []ProcessInfo

	for _, p := range procs {
		name, _ := p.Name()
		if name == "catops" || strings.HasPrefix(name, "catops-") {
			continue
		}

		memPercent, _ := p.MemoryPercent()

		// Filter by memory (processes with < 0.1% memory are not interesting)
		if memPercent < 0.1 {
			continue
		}

		pi := ProcessInfo{
			PID:  int(p.Pid),
			Name: name,
		}

		pi.MemoryPercent = float64(memPercent)

		// Get CPU times for delta calculation (non-blocking, just reads /proc/[pid]/stat)
		if times, err := p.Times(); err == nil && times != nil {
			totalTime := times.User + times.System
			currentTimes[p.Pid] = totalTime

			// Calculate CPU% from delta if we have previous data
			if prevTimes != nil && elapsed > 0 {
				if prevTotal, ok := prevTimes[p.Pid]; ok {
					// CPU% = (delta CPU time / elapsed time) * 100 / numCPU
					deltaTime := totalTime - prevTotal
					if deltaTime >= 0 {
						pi.CPUPercent = (deltaTime / elapsed) * 100.0 / numCPU
						if pi.CPUPercent > 100 {
							pi.CPUPercent = 100
						}
					}
				}
			}
		}

		// Minimal syscalls: only cmdline and memory info
		if cmdline, err := p.Cmdline(); err == nil {
			pi.Command = truncateString(cmdline, 200)
		} else {
			pi.Command = name
		}

		if memInfo, err := p.MemoryInfo(); err == nil && memInfo != nil {
			pi.MemoryRSS = memInfo.RSS
		}

		if status, err := p.Status(); err == nil && len(status) > 0 {
			pi.Status = string(status[0])
		}

		// Legacy fields
		pi.CPUUsage = pi.CPUPercent
		pi.MemoryUsage = pi.MemoryPercent
		pi.MemoryKB = int64(pi.MemoryRSS / 1024)

		processes = append(processes, pi)
	}

	// Save current times for next cycle
	prevProcCPUMu.Lock()
	prevProcCPUTimes = currentTimes
	prevProcCPUTime = time.Now()
	prevProcCPUMu.Unlock()

	// Sort by CPU+Memory combined (prioritize CPU, then memory)
	sort.Slice(processes, func(i, j int) bool {
		// Primary sort by CPU, secondary by memory
		if processes[i].CPUPercent != processes[j].CPUPercent {
			return processes[i].CPUPercent > processes[j].CPUPercent
		}
		return processes[i].MemoryPercent > processes[j].MemoryPercent
	})

	if len(processes) > limit {
		return processes[:limit], nil
	}

	return processes, nil
}

func collectContainers() ([]ContainerMetrics, error) {
	// Try docker first
	containers, err := collectDockerContainers()
	if err == nil && len(containers) > 0 {
		return containers, nil
	}

	// Try podman
	containers, err = collectPodmanContainers()
	if err == nil && len(containers) > 0 {
		return containers, nil
	}

	return nil, nil
}

func collectDockerContainers() ([]ContainerMetrics, error) {
	// Single call to docker stats - gets all running containers at once
	// Skip "docker ps" check - if no containers, stats returns empty
	cmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	if len(output) == 0 {
		return nil, nil
	}

	var containers []ContainerMetrics

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var stats struct {
			ID       string `json:"ID"`
			Name     string `json:"Name"`
			CPUPerc  string `json:"CPUPerc"`
			MemUsage string `json:"MemUsage"`
			MemPerc  string `json:"MemPerc"`
		}

		if err := json.Unmarshal([]byte(line), &stats); err != nil {
			continue
		}

		c := ContainerMetrics{
			ContainerID:   stats.ID,
			ContainerName: strings.TrimPrefix(stats.Name, "/"),
			Runtime:       "docker",
			Status:        "running",
		}

		// Parse CPU percentage
		cpuStr := strings.TrimSuffix(stats.CPUPerc, "%")
		if cpu, err := parseFloat(cpuStr); err == nil {
			c.CPUPercent = cpu
		}

		// Parse memory percentage
		memPercStr := strings.TrimSuffix(stats.MemPerc, "%")
		if memPerc, err := parseFloat(memPercStr); err == nil {
			c.MemoryPercent = memPerc
		}

		containers = append(containers, c)
	}

	// Skip docker inspect for each container - too expensive (N syscalls)
	// Basic stats from "docker stats" are enough for monitoring

	return containers, nil
}

func collectPodmanContainers() ([]ContainerMetrics, error) {
	cmd := exec.Command("podman", "ps", "-q")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	if len(output) == 0 {
		return nil, nil
	}

	// Similar to docker but with podman
	cmd = exec.Command("podman", "stats", "--no-stream", "--format", "json")
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	var stats []struct {
		ID      string  `json:"id"`
		Name    string  `json:"name"`
		CPU     float64 `json:"cpu_percent"`
		MemPerc float64 `json:"mem_percent"`
	}

	if err := json.Unmarshal(output, &stats); err != nil {
		return nil, err
	}

	containers := make([]ContainerMetrics, len(stats))
	for i, s := range stats {
		containers[i] = ContainerMetrics{
			ContainerID:   s.ID[:12],
			ContainerName: s.Name,
			Runtime:       "podman",
			Status:        "running",
			CPUPercent:    s.CPU,
			MemoryPercent: s.MemPerc,
		}
	}

	return containers, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// shouldSkipPartition returns true for pseudo filesystems that should be excluded from metrics
func shouldSkipPartition(p disk.PartitionStat) bool {
	// Linux pseudo filesystems
	if strings.HasPrefix(p.Device, "/dev/loop") ||
		p.Fstype == "squashfs" ||
		p.Fstype == "devtmpfs" ||
		p.Fstype == "tmpfs" ||
		p.Fstype == "overlay" {
		return true
	}

	// macOS pseudo filesystems
	if p.Fstype == "devfs" ||
		p.Fstype == "autofs" ||
		p.Fstype == "nullfs" ||
		strings.HasPrefix(p.Device, "map ") {
		return true
	}

	// Skip system volumes that aren't the main data partition
	// On macOS, /System/Volumes/* except Data are system partitions
	if strings.HasPrefix(p.Mountpoint, "/System/Volumes/") &&
		!strings.HasPrefix(p.Mountpoint, "/System/Volumes/Data") {
		return true
	}

	return false
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// =============================================================================
// Legacy API (for backward compatibility with UI)
// =============================================================================

// Metrics contains system metrics for UI display
type Metrics struct {
	CPUUsage      float64 `json:"cpu_usage"`
	DiskUsage     float64 `json:"disk_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	HTTPSRequests int64   `json:"https_requests"`
	IOPS          int64   `json:"iops"`
	IOWait        float64 `json:"io_wait"`
	OSName        string  `json:"os_name"`
	IPAddress     string  `json:"ip_address"`
	Uptime        string  `json:"uptime"`
	Timestamp     string  `json:"timestamp"`

	CPUDetails    ResourceUsage `json:"cpu_details"`
	MemoryDetails ResourceUsage `json:"memory_details"`
	DiskDetails   ResourceUsage `json:"disk_details"`

	TopProcesses   []ProcessInfo            `json:"top_processes"`
	NetworkMetrics *NetworkMetrics          `json:"network_metrics,omitempty"`
	Services       []ServiceInfo            `json:"services,omitempty"`
}

// ResourceUsage for legacy API
type ResourceUsage struct {
	Total     int64   `json:"total"`
	Used      int64   `json:"used"`
	Free      int64   `json:"free"`
	Available int64   `json:"available"`
	Usage     float64 `json:"usage_percent"`
}

// GetMetrics returns metrics in legacy format for UI
func GetMetrics() (*Metrics, error) {
	all, err := CollectAllMetrics()
	if err != nil {
		return nil, err
	}

	m := &Metrics{
		Timestamp: time.Now().UTC().Format("2006-01-02 15:04:05"),
	}

	if all.Summary != nil {
		s := all.Summary
		m.CPUUsage = s.CPUUsage
		m.MemoryUsage = s.MemoryUsage
		m.DiskUsage = s.DiskUsage
		m.IOWait = s.CPUIOWait
		m.IOPS = int64(s.DiskIOPSRead + s.DiskIOPSWrite)

		// Calculate HTTPS connections
		if conns, err := net.Connections("tcp"); err == nil {
			for _, c := range conns {
				if c.Raddr.Port == 443 {
					m.HTTPSRequests++
				}
			}
		}

		m.CPUDetails = ResourceUsage{
			Total: int64(s.CPUCores),
			Usage: s.CPUUsage,
		}

		m.MemoryDetails = ResourceUsage{
			Total:     int64(s.MemoryTotal / 1024),
			Used:      int64(s.MemoryUsed / 1024),
			Free:      int64(s.MemoryFree / 1024),
			Available: int64(s.MemoryAvailable / 1024),
			Usage:     s.MemoryUsage,
		}

		m.DiskDetails = ResourceUsage{
			Total:     int64(s.DiskTotal / 1024),
			Used:      int64(s.DiskUsed / 1024),
			Free:      int64(s.DiskFree / 1024),
			Available: int64(s.DiskFree / 1024),
			Usage:     s.DiskUsage,
		}
	}

	// OS Name
	switch runtime.GOOS {
	case "linux":
		m.OSName = "Linux"
	case "darwin":
		m.OSName = "macOS"
	default:
		m.OSName = runtime.GOOS
	}

	// IP Address
	if interfaces, err := net.Interfaces(); err == nil {
		for _, iface := range interfaces {
			for _, addr := range iface.Addrs {
				if strings.Contains(addr.Addr, ".") && !strings.Contains(addr.Addr, "127.0.0.1") {
					m.IPAddress = strings.Split(addr.Addr, "/")[0]
					break
				}
			}
			if m.IPAddress != "" {
				break
			}
		}
	}
	if m.IPAddress == "" {
		m.IPAddress = "unknown"
	}

	// Uptime
	if uptime, err := host.Uptime(); err == nil {
		days := uptime / (24 * 3600)
		hours := (uptime % (24 * 3600)) / 3600
		minutes := (uptime % 3600) / 60
		if days > 0 {
			m.Uptime = fmt.Sprintf("%d days", days)
		} else if hours > 0 {
			m.Uptime = fmt.Sprintf("%d hours", hours)
		} else {
			m.Uptime = fmt.Sprintf("%d minutes", minutes)
		}
	} else {
		m.Uptime = "unknown"
	}

	m.TopProcesses = all.Processes
	m.Services = all.Services

	// Network metrics
	if len(all.Networks) > 0 {
		m.NetworkMetrics = convertToLegacyNetworkMetrics(all.Networks)
	}

	return m, nil
}

func convertToLegacyNetworkMetrics(networks []NetworkInterfaceMetrics) *NetworkMetrics {
	nm := &NetworkMetrics{
		Interfaces: make([]InterfaceInfo, 0, len(networks)),
	}

	for _, n := range networks {
		nm.TotalBytesIn += n.BytesRecv
		nm.TotalBytesOut += n.BytesSent
		nm.TotalPacketsIn += n.PacketsRecv
		nm.TotalPacketsOut += n.PacketsSent
		nm.TotalErrorsIn += int64(n.ErrorsIn)
		nm.TotalErrorsOut += int64(n.ErrorsOut)

		iface := InterfaceInfo{
			Name:       n.Interface,
			BytesIn:    n.BytesRecv,
			BytesOut:   n.BytesSent,
			PacketsIn:  n.PacketsRecv,
			PacketsOut: n.PacketsSent,
			ErrorsIn:   int64(n.ErrorsIn),
			ErrorsOut:  int64(n.ErrorsOut),
			IsUp:       n.IsUp,
			MTU:        int(n.MTU),
			Speed:      int64(n.SpeedMbps),
		}
		if len(n.IPAddresses) > 0 {
			iface.IPAddresses = n.IPAddresses
		}
		nm.Interfaces = append(nm.Interfaces, iface)
	}

	return nm
}

// =============================================================================
// Server Specs (for registration)
// =============================================================================

// GetServerSpecs returns server specifications for registration
// Memory and storage are returned in GB as float64 for precision
func GetServerSpecs() (map[string]interface{}, error) {
	specs := make(map[string]interface{})

	specs["cpu_cores"] = runtime.NumCPU()

	if vm, err := mem.VirtualMemory(); err == nil {
		// Store in GB as float64 to preserve precision for small VMs (<1GB)
		// This keeps backward compatibility with existing data format
		specs["total_memory"] = float64(vm.Total) / (1024 * 1024 * 1024)
	} else {
		specs["total_memory"] = 0.0
	}

	if usage, err := disk.Usage("/"); err == nil {
		// Store in GB as float64 for consistency
		specs["total_storage"] = float64(usage.Total) / (1024 * 1024 * 1024)
	} else {
		specs["total_storage"] = 0.0
	}

	return specs, nil
}
