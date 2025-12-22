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

	// Strip Docker container prefix if present (e.g., "container_name  | ")
	originalLine := line
	line = p.stripDockerPrefix(line)
	if line != originalLine {
		entry.Attributes["docker_container"] = strings.TrimSpace(strings.Split(originalLine, "|")[0])
	}

	// Try JSON first (most structured)
	if p.tryParseJSON(line, entry) {
		return entry
	}

	// Try Docker daemon / logrus format (time="..." level="..." msg="...")
	if p.tryParseDockerLogrus(line, entry) {
		return entry
	}

	// Try Syslog RFC5424 format (before logfmt, as it contains key=value pairs)
	if p.tryParseSyslogRFC5424(line, entry) {
		return entry
	}

	// Try Gunicorn log format (before logfmt, as [timestamp] [pid] [level] pattern)
	if p.tryParseGunicornLog(line, entry) {
		return entry
	}

	// Try logfmt (key=value format)
	if p.tryParseLogfmt(line, entry) {
		return entry
	}

	// Try Uvicorn access log format (IP:PORT - "METHOD PATH HTTP/VERSION" STATUS)
	if p.tryParseUvicornAccess(line, entry) {
		return entry
	}

	// Try common log formats (Apache, Nginx)
	if p.tryParseCommonLog(line, entry) {
		return entry
	}

	// Try K8s glog format (I/W/E prefix)
	if p.tryParseKubernetesGlog(line, entry) {
		return entry
	}

	// Try Java Log4j/Logback/Spring format (before PostgreSQL/Python as they have similar timestamp format)
	if p.tryParseJavaLog(line, entry) {
		return entry
	}

	// Try Django request log format
	if p.tryParseDjangoLog(line, entry) {
		return entry
	}

	// Try Python standard logging format
	if p.tryParsePythonLog(line, entry) {
		return entry
	}

	// Try PostgreSQL log format
	if p.tryParsePostgresLog(line, entry) {
		return entry
	}

	// Try Python traceback (multiline starts with "Traceback")
	if p.tryParsePythonTraceback(line, entry) {
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

	// Common JSON log fields - timestamp
	if timestamp, ok := data["timestamp"].(string); ok {
		entry.Timestamp = p.parseTimestamp(timestamp)
	} else if timestamp, ok := data["time"].(string); ok {
		entry.Timestamp = p.parseTimestamp(timestamp)
	} else if ts, ok := data["ts"].(string); ok {
		entry.Timestamp = p.parseTimestamp(ts)
	} else if ts, ok := data["@timestamp"].(string); ok {
		entry.Timestamp = p.parseTimestamp(ts)
	}
	// MongoDB format: {"t":{"$date":"2025-01-15T10:30:00.123Z"}}
	if tObj, ok := data["t"].(map[string]interface{}); ok {
		if dateStr, ok := tObj["$date"].(string); ok {
			entry.Timestamp = p.parseTimestamp(dateStr)
		}
	}

	// Level - string format
	if level, ok := data["level"].(string); ok {
		entry.Level = p.normalizeLevel(level)
	} else if level, ok := data["severity"].(string); ok {
		entry.Level = p.normalizeLevel(level)
	} else if level, ok := data["lvl"].(string); ok {
		entry.Level = p.normalizeLevel(level)
	} else if level, ok := data["s"].(string); ok {
		// MongoDB format: s: "E" for error, "W" for warn, "I" for info
		entry.Level = p.normalizeLevel(level)
	}
	// Pino numeric level: 10=trace, 20=debug, 30=info, 40=warn, 50=error, 60=fatal
	if level, ok := data["level"].(float64); ok {
		entry.Level = p.pinoLevelToString(int(level))
	}

	// Message - multiple field names
	if msg, ok := data["message"].(string); ok {
		entry.Message = msg
	} else if msg, ok := data["msg"].(string); ok {
		entry.Message = msg
	} else if msg, ok := data["event"].(string); ok {
		// Python structlog uses "event"
		entry.Message = msg
	} else if msg, ok := data["log"].(string); ok {
		// Kubernetes pod logs
		entry.Message = strings.TrimSpace(msg)
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
		// Extract error type from stack (e.g., "Error: ECONNREFUSED\n at...")
		if entry.ErrorType == "" {
			if errMatch := regexp.MustCompile(`^(\w+Error|\w+Exception):`).FindStringSubmatch(stackTrace); len(errMatch) > 1 {
				entry.ErrorType = errMatch[1]
			} else if errMatch := regexp.MustCompile(`Error:\s*(\w+)`).FindStringSubmatch(stackTrace); len(errMatch) > 1 {
				entry.ErrorType = errMatch[1]
			}
		}
	} else if stackTrace, ok := data["stacktrace"].(string); ok {
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

// tryParseDockerLogrus attempts to parse Docker daemon / logrus format
// Example: time="2025-12-14T18:46:41.577890076Z" level=error msg="copy stream failed" error="reading from a closed fifo" stream=stderr spanID=abc traceID=xyz
func (p *LogParser) tryParseDockerLogrus(line string, entry *ParsedLogEntry) bool {
	// Must have time= or level= or msg= to be considered logrus format
	if !strings.Contains(line, "time=") && !strings.Contains(line, "level=") && !strings.Contains(line, "msg=") {
		return false
	}

	// Parse key=value or key="value" pairs
	pattern := regexp.MustCompile(`(\w+)=(\"([^\"]*)\"|([^\s]+))`)
	matches := pattern.FindAllStringSubmatch(line, -1)

	if len(matches) < 1 {
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

	// Extract timestamp
	if timestamp, ok := data["time"]; ok {
		// Try RFC3339 format first
		if t, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
			entry.Timestamp = t
		} else if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			entry.Timestamp = t
		}
	}

	// Extract level
	if level, ok := data["level"]; ok {
		entry.Level = strings.ToUpper(level)
	}

	// Extract message
	if msg, ok := data["msg"]; ok {
		entry.Message = msg
	}

	// Extract trace context
	if traceID, ok := data["traceID"]; ok {
		entry.TraceID = traceID
	}
	if spanID, ok := data["spanID"]; ok {
		entry.SpanID = spanID
	}

	// Extract error info
	if errorMsg, ok := data["error"]; ok {
		entry.ErrorType = errorMsg
	}

	// Extract HTTP fields from logrus attributes
	if method, ok := data["method"]; ok {
		entry.HTTPMethod = method
	}
	if path, ok := data["path"]; ok {
		entry.HTTPPath = path
	}
	if status, ok := data["status"]; ok {
		if statusInt, err := strconv.Atoi(status); err == nil {
			entry.HTTPStatus = statusInt
		}
	}

	// Store all fields in attributes
	for key, value := range data {
		// Skip fields we've already extracted
		if key != "time" && key != "level" && key != "msg" && key != "traceID" && key != "spanID" &&
			key != "method" && key != "path" && key != "status" && key != "error" {
			entry.Attributes[key] = value
		}
	}

	// If we found at least a message or level, consider it parsed
	return entry.Message != "" || entry.Level != ""
}

// tryParseLogfmt attempts to parse logfmt format (key=value key=value)
func (p *LogParser) tryParseLogfmt(line string, entry *ParsedLogEntry) bool {
	// Logfmt should start with key=value pattern (not have it embedded in message)
	// Check if line starts with word=something pattern
	if !regexp.MustCompile(`^\w+=`).MatchString(line) {
		// Also accept lines that start with timestamp followed by key=value
		// But NOT lines where key=value appears only in the middle (like Spring Boot logs)
		if !regexp.MustCompile(`^[\d\-T:.Z]+\s+\w+=`).MatchString(line) {
			return false
		}
	}

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

	// Extract error type from text
	p.extractErrorFromText(line, entry)

	// Extract user from text
	p.extractUserFromText(line, entry)

	// Detect level from keywords
	lineLower := strings.ToLower(line)
	switch {
	case strings.Contains(lineLower, "error") || strings.Contains(lineLower, " err ") || strings.Contains(lineLower, "[error]"):
		entry.Level = "ERROR"
	case strings.Contains(lineLower, "warn") || strings.Contains(lineLower, "warning") || strings.Contains(lineLower, "[warn]"):
		entry.Level = "WARN"
	case strings.Contains(lineLower, "fatal") || strings.Contains(lineLower, "panic") || strings.Contains(lineLower, "critical"):
		entry.Level = "FATAL"
	case strings.Contains(lineLower, "debug") || strings.Contains(lineLower, "[debug]"):
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

// parseTimestamp tries multiple timestamp formats
func (p *LogParser) parseTimestamp(s string) time.Time {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"02/Jan/2006:15:04:05 -0700",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// normalizeLevel converts various level formats to standard uppercase
func (p *LogParser) normalizeLevel(level string) string {
	level = strings.ToUpper(strings.TrimSpace(level))
	switch level {
	case "E", "ERR":
		return "ERROR"
	case "W", "WRN", "WARNING":
		return "WARN"
	case "I", "INF":
		return "INFO"
	case "D", "DBG":
		return "DEBUG"
	case "T", "TRC":
		return "TRACE"
	case "F", "FATAL", "CRITICAL", "CRIT":
		return "FATAL"
	default:
		return level
	}
}

// pinoLevelToString converts Pino numeric log levels to strings
func (p *LogParser) pinoLevelToString(level int) string {
	switch {
	case level <= 10:
		return "TRACE"
	case level <= 20:
		return "DEBUG"
	case level <= 30:
		return "INFO"
	case level <= 40:
		return "WARN"
	case level <= 50:
		return "ERROR"
	default:
		return "FATAL"
	}
}

// tryParseSyslogRFC5424 parses Syslog RFC5424 format
// Example: <134>1 2025-01-15T10:30:00.123456Z myserver myapp 1234 ID47 [exampleSDID@32473 iut="3"] Message
func (p *LogParser) tryParseSyslogRFC5424(line string, entry *ParsedLogEntry) bool {
	// Check if line starts with <priority>
	if !strings.HasPrefix(line, "<") {
		return false
	}

	// RFC5424: <priority>version timestamp hostname app-name procid msgid [structured-data] msg
	// More flexible pattern that handles nested brackets in structured data
	pattern := regexp.MustCompile(`^<(\d+)>(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.*)$`)
	matches := pattern.FindStringSubmatch(line)

	if len(matches) < 9 {
		return false
	}

	// Parse priority (facility * 8 + severity)
	if priority, err := strconv.Atoi(matches[1]); err == nil {
		severity := priority % 8
		switch severity {
		case 0, 1, 2:
			entry.Level = "FATAL"
		case 3:
			entry.Level = "ERROR"
		case 4:
			entry.Level = "WARN"
		case 5, 6:
			entry.Level = "INFO"
		case 7:
			entry.Level = "DEBUG"
		}
	}

	// Parse timestamp
	entry.Timestamp = p.parseTimestamp(matches[3])

	remainder := matches[8]

	// Extract structured data if present (starts with [)
	if strings.HasPrefix(remainder, "[") {
		// Find the end of structured data - look for ] followed by space or end
		bracketCount := 0
		sdEnd := -1
		for i, c := range remainder {
			if c == '[' {
				bracketCount++
			} else if c == ']' {
				bracketCount--
				if bracketCount == 0 {
					sdEnd = i
					break
				}
			}
		}

		if sdEnd > 0 {
			sdContent := remainder[1:sdEnd]
			// Extract key="value" pairs from structured data
			sdPattern := regexp.MustCompile(`(\w+)="([^"]*)"`)
			sdMatches := sdPattern.FindAllStringSubmatch(sdContent, -1)
			for _, m := range sdMatches {
				entry.Attributes[m[1]] = m[2]
			}
			// Message is after structured data
			if sdEnd+1 < len(remainder) {
				entry.Message = strings.TrimSpace(remainder[sdEnd+1:])
			}
		}
	} else if remainder != "-" {
		entry.Message = strings.TrimSpace(remainder)
	}

	return true
}

// tryParseKubernetesGlog parses Kubernetes glog format
// Example: W0115 10:30:00.123456       1 reflector.go:324] message
func (p *LogParser) tryParseKubernetesGlog(line string, entry *ParsedLogEntry) bool {
	// glog format: Lmmdd hh:mm:ss.uuuuuu threadid file:line] msg
	pattern := regexp.MustCompile(`^([IWEF])(\d{4})\s+(\d{2}:\d{2}:\d{2}\.\d+)\s+(\d+)\s+([^:]+):(\d+)\]\s+(.*)$`)
	matches := pattern.FindStringSubmatch(line)

	if len(matches) < 7 {
		return false
	}

	// Level from first character
	switch matches[1] {
	case "I":
		entry.Level = "INFO"
	case "W":
		entry.Level = "WARN"
	case "E":
		entry.Level = "ERROR"
	case "F":
		entry.Level = "FATAL"
	}

	entry.Message = matches[7]
	entry.Attributes["file"] = matches[5]
	entry.Attributes["line"] = matches[6]

	return true
}

// tryParseJavaLog parses Java Log4j/Logback/Spring Boot format
// Example: 2025-01-15 10:30:00.123 ERROR [main] c.e.a.MyService - Message
// Example: 2025-01-15 10:30:00.123  INFO 1234 --- [nio-8080-exec-1] c.e.app.Controller : Message
func (p *LogParser) tryParseJavaLog(line string, entry *ParsedLogEntry) bool {
	// Spring Boot format (note: multiple spaces between timestamp and level)
	// \s+ allows one or more spaces between timestamp and level
	springPattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\.\d+)\s+(\w+)\s+(\d+)\s+---\s+\[([^\]]+)\]\s+(\S+)\s+:\s+(.*)$`)
	if matches := springPattern.FindStringSubmatch(line); len(matches) > 6 {
		entry.Timestamp = p.parseTimestamp(matches[1])
		entry.Level = p.normalizeLevel(matches[2])
		entry.Attributes["pid"] = matches[3]
		entry.Attributes["thread"] = strings.TrimSpace(matches[4])
		entry.Attributes["logger"] = matches[5]
		entry.Message = matches[6]

		// Extract user_id if present in message
		if userMatch := regexp.MustCompile(`(?:user_?id|userId)[=:]\s*(\S+)`).FindStringSubmatch(entry.Message); len(userMatch) > 1 {
			entry.UserID = userMatch[1]
		}
		// Also check for id= pattern like "id=123"
		if idMatch := regexp.MustCompile(`\bid=(\d+)`).FindStringSubmatch(entry.Message); len(idMatch) > 1 {
			entry.UserID = idMatch[1]
		}

		return true
	}

	// Log4j/Logback format: 2025-01-15 10:30:00.123 ERROR [main] c.e.a.MyService - Message
	log4jPattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}[.,]\d+)\s+(\w+)\s+\[([^\]]+)\]\s+(\S+)\s+-\s+(.*)$`)
	if matches := log4jPattern.FindStringSubmatch(line); len(matches) > 5 {
		entry.Timestamp = p.parseTimestamp(strings.Replace(matches[1], ",", ".", 1))
		entry.Level = p.normalizeLevel(matches[2])
		entry.Attributes["thread"] = matches[3]
		entry.Attributes["logger"] = matches[4]
		entry.Message = matches[5]

		// Extract exception type from message (with or without colon)
		if errMatch := regexp.MustCompile(`(\w+Exception|\w+Error)(?::|$)`).FindStringSubmatch(entry.Message); len(errMatch) > 1 {
			entry.ErrorType = errMatch[1]
		}

		return true
	}

	// Java stack trace line starting with "at " (typically after exception)
	if strings.HasPrefix(strings.TrimSpace(line), "at ") {
		entry.Level = "ERROR"
		entry.StackTrace = line
		entry.Message = line
		return true
	}

	// Java stack trace (multiline or single line starting with exception class)
	// First line contains exception, rest are "at ..." lines
	firstLine := strings.Split(line, "\n")[0]
	if strings.HasPrefix(firstLine, "java.") || strings.HasPrefix(firstLine, "javax.") ||
		strings.HasPrefix(firstLine, "org.") || strings.HasPrefix(firstLine, "com.") {
		// Extract exception type: java.lang.NullPointerException: message
		if errMatch := regexp.MustCompile(`^([\w.]+(?:Exception|Error)):\s*(.*)$`).FindStringSubmatch(firstLine); len(errMatch) > 2 {
			entry.Level = "ERROR"
			entry.ErrorType = errMatch[1]
			entry.Message = errMatch[2]
			entry.StackTrace = line
			return true
		}
		// Exception without message: java.lang.NullPointerException
		if errMatch := regexp.MustCompile(`^([\w.]+(?:Exception|Error))$`).FindStringSubmatch(strings.TrimSpace(firstLine)); len(errMatch) > 1 {
			entry.Level = "ERROR"
			entry.ErrorType = errMatch[1]
			entry.Message = errMatch[1]
			entry.StackTrace = line
			return true
		}
	}

	return false
}

// tryParseDjangoLog parses Django request log format
// Example: [15/Jan/2025 10:30:00] "GET /admin/ HTTP/1.1" 200 12345
func (p *LogParser) tryParseDjangoLog(line string, entry *ParsedLogEntry) bool {
	pattern := regexp.MustCompile(`^\[(\d{2}/\w{3}/\d{4}\s+\d{2}:\d{2}:\d{2})\]\s+"(\w+)\s+([^\s]+)\s+HTTP/[\d.]+"\s+(\d{3})\s+(\d+)`)
	matches := pattern.FindStringSubmatch(line)

	if len(matches) < 5 {
		return false
	}

	// Parse timestamp
	if t, err := time.Parse("02/Jan/2006 15:04:05", matches[1]); err == nil {
		entry.Timestamp = t
	}

	entry.HTTPMethod = matches[2]
	entry.HTTPPath = matches[3]
	if status, err := strconv.Atoi(matches[4]); err == nil {
		entry.HTTPStatus = status
	}

	// Determine level from status code
	statusCode := entry.HTTPStatus
	switch {
	case statusCode >= 500:
		entry.Level = "ERROR"
	case statusCode >= 400:
		entry.Level = "WARN"
	default:
		entry.Level = "INFO"
	}

	entry.Message = line

	return true
}

// tryParsePostgresLog parses PostgreSQL log format
// Example: 2025-01-15 10:30:00.123 UTC [1234] ERROR:  duplicate key value violates unique constraint
func (p *LogParser) tryParsePostgresLog(line string, entry *ParsedLogEntry) bool {
	pattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\.\d+)\s+(\w+)\s+\[(\d+)\]\s+(\w+):\s+(.*)$`)
	matches := pattern.FindStringSubmatch(line)

	if len(matches) < 6 {
		return false
	}

	entry.Timestamp = p.parseTimestamp(matches[1])
	entry.Attributes["timezone"] = matches[2]
	entry.Attributes["pid"] = matches[3]
	entry.Level = p.normalizeLevel(matches[4])
	entry.Message = matches[5]

	// Extract error type from PostgreSQL errors
	if entry.Level == "ERROR" {
		// Look for constraint names, error codes
		if errMatch := regexp.MustCompile(`(duplicate key|foreign key|null value|syntax error|permission denied)`).FindStringSubmatch(strings.ToLower(entry.Message)); len(errMatch) > 0 {
			entry.ErrorType = errMatch[1]
		}
	}

	return true
}

// tryParsePythonLog parses Python standard logging format
// Example: 2025-01-15 10:30:00,123 - myapp.module - ERROR - Message
func (p *LogParser) tryParsePythonLog(line string, entry *ParsedLogEntry) bool {
	pattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}[,.]?\d*)\s+-\s+(\S+)\s+-\s+(\w+)\s+-\s+(.*)$`)
	matches := pattern.FindStringSubmatch(line)

	if len(matches) < 5 {
		return false
	}

	entry.Timestamp = p.parseTimestamp(strings.Replace(matches[1], ",", ".", 1))
	entry.Attributes["logger"] = matches[2]
	entry.Level = p.normalizeLevel(matches[3])
	entry.Message = matches[4]

	// Extract exception type if present in message
	if errMatch := regexp.MustCompile(`(\w+Error|\w+Exception):\s*`).FindStringSubmatch(entry.Message); len(errMatch) > 1 {
		entry.ErrorType = errMatch[1]
	}

	return true
}

