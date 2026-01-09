// Package metrics provides system metrics collection for CatOps CLI.
package metrics

import "time"

// =============================================================================
// Service Types
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

// =============================================================================
// CPU Metrics
// =============================================================================

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

// =============================================================================
// Memory Metrics
// =============================================================================

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

// =============================================================================
// Disk Metrics
// =============================================================================

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

// =============================================================================
// Network Metrics
// =============================================================================

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

// =============================================================================
// Process Metrics
// =============================================================================

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

// =============================================================================
// Service Metrics
// =============================================================================

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

// =============================================================================
// Container Metrics
// =============================================================================

// ContainerMetrics contains Docker/Podman container metrics
type ContainerMetrics struct {
	ContainerID      string   `json:"container_id"`
	ContainerName    string   `json:"container_name"`
	ImageName        string   `json:"image_name"`
	ImageTag         string   `json:"image_tag"`
	Runtime          string   `json:"runtime"`
	Status           string   `json:"status"`
	Health           string   `json:"health"`
	StartedAt        int64    `json:"started_at"`
	ExitCode         *int16   `json:"exit_code"`
	CPUPercent       float64  `json:"cpu_percent"`
	CPUSystemPercent float64  `json:"cpu_system_percent"`
	MemoryUsage      uint64   `json:"memory_usage"`
	MemoryLimit      uint64   `json:"memory_limit"`
	MemoryPercent    float64  `json:"memory_percent"`
	NetRxBytes       uint64   `json:"net_rx_bytes"`
	NetTxBytes       uint64   `json:"net_tx_bytes"`
	BlockReadBytes   uint64   `json:"block_read_bytes"`
	BlockWriteBytes  uint64   `json:"block_write_bytes"`
	PIDsCurrent      uint32   `json:"pids_current"`
	PIDsLimit        uint32   `json:"pids_limit"`
	Ports            string   `json:"ports"`
	Labels           string   `json:"labels"`
	RecentLogs       []string `json:"recent_logs"` // Container logs (errors/warnings)
}

// =============================================================================
// System Summary
// =============================================================================

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

// =============================================================================
// Aggregated Metrics
// =============================================================================

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
// Configuration
// =============================================================================

// OTelConfig holds configuration for OpenTelemetry exporter
type OTelConfig struct {
	Endpoint           string
	AuthToken          string
	ServerID           string
	Hostname           string
	CollectionInterval time.Duration
}

// Note: Legacy types (Metrics, ResourceUsage, NetworkMetrics, InterfaceInfo)
// are defined in collector.go and network.go respectively
