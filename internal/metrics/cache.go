package metrics

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

const (
	metricsCacheFile = "/tmp/catops_metrics_cache.json"
	cacheMaxAge      = 30 * time.Second // Cache is valid for 30 seconds
)

var (
	cacheMutex sync.RWMutex
)

// CachedMetrics contains metrics with timestamp
type CachedMetrics struct {
	Metrics   *Metrics  `json:"metrics"`
	Timestamp time.Time `json:"timestamp"`
}

// SaveMetricsToCache saves metrics to cache file
func SaveMetricsToCache(m *Metrics) error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	cached := CachedMetrics{
		Metrics:   m,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}

	// Write to temp file first, then rename (atomic operation)
	tmpFile := metricsCacheFile + ".tmp"
	err = os.WriteFile(tmpFile, data, 0644)
	if err != nil {
		return err
	}

	return os.Rename(tmpFile, metricsCacheFile)
}

// LoadMetricsFromCache loads metrics from cache file
// Returns metrics and true if cache is fresh (< 30 seconds old)
// Returns nil and false if cache is stale or doesn't exist
func LoadMetricsFromCache() (*Metrics, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	// Check if file exists
	if _, err := os.Stat(metricsCacheFile); os.IsNotExist(err) {
		return nil, false
	}

	// Read file
	data, err := os.ReadFile(metricsCacheFile)
	if err != nil {
		return nil, false
	}

	// Unmarshal
	var cached CachedMetrics
	err = json.Unmarshal(data, &cached)
	if err != nil {
		return nil, false
	}

	// Check if cache is fresh
	age := time.Since(cached.Timestamp)
	if age > cacheMaxAge {
		return nil, false // Cache is stale
	}

	return cached.Metrics, true
}

// GetMetricsWithCache returns cached metrics if available and fresh,
// otherwise collects fresh metrics and caches them
func GetMetricsWithCache() (*Metrics, error) {
	// Try cache first
	if cachedMetrics, ok := LoadMetricsFromCache(); ok {
		return cachedMetrics, nil
	}

	// Cache miss or stale - collect fresh metrics
	freshMetrics, err := GetMetrics()
	if err != nil {
		return nil, err
	}

	// Save to cache (ignore errors - not critical)
	_ = SaveMetricsToCache(freshMetrics)

	return freshMetrics, nil
}

// ClearMetricsCache removes the cache file
func ClearMetricsCache() error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if _, err := os.Stat(metricsCacheFile); os.IsNotExist(err) {
		return nil // Already deleted
	}

	return os.Remove(metricsCacheFile)
}

// GetCacheFilePath returns the cache file path (for debugging)
func GetCacheFilePath() string {
	return metricsCacheFile
}

// GetCacheAge returns the age of the cache file
func GetCacheAge() (time.Duration, error) {
	info, err := os.Stat(metricsCacheFile)
	if err != nil {
		return 0, err
	}

	return time.Since(info.ModTime()), nil
}
