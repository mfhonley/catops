package metrics

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ProcessInfo represents information about a running process
type ProcessInfo struct {
	PID         int     `json:"pid"`
	Name        string  `json:"name"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	MemoryKB    int64   `json:"memory_kb"`
	Command     string  `json:"command"`
	User        string  `json:"user"`

	// New fields for backend analytics
	Status      string `json:"status"`       // R (running), S (sleeping), Z (zombie), D (disk sleep)
	StartTime   int64  `json:"start_time"`   // Unix timestamp when process started
	Threads     int    `json:"threads"`      // Number of threads
	VirtualMem  int64  `json:"virtual_mem"`  // Virtual memory size (VSZ) in KB
	ResidentMem int64  `json:"resident_mem"` // Resident memory size (RSS) in KB
	TTY         string `json:"tty"`          // Terminal (pts/0, ?, etc.)
	CPU         int    `json:"cpu"`          // CPU number (0, 1, 2, etc.)
	Priority    int    `json:"priority"`     // Process priority
	Nice        int    `json:"nice"`         // Nice value
}

// ResourceUsage represents detailed resource information
type ResourceUsage struct {
	Total     int64   `json:"total"`
	Used      int64   `json:"used"`
	Free      int64   `json:"free"`
	Available int64   `json:"available"`
	Usage     float64 `json:"usage_percent"`
}

// Metrics represents system metrics
type Metrics struct {
	CPUUsage      float64 `json:"cpu_usage"`
	DiskUsage     float64 `json:"disk_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	HTTPSRequests int64   `json:"https_requests"`

	// New I/O metrics (exactly like HTTPS connections)
	IOPS   int64   `json:"iops"`    // Input/Output Operations Per Second
	IOWait float64 `json:"io_wait"` // I/O Wait percentage

	OSName    string `json:"os_name"`
	IPAddress string `json:"ip_address"`
	Uptime    string `json:"uptime"`
	Timestamp string `json:"timestamp"`

	// New detailed resource fields
	CPUDetails    ResourceUsage `json:"cpu_details"`
	MemoryDetails ResourceUsage `json:"memory_details"`
	DiskDetails   ResourceUsage `json:"disk_details"`

	// Process monitoring
	TopProcesses []ProcessInfo `json:"top_processes"`
}

// GetCPUUsage returns CPU usage percentage
func GetCPUUsage() (float64, error) {
	switch runtime.GOOS {
	case "linux":
		// Method 1: Try /proc/stat
		cmd := exec.Command("cat", "/proc/stat")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "cpu ") {
					parts := strings.Fields(line)
					if len(parts) >= 5 {
						user, _ := strconv.ParseFloat(parts[1], 64)
						nice, _ := strconv.ParseFloat(parts[2], 64)
						system, _ := strconv.ParseFloat(parts[3], 64)
						idle, _ := strconv.ParseFloat(parts[4], 64)

						total := user + nice + system + idle
						if total > 0 {
							usage := ((user + nice + system) / total) * 100
							return usage, nil
						}
					}
				}
			}
		}

		// Method 2: Try top command
		cmd = exec.Command("top", "-bn1")
		output, err = cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Cpu(s):") || strings.Contains(line, "%Cpu(s):") {
					parts := strings.Fields(line)
					for _, part := range parts {
						if strings.HasSuffix(part, "%id") {
							idleStr := strings.TrimSuffix(part, "%id")
							idle, err := strconv.ParseFloat(idleStr, 64)
							if err == nil {
								return 100 - idle, nil
							}
						}
					}
				}
			}
		}

		// Method 3: Try vmstat
		cmd = exec.Command("vmstat", "1", "2")
		output, err = cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 3 {
				parts := strings.Fields(lines[len(lines)-2]) // Get the second line (first is header)
				if len(parts) >= 15 {
					idle, _ := strconv.ParseFloat(parts[14], 64)
					return 100 - idle, nil
				}
			}
		}
	case "darwin":
		cmd := exec.Command("top", "-l", "1")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "CPU usage:") {
					parts := strings.Fields(line)
					for i, part := range parts {
						if strings.HasSuffix(part, "%") && i > 0 {
							if i+1 < len(parts) && strings.Contains(parts[i+1], "idle") {
								idleStr := strings.TrimSuffix(part, "%")
								idle, err := strconv.ParseFloat(idleStr, 64)
								if err == nil {
									return 100 - idle, nil
								}
							}
						}
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("could not parse CPU usage")
}

// GetDiskUsage returns disk usage percentage
func GetDiskUsage() (float64, error) {
	cmd := exec.Command("df", "-k", "/")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("could not parse disk usage")
	}

	parts := strings.Fields(lines[1])
	if len(parts) < 4 {
		return 0, fmt.Errorf("could not parse disk usage")
	}

	total, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}

	used, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, err
	}

	if total > 0 {
		return (used / total) * 100, nil
	}

	return 0, nil
}

