package metrics

import (
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

// CPUMetrics contains detailed CPU usage breakdown
type CPUMetrics struct {
	Total  float64 // Overall busy percentage
	User   float64 // User space
	System float64 // Kernel space
	Iowait float64 // I/O wait
	Steal  float64 // Stolen by hypervisor (AWS/GCP!)
	Idle   float64 // Idle time
}

// CPU state cache for delta-based calculation
var (
	lastCpuTimes        cpu.TimesStat
	lastPerCoreCpuTimes []cpu.TimesStat
	lastCpuMetrics      CPUMetrics
	cpuCacheMu          sync.RWMutex
	cpuCacheInitialized bool
	perCoreCacheInit    bool
	lastCpuSampleTime   time.Time
)

// init initializes CPU monitoring by storing initial CPU times
func init() {
	// Initialize total CPU baseline
	if times, err := cpu.Times(false); err == nil && len(times) > 0 {
		lastCpuTimes = times[0]
		cpuCacheInitialized = true
		lastCpuSampleTime = time.Now()
	}

	// Initialize per-core baseline
	if perCoreTimes, err := cpu.Times(true); err == nil {
		lastPerCoreCpuTimes = perCoreTimes
		perCoreCacheInit = true
	}
}

// GetCPUMetrics calculates detailed CPU usage metrics using delta-based calculation.
// Uses cpu.Times() which is non-blocking (reads /proc/stat on Linux, sysctl on macOS).
// Calculates CPU usage as delta between current and previous sample.
// First call returns zero values (baseline), subsequent calls return accurate metrics.
func GetCPUMetrics() (CPUMetrics, error) {
	cpuCacheMu.Lock()
	defer cpuCacheMu.Unlock()

	// Get current CPU times (non-blocking)
	times, err := cpu.Times(false)
	if err != nil || len(times) == 0 {
		return CPUMetrics{}, err
	}

	// First call - just store baseline and return zeros
	if !cpuCacheInitialized {
		lastCpuTimes = times[0]
		lastCpuSampleTime = time.Now()
		cpuCacheInitialized = true
		return CPUMetrics{}, nil
	}

	// Calculate delta from previous sample
	t1 := lastCpuTimes
	t2 := times[0]

	t1All, t1Busy := getAllBusy(t1)
	t2All, t2Busy := getAllBusy(t2)
	totalDelta := t2All - t1All

	var metrics CPUMetrics
	if totalDelta > 0 {
		// Calculate total CPU usage
		totalCPU := clampPercent((t2Busy - t1Busy) / totalDelta * 100)

		metrics = CPUMetrics{
			Total:  totalCPU,
			User:   clampPercent((t2.User - t1.User) / totalDelta * 100),
			System: clampPercent((t2.System - t1.System) / totalDelta * 100),
			Iowait: clampPercent((t2.Iowait - t1.Iowait) / totalDelta * 100),
			Steal:  clampPercent((t2.Steal - t1.Steal) / totalDelta * 100),
			Idle:   clampPercent((t2.Idle - t1.Idle) / totalDelta * 100),
		}
	}

	// Update cache
	lastCpuTimes = times[0]
	lastCpuMetrics = metrics
	lastCpuSampleTime = time.Now()

	return metrics, nil
}

// GetPerCoreCPUUsage calculates per-core CPU busy usage as float64 percentages (0-100).
// Uses cached previous measurements for delta calculation.
func GetPerCoreCPUUsage() ([]float64, error) {
	perCoreTimes, err := cpu.Times(true)
	if err != nil || len(perCoreTimes) == 0 {
		return nil, err
	}

	cpuCacheMu.Lock()
	defer cpuCacheMu.Unlock()

	// If not initialized yet, initialize and return zeros (first call)
	if !perCoreCacheInit {
		lastPerCoreCpuTimes = perCoreTimes
		perCoreCacheInit = true
		return make([]float64, len(perCoreTimes)), nil
	}

	lastTimes := lastPerCoreCpuTimes

	// Limit to the number of cores available in both samples
	length := len(perCoreTimes)
	if len(lastTimes) < length {
		length = len(lastTimes)
	}

	// Calculate per-core usage
	usage := make([]float64, length)
	for i := 0; i < length; i++ {
		t1 := lastTimes[i]
		t2 := perCoreTimes[i]
		usage[i] = calculateBusy(t1, t2)
	}

	// Update cache for next call
	lastPerCoreCpuTimes = perCoreTimes

	return usage, nil
}

// GetPerCoreCPUDetailed calculates detailed CPU metrics for each core
// Returns array of CPUMetrics with breakdown per core
func GetPerCoreCPUDetailed() ([]CPUMetrics, error) {
	perCoreTimes, err := cpu.Times(true)
	if err != nil || len(perCoreTimes) == 0 {
		return nil, err
	}

	cpuCacheMu.Lock()
	defer cpuCacheMu.Unlock()

	// If not initialized yet, initialize and return zeros (first call)
	if !perCoreCacheInit {
		lastPerCoreCpuTimes = perCoreTimes
		perCoreCacheInit = true
		return make([]CPUMetrics, len(perCoreTimes)), nil
	}

	lastTimes := lastPerCoreCpuTimes

	// Limit to the number of cores available in both samples
	length := len(perCoreTimes)
	if len(lastTimes) < length {
		length = len(lastTimes)
	}

	// Calculate detailed metrics per core
	metrics := make([]CPUMetrics, length)
	for i := 0; i < length; i++ {
		t1 := lastTimes[i]
		t2 := perCoreTimes[i]

		t1All, _ := getAllBusy(t1)
		t2All, _ := getAllBusy(t2)

		totalDelta := t2All - t1All
		if totalDelta <= 0 {
			continue
		}

		metrics[i] = CPUMetrics{
			Total:  calculateBusy(t1, t2),
			User:   clampPercent((t2.User - t1.User) / totalDelta * 100),
			System: clampPercent((t2.System - t1.System) / totalDelta * 100),
			Iowait: clampPercent((t2.Iowait - t1.Iowait) / totalDelta * 100),
			Steal:  clampPercent((t2.Steal - t1.Steal) / totalDelta * 100),
			Idle:   clampPercent((t2.Idle - t1.Idle) / totalDelta * 100),
		}
	}

	// Update cache for next call
	lastPerCoreCpuTimes = perCoreTimes

	return metrics, nil
}

// calculateBusy calculates the CPU busy percentage between two time points.
// Returns a percentage clamped between 0 and 100.
func calculateBusy(t1, t2 cpu.TimesStat) float64 {
	t1All, t1Busy := getAllBusy(t1)
	t2All, t2Busy := getAllBusy(t2)

	if t2All <= t1All || t2Busy <= t1Busy {
		return 0
	}

	return clampPercent((t2Busy - t1Busy) / (t2All - t1All) * 100)
}

// getAllBusy calculates total CPU time and busy CPU time from CPU times statistics.
// On Linux, it excludes guest and guest_nice time from total to match htop behavior.
// Returns (total CPU time, busy CPU time).
func getAllBusy(t cpu.TimesStat) (float64, float64) {
	tot := t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq +
		t.Softirq + t.Steal + t.Guest + t.GuestNice

	// On Linux, remove guest time from total (to match htop)
	if runtime.GOOS == "linux" {
		tot -= t.Guest     // Linux 2.6.24+
		tot -= t.GuestNice // Linux 3.2.0+
	}

	// Busy = total - idle - iowait
	busy := tot - t.Idle - t.Iowait

	return tot, busy
}

// clampPercent ensures the percentage is between 0 and 100
func clampPercent(value float64) float64 {
	return math.Min(100, math.Max(0, value))
}
