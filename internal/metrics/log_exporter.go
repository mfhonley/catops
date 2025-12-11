package metrics

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	constants "catops/config"
)

// =============================================================================
// Constants
// =============================================================================

const (
	// Collection settings
	LogCollectionInterval = 30 * time.Second
	MaxLogsPerBatch       = 500
	MaxLogMessageLength   = 2048
	MaxBufferSize         = 10000 // Max logs in memory buffer

	// File tailing
	DefaultTailLines  = 100
	MaxFileSize       = 100 * 1024 * 1024 // 100MB max file to tail
	FileCheckInterval = 5 * time.Second

	// Disk buffer
	DiskBufferDir      = ".catops/logs_buffer"
	MaxDiskBufferFiles = 100
	MaxDiskBufferSize  = 50 * 1024 * 1024 // 50MB total

	// HTTP settings
	HTTPTimeout    = 30 * time.Second
	MaxRetries     = 3
	RetryBaseDelay = 1 * time.Second
)

// =============================================================================
// Types
// =============================================================================

// LogLevel represents log severity
type LogLevel string

const (
	LogLevelTrace LogLevel = "trace"
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp   time.Time         `json:"timestamp"`
	Level       LogLevel          `json:"level"`
	Message     string            `json:"message"`
	Source      string            `json:"source"`       // journald, docker, file, syslog
	SourcePath  string            `json:"source_path"`  // file path or container name
	Service     string            `json:"service"`      // detected service name
	ContainerID string            `json:"container_id"` // docker container ID if applicable
	PID         int               `json:"pid"`          // process ID if available
	Hostname    string            `json:"hostname"`
	Facility    string            `json:"facility"`   // syslog facility
	AppName     string            `json:"app_name"`   // application name
	Fields      map[string]string `json:"fields"`     // parsed structured fields
	MessageHash string            `json:"message_hash"` // for deduplication
}

// LogSource represents a log source configuration
type LogSource struct {
	Type        string   `json:"type"`         // journald, docker, file, syslog
	Path        string   `json:"path"`         // file path or unit name
	Service     string   `json:"service"`      // service name
	ContainerID string   `json:"container_id"` // container ID
	Patterns    []string `json:"patterns"`     // include patterns (regex)
	Excludes    []string `json:"excludes"`     // exclude patterns
}

// FileTailer tracks file position for tailing
type FileTailer struct {
	Path     string
	Offset   int64
	Inode    uint64
	ModTime  time.Time
	Service  string
	LastRead time.Time
}

// LogExporter handles log collection and export to CatOps backend
type LogExporter struct {
	mu sync.RWMutex

	// Configuration
	serverID  string
	authToken string
	hostname  string
	endpoint  string

	// Log sources
	sources     []LogSource
	fileTailers map[string]*FileTailer

	// Buffer
	buffer     []LogEntry
	bufferMu   sync.Mutex
	seenHashes map[string]time.Time // for deduplication

	// State
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	lastFlush time.Time

	// HTTP client
	httpClient *http.Client

	// Patterns for log level detection
	levelPatterns map[LogLevel]*regexp.Regexp

	// Multiline patterns for stack traces
	multilineStart *regexp.Regexp
	multilineCont  *regexp.Regexp

	// Stats
	stats LogExporterStats
}

// LogExporterStats tracks exporter statistics
type LogExporterStats struct {
	LogsCollected   int64
	LogsSent        int64
	LogsDropped     int64
	LogsBuffered    int64
	Errors          int64
	LastSendTime    time.Time
	LastCollectTime time.Time
}

// LogExporterConfig holds configuration for log exporter
type LogExporterConfig struct {
	ServerID     string
	AuthToken    string
	Hostname     string
	Sources      []LogSource
	CustomFiles  []string // Additional files to tail
}

// =============================================================================
// OTLP Log Format (simplified for HTTP/JSON)
// =============================================================================

// OTLPLogRecord represents a single log record in OTLP format
type OTLPLogRecord struct {
	TimeUnixNano         int64             `json:"timeUnixNano,string"`
	SeverityNumber       int               `json:"severityNumber"`
	SeverityText         string            `json:"severityText"`
	Body                 OTLPAnyValue      `json:"body"`
	Attributes           []OTLPKeyValue    `json:"attributes,omitempty"`
	DroppedAttributesCount int             `json:"droppedAttributesCount,omitempty"`
	TraceID              string            `json:"traceId,omitempty"`
	SpanID               string            `json:"spanId,omitempty"`
}

