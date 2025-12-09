// Package metrics provides OpenTelemetry-based system metrics collection for CatOps CLI.
//
// This package uses OpenTelemetry instrumentation to collect:
// - CPU utilization and load averages
// - Memory usage and availability
// - Disk I/O and filesystem usage
// - Network I/O and connections
// - Process information
// - Docker container metrics (when available)
package metrics

import (
	"context"
	"fmt"
	"os"
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

	// Still using gopsutil for local metrics display (UI)
	// OTel SDK sends metrics to backend, gopsutil provides immediate values for CLI UI
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

	constants "catops/config"
)

// =============================================================================
// OpenTelemetry Collector Integration
// =============================================================================

var (
	// OTel meter provider and meter
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter
	otelMu        sync.Mutex
	otelStarted   bool

	// Observable instruments for async metrics
	cpuGauge    metric.Float64ObservableGauge
	memGauge    metric.Float64ObservableGauge
	diskGauge   metric.Float64ObservableGauge
	iopsGauge   metric.Int64ObservableGauge
	ioWaitGauge metric.Float64ObservableGauge

	// Cached metric values for OTel callbacks
	cachedCPU    float64
	cachedMem    float64
	cachedDisk   float64
	cachedIOPS   int64
	cachedIOWait float64
	cacheMu      sync.RWMutex
)

// OTelConfig holds configuration for OpenTelemetry exporter
type OTelConfig struct {
	Endpoint           string
	AuthToken          string
	ServerID           string
	Hostname           string
	CollectionInterval time.Duration
}

