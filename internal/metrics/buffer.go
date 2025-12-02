package metrics

import (
	"math"
	"sort"
	"sync"
	"time"

	constants "catops/config"
)

// MetricsBuffer stores historical metrics for analysis
type MetricsBuffer struct {
	cpu     *MetricTimeseries
	memory  *MetricTimeseries
	disk    *MetricTimeseries
	maxSize int
	mutex   sync.RWMutex
}

// MetricTimeseries stores a timeseries of metric values using a ring buffer
// for O(1) insert/remove instead of O(n) slice operations
type MetricTimeseries struct {
	points []MetricPoint
	max    int // Maximum points to store
	head   int // Index of oldest element
	count  int // Current number of elements
}

// MetricPoint represents a single metric measurement
type MetricPoint struct {
	Timestamp time.Time
	Value     float64
}

// MetricStatistics holds statistical analysis of metrics
type MetricStatistics struct {
	Current float64 // Current value
	Min     float64 // Minimum in window
	Max     float64 // Maximum in window
	Avg     float64 // Average in window
	P50     float64 // Median (50th percentile)
	P95     float64 // 95th percentile
	P99     float64 // 99th percentile
	StdDev  float64 // Standard deviation
}

// SpikeDetectionResult contains spike detection analysis
type SpikeDetectionResult struct {
	HasSuddenSpike    bool    // Rapid change in one interval (configurable via sudden_spike_threshold)
	HasGradualRise    bool    // Sustained increase over 5-minute window (configurable via gradual_rise_threshold)
	HasAnomalousValue bool    // Statistical anomaly (configurable via anomaly_threshold in standard deviations)
	CurrentValue      float64 // Current metric value
	PreviousValue     float64 // Previous metric value
	PercentChange     float64 // Percent change from previous
	ChangeOverWindow  float64 // Change over entire window
	DeviationFromAvg  float64 // How many stddev from average
	Stats             MetricStatistics
}

// NewMetricsBuffer creates a new metrics buffer
// maxSize: maximum number of points to store (e.g., 20 for 5 minutes at 15s intervals)
func NewMetricsBuffer(maxSize int) *MetricsBuffer {
	return &MetricsBuffer{
		cpu:     newMetricTimeseries(maxSize),
		memory:  newMetricTimeseries(maxSize),
		disk:    newMetricTimeseries(maxSize),
		maxSize: maxSize,
	}
}

func newMetricTimeseries(max int) *MetricTimeseries {
	return &MetricTimeseries{
		points: make([]MetricPoint, max), // Pre-allocate full size for ring buffer
		max:    max,
		head:   0,
		count:  0,
	}
}

// AddCPUPoint adds a CPU metric point to the buffer
func (mb *MetricsBuffer) AddCPUPoint(value float64) {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	mb.cpu.addPoint(MetricPoint{Timestamp: time.Now(), Value: value})
}

// AddMemoryPoint adds a Memory metric point to the buffer
func (mb *MetricsBuffer) AddMemoryPoint(value float64) {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	mb.memory.addPoint(MetricPoint{Timestamp: time.Now(), Value: value})
}

// AddDiskPoint adds a Disk metric point to the buffer
func (mb *MetricsBuffer) AddDiskPoint(value float64) {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	mb.disk.addPoint(MetricPoint{Timestamp: time.Now(), Value: value})
}

// addPoint adds a point using ring buffer - O(1) instead of O(n)
func (ts *MetricTimeseries) addPoint(point MetricPoint) {
	// Calculate insertion index
	insertIdx := (ts.head + ts.count) % ts.max

	ts.points[insertIdx] = point

	if ts.count < ts.max {
		ts.count++
	} else {
		// Buffer is full, move head forward (overwrite oldest)
		ts.head = (ts.head + 1) % ts.max
	}
}

// GetCPUStatistics calculates statistics for CPU in a time window
func (mb *MetricsBuffer) GetCPUStatistics(windowMinutes int) MetricStatistics {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.cpu.getStatistics(windowMinutes)
}