// OTLPAnyValue represents a value in OTLP format
type OTLPAnyValue struct {
	StringValue string `json:"stringValue,omitempty"`
}

// OTLPKeyValue represents a key-value pair
type OTLPKeyValue struct {
	Key   string       `json:"key"`
	Value OTLPAnyValue `json:"value"`
}

// OTLPScopeLogs represents logs from a scope
type OTLPScopeLogs struct {
	Scope      OTLPScope       `json:"scope"`
	LogRecords []OTLPLogRecord `json:"logRecords"`
}

// OTLPScope represents instrumentation scope
type OTLPScope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// OTLPResourceLogs represents logs from a resource
type OTLPResourceLogs struct {
	Resource  OTLPResource    `json:"resource"`
	ScopeLogs []OTLPScopeLogs `json:"scopeLogs"`
}

// OTLPResource represents a resource
type OTLPResource struct {
	Attributes []OTLPKeyValue `json:"attributes"`
}

// OTLPLogsRequest represents the full OTLP logs request
type OTLPLogsRequest struct {
	ResourceLogs []OTLPResourceLogs `json:"resourceLogs"`
}

// =============================================================================
// LogExporter Implementation
// =============================================================================

// NewLogExporter creates a new log exporter
func NewLogExporter(cfg *LogExporterConfig) (*LogExporter, error) {
	if cfg.ServerID == "" || cfg.AuthToken == "" {
		return nil, fmt.Errorf("serverID and authToken are required")
	}

	hostname := cfg.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	le := &LogExporter{
		serverID:    cfg.ServerID,
		authToken:   cfg.AuthToken,
		hostname:    hostname,
		endpoint:    fmt.Sprintf("https://%s%s", constants.OTLP_ENDPOINT, constants.OTLP_LOGS_PATH),
		sources:     cfg.Sources,
		fileTailers: make(map[string]*FileTailer),
		buffer:      make([]LogEntry, 0, MaxLogsPerBatch),
		seenHashes:  make(map[string]time.Time),
		httpClient: &http.Client{
			Timeout: HTTPTimeout,
		},
		levelPatterns: map[LogLevel]*regexp.Regexp{
			LogLevelFatal: regexp.MustCompile(`(?i)\b(fatal|critical|emergency)\b`),
			LogLevelError: regexp.MustCompile(`(?i)\b(error|err|fail|failed|failure|exception|panic)\b`),
			LogLevelWarn:  regexp.MustCompile(`(?i)\b(warn|warning)\b`),
			LogLevelDebug: regexp.MustCompile(`(?i)\b(debug|dbg|trace)\b`),
		},
		multilineStart: regexp.MustCompile(`(?i)^(\d{4}[-/]\d{2}[-/]\d{2}|[A-Z][a-z]{2}\s+\d{1,2}|\d{2}:\d{2}:\d{2}|Traceback|Exception|Error|Caused by|panic:)`),
		multilineCont:  regexp.MustCompile(`(?i)^\s+(at\s|in\s|File\s|from\s|\.\.\.|goroutine\s|\t)`),
	}

	// Auto-detect sources if not provided
	if len(le.sources) == 0 {
		le.sources = le.detectSources()
	}

	// Add custom files
	for _, path := range cfg.CustomFiles {
		le.sources = append(le.sources, LogSource{
			Type: "file",
			Path: path,
		})
	}

	// Initialize file tailers
	le.initFileTailers()

	return le, nil
}