// tryParsePythonTraceback parses Python traceback format
// Example: Traceback (most recent call last):\n  File "/app/main.py"...
func (p *LogParser) tryParsePythonTraceback(line string, entry *ParsedLogEntry) bool {
	// Check if this is a Python traceback or exception
	if strings.HasPrefix(line, "Traceback (most recent call last):") {
		entry.Level = "ERROR"
		entry.StackTrace = line
		entry.Message = "Python traceback"

		// Try to extract the exception type from the end of traceback
		lines := strings.Split(line, "\n")
		if len(lines) > 0 {
			lastLine := lines[len(lines)-1]
			if errMatch := regexp.MustCompile(`^(\w+(?:Error|Exception)):\s*(.*)$`).FindStringSubmatch(lastLine); len(errMatch) > 1 {
				entry.ErrorType = errMatch[1]
				if len(errMatch) > 2 {
					entry.Message = errMatch[2]
				}
			}
		}
		return true
	}

	// Also handle standalone exception lines like "ConnectionError: Failed to connect"
	if errMatch := regexp.MustCompile(`^(\w+(?:Error|Exception)):\s*(.*)$`).FindStringSubmatch(line); len(errMatch) > 2 {
		entry.Level = "ERROR"
		entry.ErrorType = errMatch[1]
		entry.Message = errMatch[2]
		return true
	}

	return false
}