// GetMemoryUsage returns memory usage percentage
func GetMemoryUsage() (float64, error) {
	switch runtime.GOOS {
	case "linux":
		// Linux method
		cmd := exec.Command("free")
		output, err := cmd.Output()
		if err != nil {
			return 0, err
		}

		lines := strings.Split(string(output), "\n")
		if len(lines) < 2 {
			return 0, fmt.Errorf("could not parse memory usage")
		}

		parts := strings.Fields(lines[1])
		if len(parts) < 3 {
			return 0, fmt.Errorf("could not parse memory usage")
		}

		total, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}

		used, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0, err
		}

		return (used / total) * 100, nil
	case "darwin":
		// macOS method
		cmd := exec.Command("vm_stat")
		output, err := cmd.Output()
		if err != nil {
			return 0, err
		}

		var total, used float64
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Pages free:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					free, _ := strconv.ParseFloat(parts[2], 64)
					total += free
				}
			} else if strings.Contains(line, "Pages active:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					active, _ := strconv.ParseFloat(parts[2], 64)
					used += active
					total += active
				}
			} else if strings.Contains(line, "Pages inactive:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					inactive, _ := strconv.ParseFloat(parts[2], 64)
					total += inactive
				}
			} else if strings.Contains(line, "Pages wired down:") {
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					wired, _ := strconv.ParseFloat(parts[3], 64)
					used += wired
					total += wired
				}
			}
		}

		if total > 0 {
			return (used / total) * 100, nil
		}
	}

	return 0, fmt.Errorf("could not parse memory usage")
}

// GetHTTPSRequests returns number of HTTPS connections
func GetHTTPSRequests() (int64, error) {
	// Count connections to port 443
	cmd := exec.Command("netstat", "-an")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative method
		cmd = exec.Command("ss", "-tuln")
		output, err = cmd.Output()
		if err != nil {
			return 0, fmt.Errorf("could not get HTTPS requests")
		}
	}

	lines := strings.Split(string(output), "\n")
	count := int64(0)
	for _, line := range lines {
		if strings.Contains(line, ":443") {
			count++
		}
	}

	return count, nil
}

// GetIOPS returns Input/Output Operations Per Second
func GetIOPS() (int64, error) {
	var iops int64

	switch runtime.GOOS {
	case "linux":
		// Method 1: /proc/diskstats
		cmd := exec.Command("cat", "/proc/diskstats")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 14 {
					// Чтения (колонка 4) + записи (колонка 8)
					reads, _ := strconv.ParseInt(parts[3], 10, 64)
					writes, _ := strconv.ParseInt(parts[7], 10, 64)
					iops += reads + writes
				}
			}
		}

		// Method 2: iostat если доступен
		cmd = exec.Command("iostat", "-x", "1", "2")
		output, err = cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "r/s") || strings.Contains(line, "w/s") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						// Парсим r/s и w/s
						for _, part := range parts {
							if strings.HasSuffix(part, "/s") {
								if value, err := strconv.ParseFloat(strings.TrimSuffix(part, "/s"), 64); err == nil {
									iops += int64(value)
								}
							}
						}
					}
				}
			}
		}

	case "darwin":
		// macOS: используем iostat
		cmd := exec.Command("iostat", "1", "2")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "disk0") || strings.Contains(line, "disk1") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						// transfers (колонка 2) - это примерно IOPS
						transfers, _ := strconv.ParseFloat(parts[1], 64)
						iops += int64(transfers)
					}
				}
			}
		}
	}

	return iops, nil
}