// detectSources auto-detects available log sources
func (le *LogExporter) detectSources() []LogSource {
	var sources []LogSource

	// Standard log files
	standardPaths := []string{
		"/var/log/syslog",
		"/var/log/messages",
		"/var/log/auth.log",
		"/var/log/secure",
		"/var/log/nginx/error.log",
		"/var/log/nginx/access.log",
		"/var/log/apache2/error.log",
		"/var/log/httpd/error_log",
		"/var/log/postgresql/*.log",
		"/var/log/mysql/error.log",
		"/var/log/mongodb/mongod.log",
		"/var/log/redis/redis-server.log",
	}

	for _, pattern := range standardPaths {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				sources = append(sources, LogSource{
					Type:    "file",
					Path:    path,
					Service: le.detectServiceFromPath(path),
				})
			}
		}
	}

	// Journald (Linux only)
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/run/systemd/journal"); err == nil {
			sources = append(sources, LogSource{
				Type: "journald",
			})
		}
	}

	// Docker
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		sources = append(sources, LogSource{
			Type: "docker",
		})
	}

	return sources
}

// detectServiceFromPath tries to detect service name from log path
func (le *LogExporter) detectServiceFromPath(path string) string {
	pathLower := strings.ToLower(path)

	services := map[string]string{
		"nginx":      "nginx",
		"apache":     "apache",
		"httpd":      "apache",
		"postgresql": "postgres",
		"postgres":   "postgres",
		"mysql":      "mysql",
		"mariadb":    "mysql",
		"mongodb":    "mongodb",
		"mongod":     "mongodb",
		"redis":      "redis",
		"syslog":     "syslog",
		"auth":       "auth",
		"secure":     "auth",
	}

	for pattern, service := range services {
		if strings.Contains(pathLower, pattern) {
			return service
		}
	}

	// Use filename without extension
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// initFileTailers initializes file tailers for file sources
func (le *LogExporter) initFileTailers() {
	for _, source := range le.sources {
		if source.Type != "file" {
			continue
		}

		// Handle glob patterns
		matches, err := filepath.Glob(source.Path)
		if err != nil {
			matches = []string{source.Path}
		}

		for _, path := range matches {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}

			// Start from end of file
			le.fileTailers[path] = &FileTailer{
				Path:     path,
				Offset:   info.Size(),
				ModTime:  info.ModTime(),
				Service:  source.Service,
				LastRead: time.Now(),
			}
		}
	}
}

// Start begins log collection and export
func (le *LogExporter) Start() error {
	le.mu.Lock()
	defer le.mu.Unlock()

	if le.running {
		return nil
	}

	le.ctx, le.cancel = context.WithCancel(context.Background())
	le.running = true

	// Load buffered logs from disk
	le.loadDiskBuffer()

	// Start collection goroutine
	le.wg.Add(1)
	go le.collectionLoop()

	// Start flush goroutine
	le.wg.Add(1)
	go le.flushLoop()

	return nil
}

// Stop stops the log exporter
func (le *LogExporter) Stop() error {
	le.mu.Lock()
	if !le.running {
		le.mu.Unlock()
		return nil
	}
	le.running = false
	le.cancel()
	le.mu.Unlock()

	le.wg.Wait()

	// Flush remaining logs
	le.flush(true)

	// Save remaining buffer to disk
	le.saveDiskBuffer()

	return nil
}

// collectionLoop periodically collects logs from all sources
func (le *LogExporter) collectionLoop() {
	defer le.wg.Done()

	ticker := time.NewTicker(FileCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-le.ctx.Done():
			return
		case <-ticker.C:
			le.collectAllLogs()
		}
	}
}

// flushLoop periodically sends logs to backend
func (le *LogExporter) flushLoop() {
	defer le.wg.Done()

	ticker := time.NewTicker(LogCollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-le.ctx.Done():
			return
		case <-ticker.C:
			le.flush(false)
		}
	}
}

// collectAllLogs collects logs from all configured sources
func (le *LogExporter) collectAllLogs() {
	for _, source := range le.sources {
		switch source.Type {
		case "file":
			le.collectFileLogs(source)
		case "journald":
			le.collectJournaldLogs(source)
		case "docker":
			le.collectDockerLogs(source)
		}
	}

	le.stats.LastCollectTime = time.Now()
}

// collectFileLogs collects logs from file sources
func (le *LogExporter) collectFileLogs(source LogSource) {
	matches, err := filepath.Glob(source.Path)
	if err != nil {
		matches = []string{source.Path}
	}

	for _, path := range matches {
		tailer, ok := le.fileTailers[path]
		if !ok {
			// New file discovered
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			tailer = &FileTailer{
				Path:     path,
				Offset:   0, // Start from beginning for new files
				ModTime:  info.ModTime(),
				Service:  source.Service,
				LastRead: time.Now(),
			}
			le.fileTailers[path] = tailer
		}

		le.tailFile(tailer)
	}
}