// extractErrorFromText tries to extract error information from plain text
func (p *LogParser) extractErrorFromText(line string, entry *ParsedLogEntry) {
	// Python/Java exception patterns
	errPatterns := []string{
		`(\w+Error):\s*(.*)`,
		`(\w+Exception):\s*(.*)`,
		`(\w+Fault):\s*(.*)`,
	}

	for _, pattern := range errPatterns {
		if matches := regexp.MustCompile(pattern).FindStringSubmatch(line); len(matches) > 1 {
			entry.ErrorType = matches[1]
			if len(matches) > 2 && entry.Message == "" {
				entry.Message = matches[2]
			}
			break
		}
	}
}

// extractUserFromText tries to extract user information from plain text
func (p *LogParser) extractUserFromText(line string, entry *ParsedLogEntry) {
	patterns := []string{
		`(?i)user[=:\s]+['"]?(\w+)['"]?`,
		`(?i)username[=:\s]+['"]?(\w+)['"]?`,
		`(?i)user_?id[=:\s]+['"]?(\w+)['"]?`,
		`(?i)for user ['"]?(\w+)['"]?`,
	}

	for _, pattern := range patterns {
		if matches := regexp.MustCompile(pattern).FindStringSubmatch(line); len(matches) > 1 {
			entry.UserID = matches[1]
			break
		}
	}
}

