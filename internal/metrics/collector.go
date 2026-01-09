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
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// =============================================================================
// Collector State
// =============================================================================

var (
	// Previous values for rate calculations
	prevNetStats  map[string]net.IOCountersStat
	prevDiskStats map[string]disk.IOCountersStat
	prevStatsTime time.Time
	prevStatsMu   sync.RWMutex

	// Delta tracking - для оптимизации отправки метрик
	lastSentMetrics *AllMetrics
	lastSentTime    time.Time
	deltaTrackingMu sync.RWMutex

	// Per-cycle cache for expensive operations (reused within single collection cycle)
	cycleProcesses   []*process.Process
	cycleConnections []net.ConnectionStat
	cycleCacheMu     sync.RWMutex
	cycleCacheTime   time.Time

	// Process CPU tracking for delta-based calculation (like htop does)
	prevProcCPUTimes map[int32]float64 // PID -> total CPU time (user + system)
	prevProcCPUTime  time.Time
	prevProcCPUMu    sync.RWMutex
)

// =============================================================================
// Metrics Collection
// =============================================================================

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

// CollectAllMetrics collects all system metrics and updates the cache
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
		SetCachedMetrics(m)

		// Сохраняем как последние отправленные метрики
		deltaTrackingMu.Lock()
		lastSentMetrics = m
		lastSentTime = time.Now()
		deltaTrackingMu.Unlock()
	} else {
		// Нет значительных изменений - возвращаем старые метрики из кэша
		// Это предотвратит отправку данных в OpenTelemetry
		m = GetCachedMetrics()
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

// =============================================================================
// System Summary Collection
// =============================================================================

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

// =============================================================================
// Per-Resource Collection
// =============================================================================

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

// =============================================================================
// Container Collection
// =============================================================================

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

	// Collect logs for containers using global log collector (for deduplication)
	logCollector := GetLogCollector()
	for i := range containers {
		logs, _ := logCollector.CollectContainerLogs(containers[i].ContainerID)
		containers[i].RecentLogs = logs
	}

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