// tailFile reads new lines from a file
func (le *LogExporter) tailFile(tailer *FileTailer) {
	file, err := os.Open(tailer.Path)
	if err != nil {
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return
	}

	// Check for file rotation (smaller size or different inode)
	if info.Size() < tailer.Offset {
		tailer.Offset = 0
	}

	// No new data
	if info.Size() == tailer.Offset {
		return
	}

	// Seek to last position
	_, err = file.Seek(tailer.Offset, 0)
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line

	var multilineBuffer strings.Builder
	var multilineStart time.Time

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Handle multiline logs (stack traces, etc.)
		if le.multilineStart.MatchString(line) {
			// Flush previous multiline
			if multilineBuffer.Len() > 0 {
				le.addLogEntry(multilineBuffer.String(), tailer.Service, "file", tailer.Path, multilineStart)
				multilineBuffer.Reset()
			}
			multilineBuffer.WriteString(line)
			multilineStart = le.parseTimestamp(line)
		} else if le.multilineCont.MatchString(line) && multilineBuffer.Len() > 0 {
			multilineBuffer.WriteString("\n")
			multilineBuffer.WriteString(line)
		} else {
			// Flush any pending multiline
			if multilineBuffer.Len() > 0 {
				le.addLogEntry(multilineBuffer.String(), tailer.Service, "file", tailer.Path, multilineStart)
				multilineBuffer.Reset()
			}
			le.addLogEntry(line, tailer.Service, "file", tailer.Path, time.Time{})
		}
	}

	// Flush remaining multiline
	if multilineBuffer.Len() > 0 {
		le.addLogEntry(multilineBuffer.String(), tailer.Service, "file", tailer.Path, multilineStart)
	}

	// Update offset
	newOffset, err := file.Seek(0, io.SeekCurrent)
	if err == nil {
		tailer.Offset = newOffset
	}
	tailer.LastRead = time.Now()
	tailer.ModTime = info.ModTime()
}

// collectJournaldLogs collects logs from systemd journal
func (le *LogExporter) collectJournaldLogs(source LogSource) {
	if runtime.GOOS != "linux" {
		return
	}

	ctx, cancel := context.WithTimeout(le.ctx, 10*time.Second)
	defer cancel()

	// Get logs since last collection
	since := "1 minute ago"
	if !le.stats.LastCollectTime.IsZero() {
		since = fmt.Sprintf("%d seconds ago", int(time.Since(le.stats.LastCollectTime).Seconds())+5)
	}

	args := []string{
		"--no-pager",
		"-o", "json",
		"--since", since,
		"-n", "1000",
	}

	if source.Path != "" {
		args = append(args, "-u", source.Path)
	}

	cmd := execCommandContext(ctx, "journalctl", args...)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var entry struct {
			Message           string `json:"MESSAGE"`
			Priority          string `json:"PRIORITY"`
			SyslogIdentifier  string `json:"SYSLOG_IDENTIFIER"`
			Unit              string `json:"_SYSTEMD_UNIT"`
			PID               string `json:"_PID"`
			RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"`
		}

		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		timestamp := time.Now()
		if ts, err := strconv.ParseInt(entry.RealtimeTimestamp, 10, 64); err == nil {
			timestamp = time.UnixMicro(ts)
		}

		pid := 0
		if p, err := strconv.Atoi(entry.PID); err == nil {
			pid = p
		}

		service := entry.SyslogIdentifier
		if service == "" {
			service = strings.TrimSuffix(entry.Unit, ".service")
		}

		logEntry := LogEntry{
			Timestamp:  timestamp,
			Level:      le.priorityToLevel(entry.Priority),
			Message:    entry.Message,
			Source:     "journald",
			SourcePath: entry.Unit,
			Service:    service,
			PID:        pid,
			Hostname:   le.hostname,
			AppName:    entry.SyslogIdentifier,
		}

		le.addEntry(logEntry)
	}
}

