package metrics

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedLogEntry represents a structured log entry with extracted fields
type ParsedLogEntry struct {
	Raw       string    `json:"raw"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`

	// Trace/Request context
	TraceID   string `json:"trace_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`

	// HTTP fields
	HTTPMethod   string `json:"http_method,omitempty"`
	HTTPPath     string `json:"http_path,omitempty"`
	HTTPStatus   int    `json:"http_status,omitempty"`
	HTTPDuration int64  `json:"http_duration_ms,omitempty"`

	// Error fields
	ErrorType  string `json:"error_type,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`

	// Context
	SourceIP  string `json:"source_ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`

	// Custom fields
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// LogParser handles parsing of different log formats
type LogParser struct {
	// Regex patterns for common fields
	traceIDPattern   *regexp.Regexp
	requestIDPattern *regexp.Regexp
	httpPattern      *regexp.Regexp
	ipPattern        *regexp.Regexp
	stackTraceStart  *regexp.Regexp
}

// NewLogParser creates a new log parser with initialized patterns
func NewLogParser() *LogParser {
	return &LogParser{
		traceIDPattern:   regexp.MustCompile(`(?i)trace[-_]?id[=:\s]+([a-f0-9-]+)`),
		requestIDPattern: regexp.MustCompile(`(?i)request[-_]?id[=:\s]+([a-f0-9-]+)`),
		httpPattern:      regexp.MustCompile(`(?i)(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+([^\s]+)\s+(?:HTTP/[\d.]+\s+)?(\d{3})?`),
		ipPattern:        regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`),
		stackTraceStart:  regexp.MustCompile(`(?i)(traceback|stack trace|at\s+\w+\.\w+|^\s+at\s+)`),
	}
}

// ParseLogLine attempts to parse a log line using various formats
func (p *LogParser) ParseLogLine(line string) *ParsedLogEntry {
	entry := &ParsedLogEntry{
		Raw:        line,
		Attributes: make(map[string]interface{}),
	}

	// Try JSON first (most structured)
	if p.tryParseJSON(line, entry) {
		return entry
	}

	// Try logfmt (key=value format)
	if p.tryParseLogfmt(line, entry) {
		return entry
	}

	// Try common log formats (Apache, Nginx)
	if p.tryParseCommonLog(line, entry) {
		return entry
	}

	// Fallback: extract what we can from plain text
	p.extractCommonFields(line, entry)
	entry.Message = line

	return entry
}

// tryParseJSON attempts to parse JSON log format
func (p *LogParser) tryParseJSON(line string, entry *ParsedLogEntry) bool {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return false
	}

	// Common JSON log fields
	if timestamp, ok := data["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			entry.Timestamp = t
		}
	} else if timestamp, ok := data["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			entry.Timestamp = t
		}
	}

	// Level
	if level, ok := data["level"].(string); ok {
		entry.Level = strings.ToUpper(level)
	} else if level, ok := data["severity"].(string); ok {
		entry.Level = strings.ToUpper(level)
	}

	// Message
	if msg, ok := data["message"].(string); ok {
		entry.Message = msg
	} else if msg, ok := data["msg"].(string); ok {
		entry.Message = msg
	}

	// Trace context
	if traceID, ok := data["trace_id"].(string); ok {
		entry.TraceID = traceID
	} else if traceID, ok := data["traceId"].(string); ok {
		entry.TraceID = traceID
	}

	if spanID, ok := data["span_id"].(string); ok {
		entry.SpanID = spanID
	} else if spanID, ok := data["spanId"].(string); ok {
		entry.SpanID = spanID
	}

	if requestID, ok := data["request_id"].(string); ok {
		entry.RequestID = requestID
	} else if requestID, ok := data["requestId"].(string); ok {
		entry.RequestID = requestID
	}

	if userID, ok := data["user_id"].(string); ok {
		entry.UserID = userID
	} else if userID, ok := data["userId"].(string); ok {
		entry.UserID = userID
	}

	// HTTP fields
	if method, ok := data["http_method"].(string); ok {
		entry.HTTPMethod = method
	} else if method, ok := data["method"].(string); ok {
		entry.HTTPMethod = method
	}

	if path, ok := data["http_path"].(string); ok {
		entry.HTTPPath = path
	} else if path, ok := data["path"].(string); ok {
		entry.HTTPPath = path
	} else if path, ok := data["url"].(string); ok {
		entry.HTTPPath = path
	}

	if status, ok := data["http_status"].(float64); ok {
		entry.HTTPStatus = int(status)
	} else if status, ok := data["status"].(float64); ok {
		entry.HTTPStatus = int(status)
	} else if status, ok := data["status_code"].(float64); ok {
		entry.HTTPStatus = int(status)
	}

	if duration, ok := data["duration"].(float64); ok {
		entry.HTTPDuration = int64(duration)
	} else if duration, ok := data["duration_ms"].(float64); ok {
		entry.HTTPDuration = int64(duration)
	}

	// Error fields
	if errorType, ok := data["error_type"].(string); ok {
		entry.ErrorType = errorType
	} else if errorType, ok := data["error"].(string); ok {
		entry.ErrorType = errorType
	}

	if stackTrace, ok := data["stack_trace"].(string); ok {
		entry.StackTrace = stackTrace
	} else if stackTrace, ok := data["stack"].(string); ok {
		entry.StackTrace = stackTrace
	}

	// IP and User Agent
	if ip, ok := data["ip"].(string); ok {
		entry.SourceIP = ip
	} else if ip, ok := data["source_ip"].(string); ok {
		entry.SourceIP = ip
	} else if ip, ok := data["remote_addr"].(string); ok {
		entry.SourceIP = ip
	}

	if ua, ok := data["user_agent"].(string); ok {
		entry.UserAgent = ua
	}

	// Store remaining fields in attributes
	excludeKeys := map[string]bool{
		"timestamp": true, "time": true, "level": true, "severity": true,
		"message": true, "msg": true, "trace_id": true, "span_id": true,
		"request_id": true, "user_id": true, "http_method": true, "method": true,
		"http_path": true, "path": true, "url": true, "http_status": true,
		"status": true, "status_code": true, "duration": true, "duration_ms": true,
		"error_type": true, "error": true, "stack_trace": true, "stack": true,
		"ip": true, "source_ip": true, "remote_addr": true, "user_agent": true,
	}

	for key, value := range data {
		if !excludeKeys[key] {
			entry.Attributes[key] = value
		}
	}

	return true
}

// tryParseLogfmt attempts to parse logfmt format (key=value key=value)
func (p *LogParser) tryParseLogfmt(line string, entry *ParsedLogEntry) bool {
	// Simple logfmt parsing: key=value or key="value with spaces"
	pattern := regexp.MustCompile(`(\w+)=("([^"]*)"|([^\s]+))`)
	matches := pattern.FindAllStringSubmatch(line, -1)

	if len(matches) < 2 {
		return false
	}

	data := make(map[string]string)
	for _, match := range matches {
		key := match[1]
		value := match[3] // quoted value
		if value == "" {
			value = match[4] // unquoted value
		}
		data[key] = value
	}

	// Extract common fields
	if timestamp, ok := data["timestamp"]; ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			entry.Timestamp = t
		}
	} else if timestamp, ok := data["time"]; ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			entry.Timestamp = t
		}
	}

	if level, ok := data["level"]; ok {
		entry.Level = strings.ToUpper(level)
	}

	if msg, ok := data["message"]; ok {
		entry.Message = msg
	} else if msg, ok := data["msg"]; ok {
		entry.Message = msg
	}

	if traceID, ok := data["trace_id"]; ok {
		entry.TraceID = traceID
	}

	if requestID, ok := data["request_id"]; ok {
		entry.RequestID = requestID
	}

	if method, ok := data["method"]; ok {
		entry.HTTPMethod = method
	}

	if path, ok := data["path"]; ok {
		entry.HTTPPath = path
	}

	// Store in attributes
	for key, value := range data {
		if _, exists := entry.Attributes[key]; !exists {
			entry.Attributes[key] = value
		}
	}

	return len(data) > 0
}