// GetIOWait returns I/O Wait percentage
func GetIOWait() (float64, error) {
	var ioWait float64

	switch runtime.GOOS {
	case "linux":
		// Метод 1: /proc/stat
		cmd := exec.Command("cat", "/proc/stat")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "cpu ") {
					parts := strings.Fields(line)
					if len(parts) >= 5 {
						// iowait находится в колонке 5
						iowait, _ := strconv.ParseFloat(parts[4], 64)
						idle, _ := strconv.ParseFloat(parts[3], 64)
						total := iowait + idle
						if total > 0 {
							ioWait = (iowait / total) * 100
						}
					}
				}
			}
		}

		// Метод 2: iostat
		cmd = exec.Command("iostat", "-x", "1", "2")
		output, err = cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "%util") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						// Последняя колонка - %util (I/O utilization)
						util, _ := strconv.ParseFloat(parts[len(parts)-1], 64)
						ioWait = util
					}
				}
			}
		}

		// Метод 3: vmstat
		cmd = exec.Command("vmstat", "1", "2")
		output, err = cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 3 {
				parts := strings.Fields(lines[len(lines)-2])
				if len(parts) >= 16 {
					// Колонка 16 - wa (wait)
					wa, _ := strconv.ParseFloat(parts[15], 64)
					ioWait = wa
				}
			}
		}

	case "darwin":
		// macOS: используем vm_stat и iostat
		cmd := exec.Command("vm_stat")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Pageins:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						pageins, _ := strconv.ParseFloat(parts[1], 64)
						// Примерный расчет I/O wait на основе pageins
						ioWait = pageins / 1000 // Нормализуем
					}
				}
			}
		}

		// Альтернативный метод через iostat
		cmd = exec.Command("iostat", "1", "2")
		output, err = cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "disk0") {
					parts := strings.Fields(line)
					if len(parts) >= 4 {
						// Примерный расчет на основе времени ожидания
						util, _ := strconv.ParseFloat(parts[3], 64)
						ioWait = util
					}
				}
			}
		}
	}

	return ioWait, nil
}

// GetOSName returns operating system name
func GetOSName() (string, error) {
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("cat", "/etc/os-release")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					name := strings.TrimPrefix(line, "PRETTY_NAME=")
					name = strings.Trim(name, "\"")
					return name, nil
				}
			}
		}

		cmd = exec.Command("uname", "-a")
		output, err = cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	case "darwin":
		cmd := exec.Command("sw_vers", "-productName")
		output, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	}

	return runtime.GOOS, nil
}

// GetIPAddress returns system IP address
func GetIPAddress() (string, error) {
	// Try to get local IP first (IPv4 preferred)
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("hostname", "-I")
		output, err := cmd.Output()
		if err == nil {
			ips := strings.Fields(string(output))
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0]), nil
			}
		}
	case "darwin":
		cmd := exec.Command("ifconfig")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				// Look for IPv4 addresses (contains dots)
				if strings.Contains(line, "inet ") && !strings.Contains(line, "127.0.0.1") && !strings.Contains(line, "::") {
					parts := strings.Fields(line)
					for i, part := range parts {
						if i > 0 && strings.Contains(part, ".") && !strings.Contains(part, ":") {
							return strings.TrimSpace(part), nil
						}
					}
				}
			}
		}
	}

	// Fallback to external IP
	cmd := exec.Command("curl", "-s", "ifconfig.me")
	output, err := cmd.Output()
	if err == nil {
		ip := strings.TrimSpace(string(output))
		if ip != "" && !strings.Contains(ip, "error") {
			return ip, nil
		}
	}

	return "unknown", nil
}

// GetUptime returns system uptime
func GetUptime() (string, error) {
	switch runtime.GOOS {
	case "linux":
		// Linux method
		cmd := exec.Command("uptime", "-p")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		uptime := strings.TrimSpace(string(output))
		// Remove "up " prefix
		uptime = strings.TrimPrefix(uptime, "up ")
		return uptime, nil
	case "darwin":
		// macOS method
		cmd := exec.Command("uptime")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}

		// Parse: "up 2 days, 3:45, 2 users, load averages: 1.23 1.45 1.67"
		uptimeStr := string(output)
		if strings.Contains(uptimeStr, "up ") {
			parts := strings.Split(uptimeStr, "up ")[1]
			parts = strings.Split(parts, ",")[0]
			return parts, nil
		}
		return "unknown", nil
	}

	return "unknown", fmt.Errorf("unsupported OS for uptime")
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

	ioPS, err := GetIOPS()
	if err != nil {
		return nil, fmt.Errorf("IOPS error: %w", err)
	}

	ioWait, err := GetIOWait()
	if err != nil {
		return nil, fmt.Errorf("IOWait error: %w", err)
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

	switch runtime.GOOS {
	case "linux":
		// Get CPU info from /proc/cpuinfo
		cmd := exec.Command("nproc")
		output, err := cmd.Output()
		if err == nil {
			if cores, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64); err == nil {
				usage.Total = cores
			}
		}

		// Get CPU usage percentage
		if cpuPercent, err := GetCPUUsage(); err == nil {
			usage.Usage = cpuPercent
			usage.Used = int64(cpuPercent * float64(usage.Total) / 100)
			usage.Free = usage.Total - usage.Used
			usage.Available = usage.Total
		}

	case "darwin":
		// Get CPU cores from sysctl
		cmd := exec.Command("sysctl", "-n", "hw.ncpu")
		output, err := cmd.Output()
		if err == nil {
			if cores, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64); err == nil {
				usage.Total = cores
			}
		}

		// Get CPU usage percentage
		if cpuPercent, err := GetCPUUsage(); err == nil {
			usage.Usage = cpuPercent
			usage.Used = int64(cpuPercent * float64(usage.Total) / 100)
			usage.Free = usage.Total - usage.Used
			usage.Available = usage.Total
		}
	}

	return usage, nil
}