// collectDockerLogs collects logs from Docker containers
func (le *LogExporter) collectDockerLogs(source LogSource) {
	ctx, cancel := context.WithTimeout(le.ctx, 30*time.Second)
	defer cancel()

	// Get list of running containers
	cmd := execCommandContext(ctx, "docker", "ps", "--format", "{{.ID}}|{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "|")
		if len(parts) < 2 {
			continue
		}

		containerID := parts[0]
		containerName := parts[1]

		// Skip if specific container requested
		if source.ContainerID != "" && !strings.HasPrefix(containerID, source.ContainerID) {
			continue
		}

		le.collectContainerLogs(containerID, containerName)
	}
}

// collectContainerLogs collects logs from a specific container
func (le *LogExporter) collectContainerLogs(containerID, containerName string) {
	ctx, cancel := context.WithTimeout(le.ctx, 10*time.Second)
	defer cancel()

	// Get logs since last collection
	since := "1m"
	if !le.stats.LastCollectTime.IsZero() {
		since = fmt.Sprintf("%ds", int(time.Since(le.stats.LastCollectTime).Seconds())+5)
	}

	cmd := execCommandContext(ctx, "docker", "logs", "--timestamps", "--since", since, containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		timestamp, message := le.parseDockerTimestamp(line)

		logEntry := LogEntry{
			Timestamp:   timestamp,
			Level:       le.detectLevel(message),
			Message:     message,
			Source:      "docker",
			SourcePath:  containerName,
			Service:     le.extractServiceName(containerName),
			ContainerID: containerID[:12],
			Hostname:    le.hostname,
		}

		le.addEntry(logEntry)
	}
}

// addLogEntry creates and adds a log entry
func (le *LogExporter) addLogEntry(message, service, source, sourcePath string, timestamp time.Time) {
	if timestamp.IsZero() {
		timestamp = le.parseTimestamp(message)
		if timestamp.IsZero() {
			timestamp = time.Now()
		}
	}

	entry := LogEntry{
		Timestamp:  timestamp,
		Level:      le.detectLevel(message),
		Message:    truncateMessage(message, MaxLogMessageLength),
		Source:     source,
		SourcePath: sourcePath,
		Service:    service,
		Hostname:   le.hostname,
	}

	le.addEntry(entry)
}

// addEntry adds a log entry to the buffer with deduplication
func (le *LogExporter) addEntry(entry LogEntry) {
	// Generate hash for deduplication
	hash := le.hashEntry(entry)
	entry.MessageHash = hash

	le.bufferMu.Lock()
	defer le.bufferMu.Unlock()

	// Check for duplicate (same hash within 1 minute)
	if lastSeen, ok := le.seenHashes[hash]; ok {
		if time.Since(lastSeen) < time.Minute {
			return // Skip duplicate
		}
	}

	le.seenHashes[hash] = time.Now()
	le.buffer = append(le.buffer, entry)
	le.stats.LogsCollected++

	// Enforce max buffer size
	if len(le.buffer) > MaxBufferSize {
		// Drop oldest 10%
		dropCount := MaxBufferSize / 10
		le.buffer = le.buffer[dropCount:]
		le.stats.LogsDropped += int64(dropCount)
	}

	// Clean old hashes
	if len(le.seenHashes) > MaxBufferSize*2 {
		le.cleanSeenHashes()
	}
}