// GetMemoryStatistics calculates statistics for Memory in a time window
func (mb *MetricsBuffer) GetMemoryStatistics(windowMinutes int) MetricStatistics {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.memory.getStatistics(windowMinutes)
}

// GetDiskStatistics calculates statistics for Disk in a time window
func (mb *MetricsBuffer) GetDiskStatistics(windowMinutes int) MetricStatistics {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.disk.getStatistics(windowMinutes)
}

// DetectCPUSpike performs spike detection on CPU metrics
func (mb *MetricsBuffer) DetectCPUSpike(suddenSpikeThreshold, gradualRiseThreshold, anomalyThreshold float64) SpikeDetectionResult {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.cpu.detectSpike(constants.DETECTION_WINDOW_MINUTES, suddenSpikeThreshold, gradualRiseThreshold, anomalyThreshold)
}

// DetectMemorySpike performs spike detection on Memory metrics
func (mb *MetricsBuffer) DetectMemorySpike(suddenSpikeThreshold, gradualRiseThreshold, anomalyThreshold float64) SpikeDetectionResult {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.memory.detectSpike(constants.DETECTION_WINDOW_MINUTES, suddenSpikeThreshold, gradualRiseThreshold, anomalyThreshold)
}

// DetectDiskSpike performs spike detection on Disk metrics
func (mb *MetricsBuffer) DetectDiskSpike(suddenSpikeThreshold, gradualRiseThreshold, anomalyThreshold float64) SpikeDetectionResult {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.disk.detectSpike(constants.DETECTION_WINDOW_MINUTES, suddenSpikeThreshold, gradualRiseThreshold, anomalyThreshold)
}

// getStatistics calculates statistics for a time window (ring buffer aware)
func (ts *MetricTimeseries) getStatistics(windowMinutes int) MetricStatistics {
	if ts.count == 0 {
		return MetricStatistics{}
	}

	// Get points within time window from ring buffer
	cutoff := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
	values := make([]float64, 0, ts.count)

	// Iterate through ring buffer in order (oldest to newest)
	for i := 0; i < ts.count; i++ {
		idx := (ts.head + i) % ts.max
		p := ts.points[idx]
		if p.Timestamp.After(cutoff) {
			values = append(values, p.Value)
		}
	}

	// Get current (newest) value
	newestIdx := (ts.head + ts.count - 1) % ts.max
	currentValue := ts.points[newestIdx].Value

	if len(values) == 0 {
		return MetricStatistics{
			Current: currentValue,
		}
	}

	// Sort for percentiles
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// Calculate statistics
	stats := MetricStatistics{
		Current: currentValue,
		Min:     sorted[0],
		Max:     sorted[len(sorted)-1],
		Avg:     average(values),
		P50:     percentile(sorted, 50),
		P95:     percentile(sorted, 95),
		P99:     percentile(sorted, 99),
		StdDev:  stdDev(values),
	}

	return stats
}