// GetDetailedMemoryUsage returns detailed memory information
func GetDetailedMemoryUsage() (ResourceUsage, error) {
	var usage ResourceUsage

	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("free", "-k")
		output, err := cmd.Output()
		if err != nil {
			return usage, err
		}

		lines := strings.Split(string(output), "\n")
		if len(lines) < 2 {
			return usage, fmt.Errorf("could not parse memory usage")
		}

		parts := strings.Fields(lines[1])
		if len(parts) < 4 {
			return usage, fmt.Errorf("could not parse memory usage")
		}

		total, _ := strconv.ParseInt(parts[1], 10, 64)
		used, _ := strconv.ParseInt(parts[2], 10, 64)
		free, _ := strconv.ParseInt(parts[3], 10, 64)
		available, _ := strconv.ParseInt(parts[6], 10, 64)

		usage.Total = total
		usage.Used = used
		usage.Free = free
		usage.Available = available
		usage.Usage = float64(used) / float64(total) * 100

	case "darwin":
		cmd := exec.Command("vm_stat")
		output, err := cmd.Output()
		if err != nil {
			return usage, err
		}

		var total, used, free float64
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Pages free:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					if val, err := strconv.ParseFloat(parts[2], 64); err == nil {
						free += val * 4096 // Convert pages to bytes
					}
				}
			} else if strings.Contains(line, "Pages active:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					if val, err := strconv.ParseFloat(parts[2], 64); err == nil {
						used += val * 4096
						total += val * 4096
					}
				}
			} else if strings.Contains(line, "Pages inactive:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					if val, err := strconv.ParseFloat(parts[2], 64); err == nil {
						total += val * 4096
					}
				}
			} else if strings.Contains(line, "Pages wired down:") {
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					if val, err := strconv.ParseFloat(parts[3], 64); err == nil {
						used += val * 4096
						total += val * 4096
					}
				}
			}
		}

		usage.Total = int64(total / 1024) // Convert to KB
		usage.Used = int64(used / 1024)
		usage.Free = int64(free / 1024)
		usage.Available = usage.Total
		if usage.Total > 0 {
			usage.Usage = float64(usage.Used) / float64(usage.Total) * 100
		}
	}

	return usage, nil
}

// GetDetailedDiskUsage returns detailed disk information
func GetDetailedDiskUsage() (ResourceUsage, error) {
	var usage ResourceUsage

	cmd := exec.Command("df", "-k", "/")
	output, err := cmd.Output()
	if err != nil {
		return usage, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return usage, fmt.Errorf("could not parse disk usage")
	}

	parts := strings.Fields(lines[1])
	if len(parts) < 4 {
		return usage, fmt.Errorf("could not parse disk usage")
	}

	total, _ := strconv.ParseInt(parts[1], 10, 64)
	used, _ := strconv.ParseInt(parts[2], 10, 64)
	available, _ := strconv.ParseInt(parts[3], 10, 64)

	usage.Total = total
	usage.Used = used
	usage.Available = available
	usage.Free = available
	usage.Usage = float64(used) / float64(total) * 100

	return usage, nil
}