// hashEntry generates a hash for deduplication
func (le *LogExporter) hashEntry(entry LogEntry) string {
	data := fmt.Sprintf("%s|%s|%s|%s", entry.Source, entry.Service, entry.Level, entry.Message)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// cleanSeenHashes removes old entries from seen hashes
func (le *LogExporter) cleanSeenHashes() {
	cutoff := time.Now().Add(-5 * time.Minute)
	for hash, seen := range le.seenHashes {
		if seen.Before(cutoff) {
			delete(le.seenHashes, hash)
		}
	}
}

// flush sends buffered logs to backend
func (le *LogExporter) flush(force bool) {
	le.bufferMu.Lock()
	if len(le.buffer) == 0 {
		le.bufferMu.Unlock()
		return
	}

	// Take logs from buffer
	logs := make([]LogEntry, len(le.buffer))
	copy(logs, le.buffer)
	le.buffer = le.buffer[:0]
	le.bufferMu.Unlock()

	// Send in batches
	for i := 0; i < len(logs); i += MaxLogsPerBatch {
		end := i + MaxLogsPerBatch
		if end > len(logs) {
			end = len(logs)
		}
		batch := logs[i:end]

		if err := le.sendLogs(batch); err != nil {
			le.stats.Errors++
			// Put logs back to buffer or save to disk
			le.handleSendError(batch, force)
		} else {
			le.stats.LogsSent += int64(len(batch))
			le.stats.LastSendTime = time.Now()
		}
	}

	le.lastFlush = time.Now()
}

// sendLogs sends a batch of logs to the backend
func (le *LogExporter) sendLogs(logs []LogEntry) error {
	// Convert to OTLP format
	request := le.toOTLPRequest(logs)

	// Serialize to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	// Compress with gzip
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); err != nil {
		gz.Close()
		return fmt.Errorf("gzip error: %w", err)
	}
	gz.Close()

	// Send with retries
	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(RetryBaseDelay * time.Duration(1<<attempt))
		}

		ctx, cancel := context.WithTimeout(le.ctx, HTTPTimeout)
		req, err := http.NewRequestWithContext(ctx, "POST", le.endpoint, bytes.NewReader(buf.Bytes()))
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Authorization", "Bearer "+le.authToken)
		req.Header.Set("X-CatOps-Server-ID", le.serverID)

		resp, err := le.httpClient.Do(req)
		cancel()

		if err != nil {
			lastErr = err
			continue
		}

		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)

		// Don't retry on client errors (except 429)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			break
		}
	}

	return lastErr
}

// toOTLPRequest converts log entries to OTLP format
func (le *LogExporter) toOTLPRequest(logs []LogEntry) OTLPLogsRequest {
	records := make([]OTLPLogRecord, len(logs))

	for i, log := range logs {
		records[i] = OTLPLogRecord{
			TimeUnixNano:   log.Timestamp.UnixNano(),
			SeverityNumber: le.levelToSeverity(log.Level),
			SeverityText:   string(log.Level),
			Body:           OTLPAnyValue{StringValue: log.Message},
			Attributes: []OTLPKeyValue{
				// Standard OTLP semantic conventions
				{Key: "log.source", Value: OTLPAnyValue{StringValue: log.Source}},
				{Key: "log.file.path", Value: OTLPAnyValue{StringValue: log.SourcePath}},
				{Key: "service.name", Value: OTLPAnyValue{StringValue: log.Service}},
				{Key: "container.id", Value: OTLPAnyValue{StringValue: log.ContainerID}},
				{Key: "process.pid", Value: OTLPAnyValue{StringValue: strconv.Itoa(log.PID)}},
				{Key: "syslog.facility", Value: OTLPAnyValue{StringValue: log.Facility}},
				{Key: "syslog.appname", Value: OTLPAnyValue{StringValue: log.AppName}},
				{Key: "host.name", Value: OTLPAnyValue{StringValue: log.Hostname}},
				// CatOps specific
				{Key: "catops.message_hash", Value: OTLPAnyValue{StringValue: log.MessageHash}},
			},
		}
	}

	return OTLPLogsRequest{
		ResourceLogs: []OTLPResourceLogs{
			{
				Resource: OTLPResource{
					Attributes: []OTLPKeyValue{
						{Key: "service.name", Value: OTLPAnyValue{StringValue: "catops-cli"}},
						{Key: "host.name", Value: OTLPAnyValue{StringValue: le.hostname}},
						{Key: "catops.server.id", Value: OTLPAnyValue{StringValue: le.serverID}},
					},
				},
				ScopeLogs: []OTLPScopeLogs{
					{
						Scope: OTLPScope{
							Name:    "catops.io/log-exporter",
							Version: "1.0.0",
						},
						LogRecords: records,
					},
				},
			},
		},
	}
}

// handleSendError handles errors when sending logs
func (le *LogExporter) handleSendError(logs []LogEntry, force bool) {
	if force {
		// Save to disk buffer
		le.saveLogsToDisk(logs)
	} else {
		// Put back to memory buffer
		le.bufferMu.Lock()
		le.buffer = append(logs, le.buffer...)
		if len(le.buffer) > MaxBufferSize {
			le.buffer = le.buffer[:MaxBufferSize]
		}
		le.bufferMu.Unlock()
	}
}