// StartOTelCollector initializes and starts the OpenTelemetry metrics exporter
func StartOTelCollector(cfg *OTelConfig) error {
	otelMu.Lock()
	defer otelMu.Unlock()

	if otelStarted {
		return nil // Already started
	}

	if cfg.Endpoint == "" || cfg.AuthToken == "" || cfg.ServerID == "" {
		return fmt.Errorf("OTLP config incomplete: endpoint, auth_token, and server_id required")
	}

	ctx := context.Background()

	// Create OTLP HTTP exporter
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

	// Create resource with server attributes
	hostname := cfg.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("catops-cli"),
			semconv.ServiceVersion("2.0.0"),
			semconv.HostName(hostname),
			attribute.String("catops.server.id", cfg.ServerID),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create meter provider
	interval := cfg.CollectionInterval
	if interval == 0 {
		interval = 15 * time.Second
	}

	meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter,
				sdkmetric.WithInterval(interval),
			),
		),
	)

	// Set as global provider
	otel.SetMeterProvider(meterProvider)

	// Get meter for our metrics
	meter = meterProvider.Meter("catops.io/cli",
		metric.WithInstrumentationVersion("2.0.0"),
	)

	// Register observable gauges for system metrics
	if err := registerSystemMetrics(); err != nil {
		return fmt.Errorf("failed to register system metrics: %w", err)
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

// registerSystemMetrics registers all system metric instruments
func registerSystemMetrics() error {
	var err error

	// CPU utilization (system.cpu.utilization)
	cpuGauge, err = meter.Float64ObservableGauge(
		"system.cpu.utilization",
		metric.WithDescription("CPU utilization as a fraction (0-1)"),
		metric.WithUnit("1"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			val := cachedCPU / 100.0 // Convert percentage to fraction
			cacheMu.RUnlock()
			o.Observe(val, metric.WithAttributes(attribute.String("state", "used")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Memory utilization (system.memory.utilization)
	memGauge, err = meter.Float64ObservableGauge(
		"system.memory.utilization",
		metric.WithDescription("Memory utilization as a fraction (0-1)"),
		metric.WithUnit("1"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			val := cachedMem / 100.0
			cacheMu.RUnlock()
			o.Observe(val, metric.WithAttributes(attribute.String("state", "used")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Filesystem utilization (system.filesystem.utilization)
	diskGauge, err = meter.Float64ObservableGauge(
		"system.filesystem.utilization",
		metric.WithDescription("Filesystem utilization as a fraction (0-1)"),
		metric.WithUnit("1"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			val := cachedDisk / 100.0
			cacheMu.RUnlock()
			o.Observe(val, metric.WithAttributes(
				attribute.String("device", "/"),
				attribute.String("mountpoint", "/"),
			))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Disk IOPS (system.disk.operations)
	iopsGauge, err = meter.Int64ObservableGauge(
		"system.disk.operations",
		metric.WithDescription("Disk I/O operations per second"),
		metric.WithUnit("{operations}/s"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			cacheMu.RLock()
			val := cachedIOPS
			cacheMu.RUnlock()
			o.Observe(val)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// IO Wait (custom metric for CatOps)
	ioWaitGauge, err = meter.Float64ObservableGauge(
		"system.cpu.iowait",
		metric.WithDescription("CPU time waiting for I/O as a fraction"),
		metric.WithUnit("1"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			cacheMu.RLock()
			val := cachedIOWait / 100.0
			cacheMu.RUnlock()
			o.Observe(val)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	return nil
}

// UpdateCachedMetrics updates the cached metric values for OTel export
// Called from the main metrics collection loop
func UpdateCachedMetrics(m *Metrics) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	cachedCPU = m.CPUUsage
	cachedMem = m.MemoryUsage
	cachedDisk = m.DiskUsage
	cachedIOPS = m.IOPS
	cachedIOWait = m.IOWait
}

// =============================================================================
// Legacy IOPS Monitoring (kept for backward compatibility)
// =============================================================================

var (
	iopsOnce     sync.Once
	iopsStopChan chan struct{}
	iopsStopOnce sync.Once
	iopsMutex    sync.RWMutex
	localIOPS    int64
)

// StartIOPSMonitoring starts background goroutine to measure IOPS continuously
func StartIOPSMonitoring() {
	iopsOnce.Do(func() {
		iopsStopChan = make(chan struct{})

		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Silently recover
				}
			}()

			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					iops, err := measureIOPS()
					if err == nil {
						iopsMutex.Lock()
						localIOPS = iops
						iopsMutex.Unlock()
					}
				case <-iopsStopChan:
					return
				}
			}
		}()
	})
}

// StopIOPSMonitoring gracefully stops the IOPS monitoring goroutine
func StopIOPSMonitoring() {
	iopsStopOnce.Do(func() {
		if iopsStopChan != nil {
			close(iopsStopChan)
		}
	})
}

var prevIOCounters map[string]disk.IOCountersStat
var prevIOMutex sync.RWMutex

// measureIOPS calculates IOPS from delta between measurements
func measureIOPS() (int64, error) {
	currentIO, err := disk.IOCounters()
	if err != nil {
		return 0, fmt.Errorf("failed to get IO counters: %w", err)
	}

	prevIOMutex.Lock()
	defer prevIOMutex.Unlock()

	if prevIOCounters == nil {
		prevIOCounters = currentIO
		return 0, nil
	}

	io1 := prevIOCounters
	io2 := currentIO
	prevIOCounters = currentIO

	var totalIOPS int64
	for device, stats2 := range io2 {
		if stats1, ok := io1[device]; ok {
			if strings.HasPrefix(device, "loop") ||
				strings.Contains(device, "dm-") ||
				strings.Contains(device, "sr") ||
				len(device) > 20 {
				continue
			}

			readOps := stats2.ReadCount - stats1.ReadCount
			writeOps := stats2.WriteCount - stats1.WriteCount
			totalIOPS += int64(readOps + writeOps)
		}
	}

	return totalIOPS, nil
}

// =============================================================================
// Types (unchanged for backward compatibility)
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

// ServiceInfo contains information about a detected service
type ServiceInfo struct {
	PID         int         `json:"pid"`
	ServiceType ServiceType `json:"service_type"`
	ServiceName string      `json:"service_name"`
	Framework   string      `json:"framework"`
	Port        int         `json:"port"`
	Ports       []int       `json:"ports"`
	Version     string      `json:"version"`
	Command     string      `json:"command"`
	CPUUsage    float64     `json:"cpu_usage"`
	MemoryUsage float64     `json:"memory_usage"`
	MemoryKB    int64       `json:"memory_kb"`
	Status      string      `json:"status"`
	User        string      `json:"user"`
	StartTime   int64       `json:"start_time"`
	Threads     int         `json:"threads"`
	IsContainer bool        `json:"is_container"`
	ContainerID string      `json:"container_id"`
	RecentLogs  []string    `json:"recent_logs"`
	LogSource   string      `json:"log_source"`
}

// ProcessInfo contains detailed information about a running system process
type ProcessInfo struct {
	PID         int     `json:"pid"`
	Name        string  `json:"name"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	MemoryKB    int64   `json:"memory_kb"`
	Command     string  `json:"command"`
	User        string  `json:"user"`
	Status      string  `json:"status"`
	StartTime   int64   `json:"start_time"`
	Threads     int     `json:"threads"`
	VirtualMem  int64   `json:"virtual_mem"`
	ResidentMem int64   `json:"resident_mem"`
	TTY         string  `json:"tty"`
	CPU         int     `json:"cpu"`
	Priority    int     `json:"priority"`
	Nice        int     `json:"nice"`
}

// ResourceUsage represents detailed resource information
type ResourceUsage struct {
	Total     int64   `json:"total"`
	Used      int64   `json:"used"`
	Free      int64   `json:"free"`
	Available int64   `json:"available"`
	Usage     float64 `json:"usage_percent"`
}

// Metrics contains comprehensive system monitoring data
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

	TopProcesses   []ProcessInfo   `json:"top_processes"`
	NetworkMetrics *NetworkMetrics `json:"network_metrics,omitempty"`
	Services       []ServiceInfo   `json:"services,omitempty"`
}

// =============================================================================
// Metric Collection Functions (using gopsutil for local values)
// =============================================================================

// GetCPUUsage retrieves the current CPU usage percentage
func GetCPUUsage() (float64, error) {
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, fmt.Errorf("failed to get CPU usage: %w", err)
	}

	if len(percent) > 0 {
		return percent[0], nil
	}

	return 0, fmt.Errorf("no CPU usage data available")
}

// GetDiskUsage returns disk usage percentage
func GetDiskUsage() (float64, error) {
	usage, err := disk.Usage("/")
	if err != nil {
		return 0, fmt.Errorf("failed to get disk usage: %w", err)
	}

	return usage.UsedPercent, nil
}

// GetMemoryUsage returns memory usage percentage
func GetMemoryUsage() (float64, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, fmt.Errorf("failed to get memory usage: %w", err)
	}

	return vm.UsedPercent, nil
}

// GetHTTPSRequests returns number of HTTPS connections
func GetHTTPSRequests() (int64, error) {
	connections, err := net.Connections("tcp")
	if err != nil {
		return 0, fmt.Errorf("failed to get network connections: %w", err)
	}

	count := int64(0)
	for _, conn := range connections {
		if conn.Raddr.Port == 443 {
			count++
		}
	}

	return count, nil
}

// GetIOPS returns cached Input/Output Operations Per Second
func GetIOPS() (int64, error) {
	iopsMutex.RLock()
	iops := localIOPS
	iopsMutex.RUnlock()

	return iops, nil
}

// GetIOWait returns I/O Wait percentage
func GetIOWait() (float64, error) {
	if runtime.GOOS != "linux" {
		return 0.0, nil
	}

	times, err := cpu.Times(false)
	if err != nil {
		return 0, fmt.Errorf("failed to get CPU times: %w", err)
	}

	if len(times) == 0 {
		return 0, fmt.Errorf("no CPU time data available")
	}

	cpuTime := times[0]

	total := cpuTime.User +
		cpuTime.System +
		cpuTime.Idle +
		cpuTime.Iowait +
		cpuTime.Nice +
		cpuTime.Irq +
		cpuTime.Softirq +
		cpuTime.Steal

	if total == 0 {
		return 0, nil
	}

	ioWaitPercent := (cpuTime.Iowait / total) * 100

	return ioWaitPercent, nil
}

// GetOSName returns operating system name
func GetOSName() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "Linux", nil
	case "darwin":
		return "macOS", nil
	default:
		return runtime.GOOS, nil
	}
}

// GetIPAddress returns system IP address
func GetIPAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		for _, addr := range iface.Addrs {
			if strings.Contains(addr.Addr, ".") && !strings.Contains(addr.Addr, "127.0.0.1") {
				ip := strings.Split(addr.Addr, "/")[0]
				return ip, nil
			}
		}
	}

	return "unknown", nil
}

// GetUptime returns system uptime
func GetUptime() (string, error) {
	uptime, err := host.Uptime()
	if err != nil {
		return "unknown", fmt.Errorf("failed to get uptime: %w", err)
	}

	days := uptime / (24 * 3600)
	hours := (uptime % (24 * 3600)) / 3600
	minutes := (uptime % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%d days", days), nil
	} else if hours > 0 {
		return fmt.Sprintf("%d hours", hours), nil
	} else {
		return fmt.Sprintf("%d minutes", minutes), nil
	}
}

// GetMetrics returns all system metrics
func GetMetrics() (*Metrics, error) {
	cpuUsage, err := GetCPUUsage()
	if err != nil {
		return nil, fmt.Errorf("CPU error: %w", err)
	}

	diskUsage, err := GetDiskUsage()
	if err != nil {
		return nil, fmt.Errorf("disk error: %w", err)
	}

	memoryUsage, err := GetMemoryUsage()
	if err != nil {
		return nil, fmt.Errorf("memory error: %w", err)
	}

	httpsRequests, _ := GetHTTPSRequests()
	ioPS, _ := GetIOPS()
	ioWait, _ := GetIOWait()
	osName, _ := GetOSName()
	ipAddress, _ := GetIPAddress()
	uptime, _ := GetUptime()

	cpuDetails, _ := GetDetailedCPUUsage()
	memoryDetails, _ := GetDetailedMemoryUsage()
	diskDetails, _ := GetDetailedDiskUsage()

	topProcesses, _ := GetTopProcesses(30)
	networkMetrics, _ := GetNetworkMetrics()
	services, _ := GetServices()

	m := &Metrics{
		CPUUsage:      cpuUsage,
		DiskUsage:     diskUsage,
		MemoryUsage:   memoryUsage,
		HTTPSRequests: httpsRequests,
		IOPS:          ioPS,
		IOWait:        ioWait,
		OSName:        osName,
		IPAddress:     ipAddress,
		Uptime:        uptime,
		Timestamp:     time.Now().UTC().Format("2006-01-02 15:04:05"),

		CPUDetails:    cpuDetails,
		MemoryDetails: memoryDetails,
		DiskDetails:   diskDetails,

		TopProcesses:   topProcesses,
		NetworkMetrics: networkMetrics,
		Services:       services,
	}

	// Update cached values for OTel export
	UpdateCachedMetrics(m)

	return m, nil
}

// GetDetailedCPUUsage returns detailed CPU information
func GetDetailedCPUUsage() (ResourceUsage, error) {
	var usage ResourceUsage

	cores, err := cpu.Counts(false)
	if err != nil {
		return usage, fmt.Errorf("failed to get CPU cores: %w", err)
	}
	usage.Total = int64(cores)

	if cpuPercent, err := GetCPUUsage(); err == nil {
		usage.Usage = cpuPercent
		usage.Used = int64(cpuPercent * float64(usage.Total) / 100)
		usage.Free = usage.Total - usage.Used
		usage.Available = usage.Total
	}

	return usage, nil
}

// GetDetailedMemoryUsage returns detailed memory information
func GetDetailedMemoryUsage() (ResourceUsage, error) {
	var usage ResourceUsage

	vm, err := mem.VirtualMemory()
	if err != nil {
		return usage, fmt.Errorf("failed to get memory usage: %w", err)
	}

	usage.Total = int64(vm.Total / 1024)
	usage.Used = int64(vm.Used / 1024)
	usage.Free = int64(vm.Free / 1024)
	usage.Available = int64(vm.Available / 1024)
	usage.Usage = vm.UsedPercent

	return usage, nil
}

// GetDetailedDiskUsage returns detailed disk information
func GetDetailedDiskUsage() (ResourceUsage, error) {
	var usage ResourceUsage

	diskUsage, err := disk.Usage("/")
	if err != nil {
		return usage, fmt.Errorf("failed to get disk usage: %w", err)
	}

	usage.Total = int64(diskUsage.Total / 1024)
	usage.Used = int64(diskUsage.Used / 1024)
	usage.Available = int64(diskUsage.Free / 1024)
	usage.Free = int64(diskUsage.Free / 1024)
	usage.Usage = diskUsage.UsedPercent

	return usage, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetTopProcesses returns top processes by resource usage
func GetTopProcesses(limit int) ([]ProcessInfo, error) {
	var processes []ProcessInfo

	allProcesses, err := process.Processes()
	if err != nil {
		return processes, fmt.Errorf("failed to get processes: %w", err)
	}

	processes = make([]ProcessInfo, 0, min(len(allProcesses), limit*3))

	for _, proc := range allProcesses {
		name, _ := proc.Name()
		cpuPercent, _ := proc.CPUPercent()
		memoryPercent, _ := proc.MemoryPercent()
		memoryInfo, _ := proc.MemoryInfo()

		status, _ := proc.Status()
		createTime, _ := proc.CreateTime()
		numThreads, _ := proc.NumThreads()

		username := "unknown"
		if uids, err := proc.Uids(); err == nil && len(uids) > 0 {
			username = fmt.Sprintf("%d", uids[0])
		}

		terminal, _ := proc.Terminal()

		if cpuPercent < 0.1 && memoryPercent < 0.1 {
			continue
		}

		if name == "catops" || strings.HasPrefix(name, "catops-") {
			continue
		}

		command := name
		if cmdline, err := proc.Cmdline(); err == nil && cmdline != "" {
			command = cmdline
		}

		if len(command) > 200 {
			command = command[:197] + "..."
		}

		var memoryKB int64
		if memoryInfo != nil {
			memoryKB = int64(memoryInfo.RSS / 1024)
		}

		var virtualMem int64
		if memoryInfo != nil {
			virtualMem = int64(memoryInfo.VMS / 1024)
		}

		statusChar := "R"
		if len(status) > 0 {
			statusChar = string(status[0])
		}

		startTime := createTime / 1000
		threadCount := int(numThreads)

		p := ProcessInfo{
			PID:         int(proc.Pid),
			Name:        name,
			CPUUsage:    cpuPercent,
			MemoryUsage: float64(memoryPercent),
			MemoryKB:    memoryKB,
			Command:     command,
			User:        username,
			Status:      statusChar,
			StartTime:   startTime,
			Priority:    20,
			Nice:        0,
			VirtualMem:  virtualMem,
			ResidentMem: memoryKB,
			TTY:         terminal,
			CPU:         0,
			Threads:     threadCount,
		}

		processes = append(processes, p)
	}

	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPUUsage > processes[j].CPUUsage
	})

	if len(processes) > limit {
		return processes[:limit], nil
	}

	return processes, nil
}

// GetServerSpecs returns server specifications for registration
func GetServerSpecs() (map[string]interface{}, error) {
	specs := make(map[string]interface{})

	cpuCores, err := GetCPUCores()
	if err != nil {
		specs["cpu_cores"] = 0
	} else {
		specs["cpu_cores"] = cpuCores
	}

	totalMemory, err := GetTotalMemory()
	if err != nil {
		specs["total_memory"] = 0
	} else {
		specs["total_memory"] = totalMemory
	}

	totalStorage, err := GetTotalStorage()
	if err != nil {
		specs["total_storage"] = 0
	} else {
		specs["total_storage"] = totalStorage
	}

	return specs, nil
}

// GetCPUCores returns the number of CPU cores
func GetCPUCores() (int64, error) {
	return int64(runtime.NumCPU()), nil
}

// GetTotalMemory returns total memory in GB
func GetTotalMemory() (int64, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, fmt.Errorf("failed to get total memory: %w", err)
	}

	return int64(vm.Total / (1024 * 1024 * 1024)), nil
}

// GetTotalStorage returns total storage in GB
func GetTotalStorage() (int64, error) {
	usage, err := disk.Usage("/")
	if err != nil {
		return 0, fmt.Errorf("failed to get total storage: %w", err)
	}

	return int64(usage.Total / (1024 * 1024 * 1024)), nil
}
