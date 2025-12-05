package metrics

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	maxLogLines    = 20  // Maximum log lines to collect per service
	logTimeout     = 5   // Timeout in seconds for log collection
	maxLogLineLen  = 500 // Maximum length per log line
)

// DockerContainer represents a running docker container
type DockerContainer struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Names   string // from docker ps
	Image   string `json:"Image"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Pid     int    // main process PID
	Compose string // docker-compose service name if applicable
}

// LogCollector collects logs from various sources
type LogCollector struct {
	// Patterns to filter interesting log lines (errors, warnings)
	errorPatterns []*regexp.Regexp
	// Cache of running docker containers (container name/id -> DockerContainer)
	dockerContainers map[string]DockerContainer
	dockerLoaded     bool
}

// NewLogCollector creates a new LogCollector
func NewLogCollector() *LogCollector {
	lc := &LogCollector{
		errorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\b(error|err|fail|failed|failure|exception|panic|fatal|critical)\b`),
			regexp.MustCompile(`(?i)\b(warn|warning)\b`),
			regexp.MustCompile(`(?i)(timeout|timed out|connection refused|connection reset)`),
			regexp.MustCompile(`(?i)(out of memory|oom|killed|segfault)`),
			regexp.MustCompile(`(?i)(denied|unauthorized|forbidden|permission)`),
		},
		dockerContainers: make(map[string]DockerContainer),
	}
	// Pre-load docker containers
	lc.loadDockerContainers()
	return lc
}

// loadDockerContainers loads all running docker containers
func (lc *LogCollector) loadDockerContainers() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(logTimeout)*time.Second)
	defer cancel()

	// Get all running containers with their PIDs
	// Format: ID|Names|Image|State|Pid
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.ID}}|{{.Names}}|{{.Image}}|{{.State}}")
	output, err := cmd.Output()
	if err != nil {
		// Docker not available or no containers
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		containerID := parts[0]
		containerName := parts[1]
		image := parts[2]
		state := parts[3]

		// Get container PID using docker inspect
		pid := lc.getContainerPID(containerID)

		// Check if it's a docker-compose service
		composeName := lc.getComposeServiceName(containerName)

		container := DockerContainer{
			ID:      containerID,
			Name:    containerName,
			Names:   containerName,
			Image:   image,
			State:   state,
			Pid:     pid,
			Compose: composeName,
		}

		// Index by ID, name, and PID for quick lookup
		lc.dockerContainers[containerID] = container
		lc.dockerContainers[containerName] = container
		if pid > 0 {
			lc.dockerContainers[fmt.Sprintf("pid:%d", pid)] = container
		}
	}

	lc.dockerLoaded = true
}

// getContainerPID gets the main PID of a container
func (lc *LogCollector) getContainerPID(containerID string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Pid}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(string(output)))
	return pid
}

// getComposeServiceName extracts docker-compose service name from container name
func (lc *LogCollector) getComposeServiceName(containerName string) string {
	// Docker-compose names containers as: project_service_1 or project-service-1
	parts := strings.Split(containerName, "_")
	if len(parts) >= 2 {
		return parts[len(parts)-2] // service name is second to last
	}
	parts = strings.Split(containerName, "-")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return ""
}

// findContainerForService tries to find a docker container for the service
func (lc *LogCollector) findContainerForService(service *ServiceInfo) *DockerContainer {
	// Try by container ID if we have it
	if service.ContainerID != "" {
		if c, ok := lc.dockerContainers[service.ContainerID]; ok {
			return &c
		}
	}

	// Try by PID
	if c, ok := lc.dockerContainers[fmt.Sprintf("pid:%d", service.PID)]; ok {
		return &c
	}

	// Try to match by service name in container name
	serviceLower := strings.ToLower(service.ServiceName)
	for _, container := range lc.dockerContainers {
		nameLower := strings.ToLower(container.Name)
		if strings.Contains(nameLower, serviceLower) ||
			strings.Contains(serviceLower, nameLower) {
			return &container
		}
	}

	return nil
}

// CollectServiceLogs collects logs for a service based on its type and container status
func (lc *LogCollector) CollectServiceLogs(service *ServiceInfo) ([]string, string) {
	// 1. Try to find docker container for this service
	container := lc.findContainerForService(service)
	if container != nil {
		logs, _ := lc.collectDockerLogs(container.ID)
		if len(logs) > 0 {
			// Update service with container info
			service.ContainerID = container.ID
			service.IsContainer = true
			return logs, "docker"
		}
	}

	// 2. If explicitly marked as container but no logs yet, try by container ID
	if service.IsContainer && service.ContainerID != "" {
		logs, err := lc.collectDockerLogs(service.ContainerID)
		if err == nil && len(logs) > 0 {
			return logs, "docker"
		}
	}

	// Skip journald on non-Linux systems (macOS, Windows don't have journalctl)
	if runtime.GOOS != "linux" {
		return nil, ""
	}

	// 3. Try journald for system services (nginx, redis, postgres, etc.)
	if lc.isSystemService(service.ServiceType) {
		logs, err := lc.collectJournaldLogs(service.ServiceType)
		if err == nil && len(logs) > 0 {
			return logs, "journald"
		}
	}

	// 4. Try journald by PID for any service
	logs, err := lc.collectJournaldByPID(service.PID)
	if err == nil && len(logs) > 0 {
		return logs, "journald"
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

// collectJournaldByPID collects recent logs from journald by process PID
func (lc *LogCollector) collectJournaldByPID(pid int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(logTimeout)*time.Second)
	defer cancel()

	// Get logs by PID - useful for apps that write to stdout/stderr
	cmd := exec.CommandContext(ctx, "journalctl", "_PID="+fmt.Sprintf("%d", pid), "-n", "100", "--no-pager", "-o", "short-iso")
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