// detectSpike performs spike detection analysis (ring buffer aware)
func (ts *MetricTimeseries) detectSpike(windowMinutes int, suddenSpikeThreshold, gradualRiseThreshold, anomalyThreshold float64) SpikeDetectionResult {
	result := SpikeDetectionResult{
		Stats: ts.getStatistics(windowMinutes),
	}

	if ts.count == 0 {
		return result
	}

	// Get current (newest) value from ring buffer
	newestIdx := (ts.head + ts.count - 1) % ts.max
	result.CurrentValue = ts.points[newestIdx].Value

	// Check for sudden spike (comparison with previous point)
	if ts.count >= 2 {
		prevIdx := (ts.head + ts.count - 2) % ts.max
		previous := ts.points[prevIdx]
		result.PreviousValue = previous.Value

		// Calculate percent change
		if previous.Value > 0 {
			result.PercentChange = ((result.CurrentValue - previous.Value) / previous.Value) * 100
		} else {
			result.PercentChange = 0
		}

		// Sudden spike: требуется И относительное И абсолютное изменение
		// Игнорируем малые значения (< 10%) чтобы избежать "1% → 2% = +100%"
		// Также требуем абсолютное изменение > 5% для фильтрации шума
		absoluteChange := result.CurrentValue - result.PreviousValue
		if result.PercentChange > suddenSpikeThreshold && result.CurrentValue > 10.0 && absoluteChange > 5.0 {
			result.HasSuddenSpike = true
		}
	}

	// Check for gradual rise (comparison with oldest point in window)
	cutoff := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
	var oldestInWindow *MetricPoint

	// Iterate through ring buffer in order (oldest to newest)
	for i := 0; i < ts.count; i++ {
		idx := (ts.head + i) % ts.max
		if ts.points[idx].Timestamp.After(cutoff) {
			oldestInWindow = &ts.points[idx]
			break
		}
	}

	if oldestInWindow != nil {
		result.ChangeOverWindow = result.CurrentValue - oldestInWindow.Value

		// Gradual rise: >gradualRiseThreshold% increase over window
		// Calculate percent change to compare with threshold correctly
		// Также проверяем абсолютное значение и изменение для фильтрации шума
		if oldestInWindow.Value > 0 {
			percentChange := (result.ChangeOverWindow / oldestInWindow.Value) * 100
			// Требуем: относительное изменение > порога И текущее значение > 10% И абсолютное изменение > 5%
			if percentChange > gradualRiseThreshold && result.CurrentValue > 10.0 && result.ChangeOverWindow > 5.0 {
				result.HasGradualRise = true
			}
		} else if result.ChangeOverWindow > gradualRiseThreshold && result.ChangeOverWindow > 5.0 {
			// Fallback: if oldestInWindow.Value is 0, compare absolute change
			result.HasGradualRise = true
		}
	}

	// Check for anomalous value (statistical anomaly)
	if result.Stats.StdDev > 0 {
		result.DeviationFromAvg = math.Abs(result.CurrentValue-result.Stats.Avg) / result.Stats.StdDev

		// Anomalous: exceeds configured threshold (configurable via anomaly_threshold)
		// Также проверяем абсолютное значение > 10% для фильтрации шума на малых значениях
		if result.DeviationFromAvg > anomalyThreshold && result.CurrentValue > 10.0 {
			result.HasAnomalousValue = true
		}
	}

	return result
}

// getOrderedPoints returns ring buffer points in order (oldest to newest)
func (ts *MetricTimeseries) getOrderedPoints() []MetricPoint {
	if ts.count == 0 {
		return []MetricPoint{}
	}
	result := make([]MetricPoint, ts.count)
	for i := 0; i < ts.count; i++ {
		idx := (ts.head + i) % ts.max
		result[i] = ts.points[idx]
	}
	return result
}

// GetCPUHistory returns CPU history points (for debugging/testing)
func (mb *MetricsBuffer) GetCPUHistory() []MetricPoint {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.cpu.getOrderedPoints()
}

// GetMemoryHistory returns Memory history points (for debugging/testing)
func (mb *MetricsBuffer) GetMemoryHistory() []MetricPoint {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.memory.getOrderedPoints()
}

// GetDiskHistory returns Disk history points (for debugging/testing)
func (mb *MetricsBuffer) GetDiskHistory() []MetricPoint {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.disk.getOrderedPoints()
}

// Helper functions for statistics

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func percentile(sortedValues []float64, p int) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	index := int(float64(len(sortedValues)-1) * float64(p) / 100.0)
	if index >= len(sortedValues) {
		index = len(sortedValues) - 1
	}
	return sortedValues[index]
}

func stdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	avg := average(values)
	variance := 0.0

	for _, v := range values {
		diff := v - avg
		variance += diff * diff
	}

	variance /= float64(len(values))
	return math.Sqrt(variance)
}