// tryParseCommonLog attempts to parse Apache/Nginx common log format
func (p *LogParser) tryParseCommonLog(line string, entry *ParsedLogEntry) bool {
	// Common log format: 127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /index.html HTTP/1.0" 200 2326
	pattern := regexp.MustCompile(`^(\S+)\s+\S+\s+\S+\s+\[([^\]]+)\]\s+"(\S+)\s+(\S+)\s+(\S+)"\s+(\d{3})\s+(\d+)`)
	matches := pattern.FindStringSubmatch(line)

	if len(matches) < 7 {
		return false
	}

	entry.SourceIP = matches[1]
	entry.HTTPMethod = matches[3]
	entry.HTTPPath = matches[4]

	if len(matches) > 6 && matches[6] != "" {
		if statusInt, err := strconv.Atoi(matches[6]); err == nil {
			entry.HTTPStatus = statusInt
		}
	}

	entry.Message = line

	return true
}

// extractCommonFields extracts common fields from unstructured text
func (p *LogParser) extractCommonFields(line string, entry *ParsedLogEntry) {
	// Extract trace ID
	if matches := p.traceIDPattern.FindStringSubmatch(line); len(matches) > 1 {
		entry.TraceID = matches[1]
	}

	// Extract request ID
	if matches := p.requestIDPattern.FindStringSubmatch(line); len(matches) > 1 {
		entry.RequestID = matches[1]
	}

	// Extract HTTP info
	if matches := p.httpPattern.FindStringSubmatch(line); len(matches) > 3 {
		entry.HTTPMethod = matches[1]
		entry.HTTPPath = matches[2]
		if matches[3] != "" {
			if status, err := strconv.Atoi(matches[3]); err == nil {
				entry.HTTPStatus = status
			}
		}
	}

	// Extract IP
	if matches := p.ipPattern.FindStringSubmatch(line); len(matches) > 1 {
		entry.SourceIP = matches[1]
	}

	// Detect level from keywords
	lineLower := strings.ToLower(line)
	switch {
	case strings.Contains(lineLower, "error") || strings.Contains(lineLower, "err"):
		entry.Level = "ERROR"
	case strings.Contains(lineLower, "warn") || strings.Contains(lineLower, "warning"):
		entry.Level = "WARN"
	case strings.Contains(lineLower, "fatal") || strings.Contains(lineLower, "panic"):
		entry.Level = "FATAL"
	case strings.Contains(lineLower, "debug"):
		entry.Level = "DEBUG"
	default:
		entry.Level = "INFO"
	}
}

