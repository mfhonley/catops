package metrics

import (
	"math"
	"testing"
	"time"
)

func TestMetricsBuffer_AddPoint(t *testing.T) {
	buffer := NewMetricsBuffer(5)

	// Add 3 points
	buffer.AddCPUPoint(50.0)
	time.Sleep(10 * time.Millisecond)
	buffer.AddCPUPoint(60.0)
	time.Sleep(10 * time.Millisecond)
	buffer.AddCPUPoint(70.0)

	stats := buffer.GetCPUStatistics(1)

	if stats.Min != 50.0 {
		t.Errorf("Expected Min=50.0, got %.1f", stats.Min)
	}
	if stats.Max != 70.0 {
		t.Errorf("Expected Max=70.0, got %.1f", stats.Max)
	}
	if math.Abs(stats.Avg-60.0) > 0.1 {
		t.Errorf("Expected Avg≈60.0, got %.1f", stats.Avg)
	}
}

func TestMetricsBuffer_BufferOverflow(t *testing.T) {
	buffer := NewMetricsBuffer(3) // Small buffer

	// Add more points than buffer size
	buffer.AddCPUPoint(10.0)
	buffer.AddCPUPoint(20.0)
	buffer.AddCPUPoint(30.0)
	buffer.AddCPUPoint(40.0) // Should evict 10.0
	buffer.AddCPUPoint(50.0) // Should evict 20.0

	stats := buffer.GetCPUStatistics(1)

	// Should only have last 3 points: 30, 40, 50
	if stats.Min != 30.0 {
		t.Errorf("Expected Min=30.0 (oldest should be evicted), got %.1f", stats.Min)
	}
	if stats.Max != 50.0 {
		t.Errorf("Expected Max=50.0, got %.1f", stats.Max)
	}
}

func TestMetricsBuffer_Percentiles(t *testing.T) {
	buffer := NewMetricsBuffer(100)

	// Add 100 points from 1 to 100
	for i := 1; i <= 100; i++ {
		buffer.AddCPUPoint(float64(i))
	}

	stats := buffer.GetCPUStatistics(10)

	// P50 should be around 50
	if stats.P50 < 49 || stats.P50 > 51 {
		t.Errorf("Expected P50≈50, got %.1f", stats.P50)
	}

	// P95 should be around 95
	if stats.P95 < 94 || stats.P95 > 96 {
		t.Errorf("Expected P95≈95, got %.1f", stats.P95)
	}

	// P99 should be around 99
	if stats.P99 < 98 || stats.P99 > 100 {
		t.Errorf("Expected P99≈99, got %.1f", stats.P99)
	}
}

func TestMetricsBuffer_SuddenSpike(t *testing.T) {
	buffer := NewMetricsBuffer(20)

	// Add stable points
	for i := 0; i < 10; i++ {
		buffer.AddCPUPoint(30.0)
		time.Sleep(1 * time.Millisecond)
	}

	// Add sudden spike
	buffer.AddCPUPoint(95.0) // +65% jump

	result := buffer.DetectCPUSpike(20.0, 10.0)

	if !result.HasSuddenSpike {
		t.Error("Expected sudden spike detection, but got none")
	}

	if result.PercentChange < 200 { // 65/30 * 100 ≈ 216%
		t.Errorf("Expected large percent change, got %.1f%%", result.PercentChange)
	}

	t.Logf("Spike detected: %.1f%% → %.1f%% (change: %.1f%%)",
		result.PreviousValue, result.CurrentValue, result.PercentChange)
}

func TestMetricsBuffer_GradualRise(t *testing.T) {
	buffer := NewMetricsBuffer(20)

	// Simulate gradual memory leak: 50% → 65% over 20 points
	for i := 0; i < 20; i++ {
		value := 50.0 + float64(i)*0.75 // Increases by 0.75% each point
		buffer.AddMemoryPoint(value)
		time.Sleep(1 * time.Millisecond)
	}

	result := buffer.DetectMemorySpike(20.0, 10.0)

	if !result.HasGradualRise {
		t.Error("Expected gradual rise detection, but got none")
	}

	if result.ChangeOverWindow < 10 {
		t.Errorf("Expected change >10%%, got %.1f%%", result.ChangeOverWindow)
	}

	t.Logf("Gradual rise detected: change over window = %.1f%%", result.ChangeOverWindow)
}

