// Package metrics provides legacy API for backward compatibility with UI.
package metrics

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

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

	TopProcesses   []ProcessInfo   `json:"top_processes"`
	NetworkMetrics *NetworkMetrics `json:"network_metrics,omitempty"`
	Services       []ServiceInfo   `json:"services,omitempty"`
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