// ExtractStackTrace extracts multi-line stack traces from logs
func (p *LogParser) ExtractStackTrace(lines []string, startIndex int) (stackTrace string, endIndex int) {
	if startIndex >= len(lines) {
		return "", startIndex
	}

	// Check if this line starts a stack trace
	if !p.stackTraceStart.MatchString(lines[startIndex]) {
		return "", startIndex
	}

	var traceLines []string
	i := startIndex

	// Collect stack trace lines (usually indented or start with "at")
	for i < len(lines) {
		line := lines[i]

		// Stack trace lines are usually indented or start with specific patterns
		if strings.HasPrefix(strings.TrimSpace(line), "at ") ||
			strings.HasPrefix(line, "  ") ||
			strings.HasPrefix(line, "\t") ||
			regexp.MustCompile(`^\s+File "`).MatchString(line) ||
			regexp.MustCompile(`^\s+line \d+`).MatchString(line) {
			traceLines = append(traceLines, line)
			i++
		} else if len(traceLines) > 0 {
			// End of stack trace
			break
		} else {
			i++
		}

		// Limit stack trace to reasonable size
		if len(traceLines) > 50 {
			break
		}
	}

	if len(traceLines) > 0 {
		return strings.Join(traceLines, "\n"), i
	}

	return "", startIndex
}