// GetTopProcesses returns top processes by resource usage
func GetTopProcesses(limit int) ([]ProcessInfo, error) {
	var processes []ProcessInfo

	// Get total CPU cores for normalization
	var totalCores int64 = 1 // default fallback
	switch runtime.GOOS {
	case "linux":
		if cmd := exec.Command("nproc"); cmd != nil {
			if output, err := cmd.Output(); err == nil {
				if cores, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64); err == nil {
					totalCores = cores
				}
			}
		}
	case "darwin":
		if cmd := exec.Command("sysctl", "-n", "hw.ncpu"); cmd != nil {
			if output, err := cmd.Output(); err == nil {
				if cores, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64); err == nil {
					totalCores = cores
				}
			}
		}
	}

	switch runtime.GOOS {
	case "linux":
		// Use ps command to get process information
		cmd := exec.Command("ps", "aux", "--sort=-%cpu", "--no-headers")
		output, err := cmd.Output()
		if err != nil {
			return processes, err
		}

		lines := strings.Split(string(output), "\n")
		count := 0

		for _, line := range lines {
			if count >= limit {
				break
			}

			parts := strings.Fields(line)
			if len(parts) < 11 {
				continue
			}

			user := parts[0]
			pid, _ := strconv.Atoi(parts[1])
			cpuPercent, _ := strconv.ParseFloat(parts[2], 64)
			memPercent, _ := strconv.ParseFloat(parts[3], 64)
			memKB, _ := strconv.ParseInt(parts[5], 10, 64)

			// Normalize CPU percentage to total system capacity
			normalizedCPU := cpuPercent / float64(totalCores)

			// Get command (everything after column 10)
			command := strings.Join(parts[10:], " ")
			if len(command) > 50 {
				command = command[:47] + "..."
			}

			// Parse additional process information for Linux
			status := parts[7]                           // Process status (R, S, Z, D)
			ttt := parts[6]                              // TTY
			vsz, _ := strconv.ParseInt(parts[4], 10, 64) // Virtual memory

			// Get start time (simplified - just use current time for now)
			startTime := time.Now().Unix()

			// Get CPU number (simplified - use 0 for now)
			cpuNum := 0

			// Get thread count (simplified - use 1 for now)
			threadCount := 1

			// Priority and Nice (simplified - use defaults for now)
			priority := 20
			nice := 0

			process := ProcessInfo{
				PID:         pid,
				Name:        parts[10],
				CPUUsage:    normalizedCPU,
				MemoryUsage: memPercent,
				MemoryKB:    memKB,
				Command:     command,
				User:        user,

				// New fields
				Status:      status,
				StartTime:   startTime,
				Priority:    priority,
				Nice:        nice,
				VirtualMem:  vsz,
				ResidentMem: memKB, // RSS is same as MemoryKB for now
				TTY:         ttt,
				CPU:         cpuNum,
				Threads:     threadCount,
			}

			processes = append(processes, process)
			count++
		}

	case "darwin":
		// Use ps command for macOS
		cmd := exec.Command("ps", "aux")
		output, err := cmd.Output()
		if err != nil {
			return processes, err
		}

		lines := strings.Split(string(output), "\n")
		count := 0

		for _, line := range lines {
			if count == 0 { // Skip header
				count++
				continue
			}

			if count > limit {
				break
			}

			parts := strings.Fields(line)
			if len(parts) < 11 {
				continue
			}

			user := parts[0]
			pid, _ := strconv.Atoi(parts[1])
			cpuPercent, _ := strconv.ParseFloat(parts[2], 64)
			memPercent, _ := strconv.ParseFloat(parts[3], 64)
			memKB, _ := strconv.ParseInt(parts[5], 10, 64)

			// Normalize CPU percentage to total system capacity
			normalizedCPU := cpuPercent / float64(totalCores)

			// Get command (everything after column 10)
			command := strings.Join(parts[10:], " ")
			if len(command) > 50 {
				command = command[:47] + "..."
			}

			// Parse additional process information for macOS
			status := parts[7]                           // Process status (R, S, Z, D)
			ttt := parts[6]                              // TTY
			vsz, _ := strconv.ParseInt(parts[4], 10, 64) // Virtual memory

			// Get start time (simplified - just use current time for now)
			startTime := time.Now().Unix()

			// Get CPU number (simplified - use 0 for now)
			cpuNum := 0

			// Get thread count (simplified - use 1 for now)
			threadCount := 1

			// Priority and Nice (simplified - use defaults for now)
			priority := 20
			nice := 0

			process := ProcessInfo{
				PID:         pid,
				Name:        parts[10],
				CPUUsage:    normalizedCPU,
				MemoryUsage: memPercent,
				MemoryKB:    memKB,
				Command:     command,
				User:        user,

				// New fields
				Status:      status,
				StartTime:   startTime,
				Priority:    priority,
				Nice:        nice,
				VirtualMem:  vsz,
				ResidentMem: memKB, // RSS is same as MemoryKB for now
				TTY:         ttt,
				CPU:         cpuNum,
				Threads:     threadCount,
			}

			processes = append(processes, process)
			count++
		}
	}

	return processes, nil
}