// stripDockerPrefix removes Docker Compose log prefix (e.g., "container_name  | ")
func (p *LogParser) stripDockerPrefix(line string) string {
	// Docker compose format: "container_name  | actual log content"
	// The container name is followed by spaces and a pipe character
	if idx := strings.Index(line, " | "); idx > 0 && idx < 50 {
		// Verify it looks like a container name (no spaces before the separator)
		prefix := line[:idx]
		if !strings.Contains(strings.TrimSpace(prefix), " ") {
			return strings.TrimSpace(line[idx+3:])
		}
	}
	return line
}

// tryParseUvicornAccess parses Uvicorn access log format
// Example: 172.19.0.1:35730 - "GET /rates/?currency_from=RUB&currency_to=USDT HTTP/1.0" 200
func (p *LogParser) tryParseUvicornAccess(line string, entry *ParsedLogEntry) bool {
	// Uvicorn format: IP:PORT - "METHOD PATH HTTP/VERSION" STATUS
	pattern := regexp.MustCompile(`^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})(?::\d+)?\s+-\s+"(\w+)\s+([^\s]+)\s+HTTP/[\d.]+"\s+(\d{3})`)
	matches := pattern.FindStringSubmatch(line)

	if len(matches) < 5 {
		return false
	}

	entry.SourceIP = matches[1]
	entry.HTTPMethod = matches[2]
	entry.HTTPPath = matches[3]
	if status, err := strconv.Atoi(matches[4]); err == nil {
		entry.HTTPStatus = status
	}

	// Set level based on status code
	switch {
	case entry.HTTPStatus >= 500:
		entry.Level = "ERROR"
	case entry.HTTPStatus >= 400:
		entry.Level = "WARN"
	default:
		entry.Level = "INFO"
	}

	entry.Message = line

	return true
}

// tryParseGunicornLog parses Gunicorn log format
// Example: [2025-12-22 18:02:09 +0000] [10] [WARNING] Invalid HTTP request received.
func (p *LogParser) tryParseGunicornLog(line string, entry *ParsedLogEntry) bool {
	// Gunicorn format: [TIMESTAMP] [PID] [LEVEL] MESSAGE
	pattern := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})\s+([+-]\d{4})\]\s+\[(\d+)\]\s+\[(\w+)\]\s+(.*)$`)
	matches := pattern.FindStringSubmatch(line)

	if len(matches) < 6 {
		return false
	}

	// Parse timestamp with timezone
	timestampStr := matches[1] + " " + matches[2]
	if t, err := time.Parse("2006-01-02 15:04:05 -0700", timestampStr); err == nil {
		entry.Timestamp = t
	}

	entry.Attributes["pid"] = matches[3]
	entry.Level = p.normalizeLevel(matches[4])
	entry.Message = matches[5]

	return true
}
