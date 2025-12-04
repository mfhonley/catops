package metrics

import (
	"bufio"
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	maxLogLines    = 20  // Maximum log lines to collect per service
	logTimeout     = 5   // Timeout in seconds for log collection
	maxLogLineLen  = 500 // Maximum length per log line
)

// LogCollector collects logs from various sources
type LogCollector struct {
	// Patterns to filter interesting log lines (errors, warnings)
	errorPatterns []*regexp.Regexp
}

// NewLogCollector creates a new LogCollector
func NewLogCollector() *LogCollector {
	return &LogCollector{
		errorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\b(error|err|fail|failed|failure|exception|panic|fatal|critical)\b`),
			regexp.MustCompile(`(?i)\b(warn|warning)\b`),
			regexp.MustCompile(`(?i)(timeout|timed out|connection refused|connection reset)`),
			regexp.MustCompile(`(?i)(out of memory|oom|killed|segfault)`),
			regexp.MustCompile(`(?i)(denied|unauthorized|forbidden|permission)`),
		},
	}
}

// CollectServiceLogs collects logs for a service based on its type and container status
func (lc *LogCollector) CollectServiceLogs(service *ServiceInfo) ([]string, string) {
	// If running in Docker container, use docker logs
	if service.IsContainer && service.ContainerID != "" {
		logs, err := lc.collectDockerLogs(service.ContainerID)
		if err == nil && len(logs) > 0 {
			return logs, "docker"
		}
	}

	// Try journald for system services
	if lc.isSystemService(service.ServiceType) {
		logs, err := lc.collectJournaldLogs(service.ServiceType)
		if err == nil && len(logs) > 0 {
			return logs, "journald"
		}
	}

	return nil, ""
}

// collectDockerLogs collects recent error logs from a Docker container
func (lc *LogCollector) collectDockerLogs(containerID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(logTimeout)*time.Second)
	defer cancel()

	// Get last 100 lines and filter for errors
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", "100", "--timestamps", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return lc.filterLogLines(string(output)), nil
}

// collectJournaldLogs collects recent error logs from journald
func (lc *LogCollector) collectJournaldLogs(serviceType ServiceType) ([]string, error) {
	unitName := lc.getSystemdUnitName(serviceType)
	if unitName == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(logTimeout)*time.Second)
	defer cancel()

	// Get last 100 lines from journald
	cmd := exec.CommandContext(ctx, "journalctl", "-u", unitName, "-n", "100", "--no-pager", "-o", "short-iso")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return lc.filterLogLines(string(output)), nil
}

// filterLogLines filters log output to only include error/warning lines
func (lc *LogCollector) filterLogLines(output string) []string {
	var filtered []string
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if lc.isInterestingLine(line) {
			// Truncate long lines
			if len(line) > maxLogLineLen {
				line = line[:maxLogLineLen-3] + "..."
			}
			filtered = append(filtered, line)
		}
	}

	// Keep only the last N lines
	if len(filtered) > maxLogLines {
		filtered = filtered[len(filtered)-maxLogLines:]
	}

	return filtered
}

// isInterestingLine checks if a log line contains error/warning patterns
func (lc *LogCollector) isInterestingLine(line string) bool {
	for _, pattern := range lc.errorPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

// isSystemService checks if the service type is typically managed by systemd
func (lc *LogCollector) isSystemService(serviceType ServiceType) bool {
	switch serviceType {
	case ServiceTypeNginx, ServiceTypeApache, ServiceTypeRedis,
		ServiceTypePostgres, ServiceTypeMySQL, ServiceTypeMongoDB:
		return true
	default:
		return false
	}
}

// getSystemdUnitName returns the systemd unit name for a service type
func (lc *LogCollector) getSystemdUnitName(serviceType ServiceType) string {
	switch serviceType {
	case ServiceTypeNginx:
		return "nginx"
	case ServiceTypeApache:
		return "apache2" // or httpd on RHEL
	case ServiceTypeRedis:
		return "redis-server" // or redis on some systems
	case ServiceTypePostgres:
		return "postgresql"
	case ServiceTypeMySQL:
		return "mysql" // or mariadb
	case ServiceTypeMongoDB:
		return "mongod"
	default:
		return ""
	}
}

// GetAllServiceLogs collects logs for all detected services
func (lc *LogCollector) GetAllServiceLogs(services []ServiceInfo) []ServiceInfo {
	for i := range services {
		logs, source := lc.CollectServiceLogs(&services[i])
		services[i].RecentLogs = logs
		services[i].LogSource = source
	}
	return services
}