func TestMetricsBuffer_NoFalsePositives(t *testing.T) {
	buffer := NewMetricsBuffer(20)

	// Add stable values (normal fluctuation)
	values := []float64{45, 47, 46, 48, 47, 49, 48, 46, 47, 48}
	for _, v := range values {
		buffer.AddCPUPoint(v)
		time.Sleep(1 * time.Millisecond)
	}

	result := buffer.DetectCPUSpike(20.0, 10.0)

	if result.HasSuddenSpike {
		t.Error("False positive: detected spike in stable data")
	}

	if result.HasGradualRise {
		t.Error("False positive: detected gradual rise in stable data")
	}
}

func TestMetricsBuffer_StatisticalAnomaly(t *testing.T) {
	buffer := NewMetricsBuffer(50)

	// Add stable values around 50
	for i := 0; i < 40; i++ {
		buffer.AddCPUPoint(50.0)
		time.Sleep(1 * time.Millisecond)
	}

	// Add anomalous value (very different from average)
	buffer.AddCPUPoint(95.0)

	result := buffer.DetectCPUSpike(20.0, 10.0)

	if !result.HasAnomalousValue {
		t.Error("Expected anomaly detection, but got none")
	}

	t.Logf("Anomaly detected: %.1f stddev from average", result.DeviationFromAvg)
}

func TestMetricsBuffer_ConcurrentAccess(t *testing.T) {
	buffer := NewMetricsBuffer(100)

	// Test thread safety
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			buffer.AddCPUPoint(float64(i))
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = buffer.GetCPUStatistics(1)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done

	// Verify data integrity
	stats := buffer.GetCPUStatistics(10)
	if stats.Current < 0 || stats.Current > 100 {
		t.Errorf("Concurrent access corrupted data: current=%.1f", stats.Current)
	}
}

func TestMetricsBuffer_GradualRisePercentage(t *testing.T) {
	buffer := NewMetricsBuffer(20)

	// Test percent-based gradual rise detection
	// Start at 10%, rise to 22% = +120% increase (should trigger 10% threshold)
	buffer.AddCPUPoint(10.0)
	time.Sleep(1 * time.Millisecond)

	for i := 0; i < 10; i++ {
		value := 10.0 + float64(i)*1.2
		buffer.AddCPUPoint(value)
		time.Sleep(1 * time.Millisecond)
	}

	buffer.AddCPUPoint(22.0) // Final value
	time.Sleep(1 * time.Millisecond)

	result := buffer.DetectCPUSpike(20.0, 10.0)

	// Change: 10% → 22% = +12% absolute = +120% percent change
	// Should trigger gradual rise (threshold: 10%)
	if !result.HasGradualRise {
		t.Errorf("Expected gradual rise detection for 10%% → 22%% (+120%% change)")
	}

	t.Logf("Gradual rise: 10%% → 22%% (absolute change: %.1f, should be detected as >10%% threshold)",
		result.ChangeOverWindow)
}

func TestMetricsBuffer_EmptyBuffer(t *testing.T) {
	buffer := NewMetricsBuffer(10)

	// Test with empty buffer
	stats := buffer.GetCPUStatistics(1)

	if stats.Current != 0 || stats.Avg != 0 {
		t.Error("Empty buffer should return zero stats")
	}

	result := buffer.DetectCPUSpike(20.0, 10.0)

	if result.HasSuddenSpike || result.HasGradualRise {
		t.Error("Empty buffer should not detect spikes")
	}
}

func TestMetricsBuffer_SinglePoint(t *testing.T) {
	buffer := NewMetricsBuffer(10)

	buffer.AddCPUPoint(50.0)

	stats := buffer.GetCPUStatistics(1)

	if stats.Current != 50.0 {
		t.Errorf("Expected Current=50.0, got %.1f", stats.Current)
	}

	// With only one point, can't detect spikes
	result := buffer.DetectCPUSpike(20.0, 10.0)

	if result.HasSuddenSpike {
		t.Error("Can't detect spike with single point")
	}
}