// saveLogsToDisk saves logs to disk buffer
func (le *LogExporter) saveLogsToDisk(logs []LogEntry) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	bufferDir := filepath.Join(homeDir, DiskBufferDir)
	if err := os.MkdirAll(bufferDir, 0755); err != nil {
		return
	}

	filename := filepath.Join(bufferDir, fmt.Sprintf("logs_%d.json", time.Now().UnixNano()))

	data, err := json.Marshal(logs)
	if err != nil {
		return
	}

	os.WriteFile(filename, data, 0644)
	le.stats.LogsBuffered += int64(len(logs))
}

// loadDiskBuffer loads logs from disk buffer
func (le *LogExporter) loadDiskBuffer() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	bufferDir := filepath.Join(homeDir, DiskBufferDir)
	files, err := filepath.Glob(filepath.Join(bufferDir, "logs_*.json"))
	if err != nil {
		return
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var logs []LogEntry
		if err := json.Unmarshal(data, &logs); err != nil {
			continue
		}

		le.bufferMu.Lock()
		le.buffer = append(le.buffer, logs...)
		le.bufferMu.Unlock()

		os.Remove(file)
	}
}

// saveDiskBuffer saves remaining buffer to disk
func (le *LogExporter) saveDiskBuffer() {
	le.bufferMu.Lock()
	logs := le.buffer
	le.buffer = nil
	le.bufferMu.Unlock()

	if len(logs) > 0 {
		le.saveLogsToDisk(logs)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// detectLevel detects log level from message content
func (le *LogExporter) detectLevel(message string) LogLevel {
	for level, pattern := range le.levelPatterns {
		if pattern.MatchString(message) {
			return level
		}
	}
	return LogLevelInfo
}

// priorityToLevel converts syslog priority to log level
func (le *LogExporter) priorityToLevel(priority string) LogLevel {
	switch priority {
	case "0", "1", "2":
		return LogLevelFatal
	case "3":
		return LogLevelError
	case "4":
		return LogLevelWarn
	case "5", "6":
		return LogLevelInfo
	case "7":
		return LogLevelDebug
	default:
		return LogLevelInfo
	}
}

// levelToSeverity converts log level to OTLP severity number
func (le *LogExporter) levelToSeverity(level LogLevel) int {
	switch level {
	case LogLevelTrace:
		return 1
	case LogLevelDebug:
		return 5
	case LogLevelInfo:
		return 9
	case LogLevelWarn:
		return 13
	case LogLevelError:
		return 17
	case LogLevelFatal:
		return 21
	default:
		return 9
	}
}

// parseTimestamp tries to parse timestamp from log line
func (le *LogExporter) parseTimestamp(line string) time.Time {
	patterns := []string{
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"Jan 2 15:04:05",
		"Jan  2 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}

	// Try each pattern
	for _, pattern := range patterns {
		// Find potential timestamp in line (first 40 chars usually)
		searchLen := len(line)
		if searchLen > 40 {
			searchLen = 40
		}

		if t, err := time.Parse(pattern, line[:searchLen]); err == nil {
			// Fix year for syslog format
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			return t
		}
	}

	return time.Time{}
}

// parseDockerTimestamp parses Docker log timestamp
func (le *LogExporter) parseDockerTimestamp(line string) (time.Time, string) {
	// Docker format: 2024-01-15T10:30:45.123456789Z message
	if len(line) < 30 {
		return time.Now(), line
	}

	if t, err := time.Parse(time.RFC3339Nano, line[:30]); err == nil {
		return t, strings.TrimSpace(line[31:])
	}

	return time.Now(), line
}

// extractServiceName extracts service name from container name
func (le *LogExporter) extractServiceName(containerName string) string {
	// Remove common prefixes/suffixes
	name := containerName
	name = strings.TrimPrefix(name, "/")

	// Docker compose format: project_service_1
	parts := strings.Split(name, "_")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}

	// Kubernetes format: service-deployment-hash-hash
	parts = strings.Split(name, "-")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-2], "-")
	}

	return name
}

func truncateMessage(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// GetStats returns exporter statistics
func (le *LogExporter) GetStats() LogExporterStats {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.stats
}

// execCommandContext wraps os/exec.CommandContext
func execCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
