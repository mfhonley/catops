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

// MetricTimeseries stores a timeseries of metric values
type MetricTimeseries struct {
	points []MetricPoint
	max    int // Maximum points to store
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
		points: make([]MetricPoint, 0, max),
		max:    max,
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

// addPoint adds a point and removes oldest if buffer is full
func (ts *MetricTimeseries) addPoint(point MetricPoint) {
	ts.points = append(ts.points, point)

	// Remove oldest point if buffer is full
	if len(ts.points) > ts.max {
		ts.points = ts.points[1:]
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

// getStatistics calculates statistics for a time window
func (ts *MetricTimeseries) getStatistics(windowMinutes int) MetricStatistics {
	if len(ts.points) == 0 {
		return MetricStatistics{}
	}

	// Get points within time window
	cutoff := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
	var values []float64

	for _, p := range ts.points {
		if p.Timestamp.After(cutoff) {
			values = append(values, p.Value)
		}
	}

	if len(values) == 0 {
		return MetricStatistics{
			Current: ts.points[len(ts.points)-1].Value,
		}
	}

	// Sort for percentiles
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// Calculate statistics
	stats := MetricStatistics{
		Current: ts.points[len(ts.points)-1].Value,
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

// detectSpike performs spike detection analysis
func (ts *MetricTimeseries) detectSpike(windowMinutes int, suddenSpikeThreshold, gradualRiseThreshold, anomalyThreshold float64) SpikeDetectionResult {
	result := SpikeDetectionResult{
		Stats: ts.getStatistics(windowMinutes),
	}

	if len(ts.points) == 0 {
		return result
	}

	result.CurrentValue = ts.points[len(ts.points)-1].Value

	// Check for sudden spike (comparison with previous point)
	if len(ts.points) >= 2 {
		previous := ts.points[len(ts.points)-2]
		result.PreviousValue = previous.Value

		// Calculate percent change
		if previous.Value > 0 {
			result.PercentChange = ((result.CurrentValue - previous.Value) / previous.Value) * 100
		} else {
			result.PercentChange = 0
		}

		// Sudden spike: >suddenSpikeThreshold% increase in one interval
		if result.PercentChange > suddenSpikeThreshold {
			result.HasSuddenSpike = true
		}
	}

	// Check for gradual rise (comparison with oldest point in window)
	cutoff := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
	var oldestInWindow *MetricPoint

	for i := range ts.points {
		if ts.points[i].Timestamp.After(cutoff) {
			oldestInWindow = &ts.points[i]
			break
		}
	}

	if oldestInWindow != nil {
		result.ChangeOverWindow = result.CurrentValue - oldestInWindow.Value

		// Gradual rise: >gradualRiseThreshold% increase over window
		// Calculate percent change to compare with threshold correctly
		if oldestInWindow.Value > 0 {
			percentChange := (result.ChangeOverWindow / oldestInWindow.Value) * 100
			if percentChange > gradualRiseThreshold {
				result.HasGradualRise = true
			}
		} else if result.ChangeOverWindow > gradualRiseThreshold {
			// Fallback: if oldestInWindow.Value is 0, compare absolute change
			result.HasGradualRise = true
		}
	}

	// Check for anomalous value (statistical anomaly)
	if result.Stats.StdDev > 0 {
		result.DeviationFromAvg = math.Abs(result.CurrentValue-result.Stats.Avg) / result.Stats.StdDev

		// Anomalous: exceeds configured threshold (configurable via anomaly_threshold)
		if result.DeviationFromAvg > anomalyThreshold {
			result.HasAnomalousValue = true
		}
	}

	return result
}

// GetCPUHistory returns CPU history points (for debugging/testing)
func (mb *MetricsBuffer) GetCPUHistory() []MetricPoint {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	result := make([]MetricPoint, len(mb.cpu.points))
	copy(result, mb.cpu.points)
	return result
}

// GetMemoryHistory returns Memory history points (for debugging/testing)
func (mb *MetricsBuffer) GetMemoryHistory() []MetricPoint {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	result := make([]MetricPoint, len(mb.memory.points))
	copy(result, mb.memory.points)
	return result
}

// GetDiskHistory returns Disk history points (for debugging/testing)
func (mb *MetricsBuffer) GetDiskHistory() []MetricPoint {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	result := make([]MetricPoint, len(mb.disk.points))
	copy(result, mb.disk.points)
	return result
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
