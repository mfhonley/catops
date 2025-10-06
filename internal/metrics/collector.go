package metrics

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// ProcessInfo contains detailed information about a running system process
type ProcessInfo struct {
	PID         int     `json:"pid"`
	Name        string  `json:"name"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	MemoryKB    int64   `json:"memory_kb"`
	Command     string  `json:"command"`
	User        string  `json:"user"`

	// Extended process details for comprehensive monitoring
	Status      string `json:"status"`       // Process state: R (running), S (sleeping), Z (zombie), D (disk sleep)
	StartTime   int64  `json:"start_time"`   // Process start time as Unix timestamp
	Threads     int    `json:"threads"`      // Number of threads used by the process
	VirtualMem  int64  `json:"virtual_mem"`  // Virtual memory size (VSZ) in KB
	ResidentMem int64  `json:"resident_mem"` // Resident memory size (RSS) in KB
	TTY         string `json:"tty"`          // Terminal device (pts/0, ?, etc.)
	CPU         int    `json:"cpu"`          // CPU number this process is running on
	Priority    int    `json:"priority"`     // Process priority (lower = higher priority)
	Nice        int    `json:"nice"`         // Process nice value (affects scheduling)
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

	// I/O performance metrics for storage monitoring
	IOPS   int64   `json:"iops"`    // Input/Output Operations Per Second
	IOWait float64 `json:"io_wait"` // I/O Wait percentage (indicates storage bottlenecks)

	OSName    string `json:"os_name"`
	IPAddress string `json:"ip_address"`
	Uptime    string `json:"uptime"`
	Timestamp string `json:"timestamp"`

	// Detailed resource breakdown for granular monitoring
	CPUDetails    ResourceUsage `json:"cpu_details"`    // CPU cores and usage breakdown
	MemoryDetails ResourceUsage `json:"memory_details"` // Memory allocation and availability
	DiskDetails   ResourceUsage `json:"disk_details"`   // Disk space and usage details

	// Process monitoring and analysis
	TopProcesses []ProcessInfo `json:"top_processes"` // Top processes by resource consumption
}

// GetCPUUsage retrieves the current CPU usage percentage across all cores
func GetCPUUsage() (float64, error) {
	// Use native gopsutil instead of exec.Command for better performance
	percent, err := cpu.Percent(0, false)
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
	// Use native gopsutil instead of exec.Command for better performance
	usage, err := disk.Usage("/")
	if err != nil {
		return 0, fmt.Errorf("failed to get disk usage: %w", err)
	}

	return usage.UsedPercent, nil
}

// GetMemoryUsage returns memory usage percentage
func GetMemoryUsage() (float64, error) {
	// Use native gopsutil instead of exec.Command for better performance
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, fmt.Errorf("failed to get memory usage: %w", err)
	}

	return vm.UsedPercent, nil
}

// GetHTTPSRequests returns number of HTTPS connections
func GetHTTPSRequests() (int64, error) {
	// Use native gopsutil instead of exec.Command for better performance
	connections, err := net.Connections("tcp")
	if err != nil {
		return 0, fmt.Errorf("failed to get network connections: %w", err)
	}

	count := int64(0)
	for _, conn := range connections {
		// Check if connection is to port 443 (HTTPS)
		if conn.Raddr.Port == 443 {
			count++
		}
	}

	return count, nil
}

// GetIOPS returns Input/Output Operations Per Second
func GetIOPS() (int64, error) {
	// Get disk I/O counters (first snapshot)
	io1, err := disk.IOCounters()
	if err != nil {
		return 0, fmt.Errorf("failed to get IO counters: %w", err)
	}

	// Wait 1 second to measure IOPS
	time.Sleep(1 * time.Second)

	// Get disk I/O counters (second snapshot)
	io2, err := disk.IOCounters()
	if err != nil {
		return 0, fmt.Errorf("failed to get IO counters: %w", err)
	}

	// Calculate IOPS = (read operations + write operations) per second
	var totalIOPS int64
	for device, stats2 := range io2 {
		if stats1, ok := io1[device]; ok {
			// Skip loop devices, partitions, and virtual devices
			if strings.HasPrefix(device, "loop") ||
				strings.Contains(device, "dm-") ||
				strings.Contains(device, "sr") ||
				len(device) > 20 { // Skip very long device names (usually virtual)
				continue
			}

			// Calculate read and write operations difference
			readOps := stats2.ReadCount - stats1.ReadCount
			writeOps := stats2.WriteCount - stats1.WriteCount
			totalIOPS += int64(readOps + writeOps)
		}
	}

	return totalIOPS, nil
}

// GetIOWait returns I/O Wait percentage
func GetIOWait() (float64, error) {
	// IOWait is only available on Linux
	if runtime.GOOS != "linux" {
		return 0.0, nil // macOS/Windows don't have IOWait metric
	}

	// Get CPU times (includes IOWait on Linux)
	times, err := cpu.Times(false)
	if err != nil {
		return 0, fmt.Errorf("failed to get CPU times: %w", err)
	}

	if len(times) == 0 {
		return 0, fmt.Errorf("no CPU time data available")
	}

	cpuTime := times[0]

	// Calculate total CPU time
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

	// IOWait percentage = (IOWait time / Total time) * 100
	ioWaitPercent := (cpuTime.Iowait / total) * 100

	return ioWaitPercent, nil
}

// GetOSName returns operating system name
func GetOSName() (string, error) {
	// Use native runtime instead of exec.Command for better performance
	switch runtime.GOOS {
	case "linux":
		return "Linux", nil
	case "darwin":
		return "macOS", nil
	case "windows":
		return "Windows", nil
	default:
		return runtime.GOOS, nil
	}
}

// GetIPAddress returns system IP address
func GetIPAddress() (string, error) {
	// Use native gopsutil instead of exec.Command for better performance
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		for _, addr := range iface.Addrs {
			// Look for IPv4 addresses (contains dots)
			if strings.Contains(addr.Addr, ".") && !strings.Contains(addr.Addr, "127.0.0.1") {
				// Extract IP from CIDR notation (e.g., "192.168.1.1/24" -> "192.168.1.1")
				ip := strings.Split(addr.Addr, "/")[0]
				return ip, nil
			}
		}
	}

	return "unknown", nil
}

// GetUptime returns system uptime
func GetUptime() (string, error) {
	// Use native gopsutil instead of exec.Command for better performance
	uptime, err := host.Uptime()
	if err != nil {
		return "unknown", fmt.Errorf("failed to get uptime: %w", err)
	}

	// Convert seconds to human readable format
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

	httpsRequests, err := GetHTTPSRequests()
	if err != nil {
		return nil, fmt.Errorf("HTTPS requests error: %w", err)
	}

	// Get IOPS (non-critical - don't fail if error occurs)
	ioPS, err := GetIOPS()
	if err != nil {
		// Log error but continue with 0 value
		ioPS = 0
	}

	// Get IOWait (non-critical - don't fail if error occurs)
	ioWait, err := GetIOWait()
	if err != nil {
		// Log error but continue with 0 value
		ioWait = 0
	}

	osName, err := GetOSName()
	if err != nil {
		return nil, fmt.Errorf("OS name error: %w", err)
	}

	ipAddress, err := GetIPAddress()
	if err != nil {
		return nil, fmt.Errorf("IP address error: %w", err)
	}

	uptime, err := GetUptime()
	if err != nil {
		return nil, fmt.Errorf("uptime error: %w", err)
	}

	// Get detailed resource information
	cpuDetails, _ := GetDetailedCPUUsage()
	memoryDetails, _ := GetDetailedMemoryUsage()
	diskDetails, _ := GetDetailedDiskUsage()

	// Get top processes
	topProcesses, _ := GetTopProcesses(10)

	return &Metrics{
		CPUUsage:      cpuUsage,
		DiskUsage:     diskUsage,
		MemoryUsage:   memoryUsage,
		HTTPSRequests: httpsRequests,
		IOPS:          ioPS,
		IOWait:        ioWait,
		OSName:        osName,
		IPAddress:     ipAddress,
		Uptime:        uptime,
		Timestamp:     time.Now().Format("2006-01-02 15:04:05"),

		// New detailed resource fields
		CPUDetails:    cpuDetails,
		MemoryDetails: memoryDetails,
		DiskDetails:   diskDetails,

		// Process monitoring
		TopProcesses: topProcesses,
	}, nil
}

// GetDetailedCPUUsage returns detailed CPU information
func GetDetailedCPUUsage() (ResourceUsage, error) {
	var usage ResourceUsage

	// Use native gopsutil instead of exec.Command for better performance
	cores, err := cpu.Counts(false)
	if err != nil {
		return usage, fmt.Errorf("failed to get CPU cores: %w", err)
	}
	usage.Total = int64(cores)

	// Get CPU usage percentage
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

	// Use native gopsutil instead of exec.Command for better performance
	vm, err := mem.VirtualMemory()
	if err != nil {
		return usage, fmt.Errorf("failed to get memory usage: %w", err)
	}

	usage.Total = int64(vm.Total / 1024) // Convert to KB
	usage.Used = int64(vm.Used / 1024)
	usage.Free = int64(vm.Free / 1024)
	usage.Available = int64(vm.Available / 1024)
	usage.Usage = vm.UsedPercent

	return usage, nil
}

// GetDetailedDiskUsage returns detailed disk information
func GetDetailedDiskUsage() (ResourceUsage, error) {
	var usage ResourceUsage

	// Use native gopsutil instead of exec.Command for better performance
	diskUsage, err := disk.Usage("/")
	if err != nil {
		return usage, fmt.Errorf("failed to get disk usage: %w", err)
	}

	usage.Total = int64(diskUsage.Total / 1024) // Convert to KB
	usage.Used = int64(diskUsage.Used / 1024)
	usage.Available = int64(diskUsage.Free / 1024)
	usage.Free = int64(diskUsage.Free / 1024)
	usage.Usage = diskUsage.UsedPercent

	return usage, nil
}

// GetTopProcesses returns top processes by resource usage
func GetTopProcesses(limit int) ([]ProcessInfo, error) {
	var processes []ProcessInfo

	// Use native gopsutil instead of exec.Command for better performance
	allProcesses, err := process.Processes()
	if err != nil {
		return processes, fmt.Errorf("failed to get processes: %w", err)
	}

	// Get total CPU cores for normalization
	totalCores, err := cpu.Counts(false)
	if err != nil {
		totalCores = 1 // fallback
	}

	// Collect process information
	for _, proc := range allProcesses {
		// Get basic process info
		name, _ := proc.Name()
		cpuPercent, _ := proc.CPUPercent()
		memoryPercent, _ := proc.MemoryPercent()
		memoryInfo, _ := proc.MemoryInfo()

		// Get process status
		status, _ := proc.Status()
		createTime, _ := proc.CreateTime()
		numThreads, _ := proc.NumThreads()

		// Get user info (simplified - use PID as fallback)
		username := "unknown"
		if uids, err := proc.Uids(); err == nil && len(uids) > 0 {
			username = fmt.Sprintf("%d", uids[0]) // Use UID as string
		}

		// Get terminal info
		terminal, _ := proc.Terminal()

		// Normalize CPU percentage to total system capacity
		normalizedCPU := cpuPercent / float64(totalCores)

		// Get command (simplified)
		command := name
		if len(command) > 50 {
			command = command[:47] + "..."
		}

		// Get memory in KB
		var memoryKB int64
		if memoryInfo != nil {
			memoryKB = int64(memoryInfo.RSS / 1024) // Convert bytes to KB
		}

		// Get virtual memory in KB
		var virtualMem int64
		if memoryInfo != nil {
			virtualMem = int64(memoryInfo.VMS / 1024) // Convert bytes to KB
		}

		// Get process status character
		statusChar := "R" // default to running
		if len(status) > 0 {
			statusChar = string(status[0])
		}

		// Get start time (convert from milliseconds to seconds)
		startTime := createTime / 1000

		// Get thread count
		threadCount := int(numThreads)

		// Priority and Nice (simplified - use defaults)
		priority := 20
		nice := 0

		// Get CPU number (simplified - use 0)
		cpuNum := 0

		process := ProcessInfo{
			PID:         int(proc.Pid),
			Name:        name,
			CPUUsage:    normalizedCPU,
			MemoryUsage: float64(memoryPercent),
			MemoryKB:    memoryKB,
			Command:     command,
			User:        username,

			// Extended fields
			Status:      statusChar,
			StartTime:   startTime,
			Priority:    priority,
			Nice:        nice,
			VirtualMem:  virtualMem,
			ResidentMem: memoryKB,
			TTY:         terminal,
			CPU:         cpuNum,
			Threads:     threadCount,
		}

		processes = append(processes, process)
	}

	// Sort by CPU usage and return top processes
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPUUsage > processes[j].CPUUsage
	})

	// Return only the requested limit
	if len(processes) > limit {
		return processes[:limit], nil
	}

	return processes, nil
}

// GetServerSpecs returns server specifications for registration
func GetServerSpecs() (map[string]interface{}, error) {
	specs := make(map[string]interface{})

	// Get CPU cores
	cpuCores, err := GetCPUCores()
	if err != nil {
		specs["cpu_cores"] = 0
	} else {
		specs["cpu_cores"] = cpuCores
	}

	// Get total memory in GB
	totalMemory, err := GetTotalMemory()
	if err != nil {
		specs["total_memory"] = 0
	} else {
		specs["total_memory"] = totalMemory
	}

	// Get total storage in GB
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
	// Use native runtime instead of exec.Command for better performance
	return int64(runtime.NumCPU()), nil
}

// GetTotalMemory returns total memory in GB
func GetTotalMemory() (int64, error) {
	// Use native gopsutil instead of exec.Command for better performance
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, fmt.Errorf("failed to get total memory: %w", err)
	}

	// Convert bytes to GB
	return int64(vm.Total / (1024 * 1024 * 1024)), nil
}

// GetTotalStorage returns total storage in GB
func GetTotalStorage() (int64, error) {
	// Use native gopsutil instead of exec.Command for better performance
	usage, err := disk.Usage("/")
	if err != nil {
		return 0, fmt.Errorf("failed to get total storage: %w", err)
	}

	// Convert bytes to GB
	return int64(usage.Total / (1024 * 1024 * 1024)), nil
}
