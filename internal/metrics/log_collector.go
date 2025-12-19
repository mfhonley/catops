package metrics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	maxLogLines    = 100  // Maximum log lines to collect per service (increased for AI analysis)
	logTimeout     = 10   // Timeout in seconds for log collection (increased for thorough collection)
	maxLogLineLen  = 2000 // Maximum length per log line (increased to capture full stack traces)
	errorLogLines  = 50   // Priority lines for errors/warnings
	normalLogLines = 50   // Remaining lines for normal logs
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

	// 3. Try pm2 logs for Node.js apps
	if service.ServiceType == ServiceTypeNodeApp {
		logs := lc.collectPM2Logs(service.PID)
		if len(logs) > 0 {
			return logs, "pm2"
		}
	}

	// Skip journald on non-Linux systems (macOS, Windows don't have journalctl)
	if runtime.GOOS != "linux" {
		return nil, ""
	}

	// 4. Try journald for system services (nginx, redis, postgres, etc.)
	if lc.isSystemService(service.ServiceType) {
		logs, err := lc.collectJournaldLogs(service.ServiceType)
		if err == nil && len(logs) > 0 {
			return logs, "journald"
		}
	}

	// 5. Try journald by PID for any service
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

	// Get last N lines (increased for better AI analysis)
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", maxLogLines*2), "--timestamps", containerID)
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

	// Get last N lines from journald (increased for better AI analysis)
	cmd := exec.CommandContext(ctx, "journalctl", "-u", unitName, "-n", fmt.Sprintf("%d", maxLogLines*2), "--no-pager", "-o", "short-iso")
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

	// Get logs by PID - useful for apps that write to stdout/stderr (increased for better AI analysis)
	cmd := exec.CommandContext(ctx, "journalctl", "_PID="+fmt.Sprintf("%d", pid), "-n", fmt.Sprintf("%d", maxLogLines*2), "--no-pager", "-o", "short-iso")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return lc.filterLogLines(string(output)), nil
}

// pm2Process represents a pm2 process from jlist output
type pm2Process struct {
	Name string `json:"name"`
	PID  int    `json:"pid"`
}

// collectPM2Logs collects logs from pm2 for Node.js applications
func (lc *LogCollector) collectPM2Logs(pid int) []string {
	// First, try to find the pm2 app name by PID
	appName := lc.findPM2AppByPID(pid)
	if appName == "" {
		return nil
	}

	// Get pm2 home directory (usually ~/.pm2)
	pm2Home := os.Getenv("PM2_HOME")
	if pm2Home == "" {
		// Try default locations
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		pm2Home = filepath.Join(homeDir, ".pm2")
	}

	logsDir := filepath.Join(pm2Home, "logs")

	// Try to read the error log file first (more likely to have interesting content)
	var allLogs []string

	// PM2 log file naming: <app-name>-error.log and <app-name>-out.log
	errorLogPath := filepath.Join(logsDir, appName+"-error.log")
	if logs := lc.readLastLines(errorLogPath, 50); len(logs) > 0 {
		allLogs = append(allLogs, logs...)
	}

	// Also check stdout log for errors
	outLogPath := filepath.Join(logsDir, appName+"-out.log")
	if logs := lc.readLastLines(outLogPath, 50); len(logs) > 0 {
		allLogs = append(allLogs, logs...)
	}

	// Filter for interesting lines
	return lc.filterLogLines(strings.Join(allLogs, "\n"))
}

// findPM2AppByPID finds pm2 application name by its PID
func (lc *LogCollector) findPM2AppByPID(pid int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Use pm2 jlist to get JSON list of all processes
	cmd := exec.CommandContext(ctx, "pm2", "jlist")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	var processes []pm2Process
	if err := json.Unmarshal(output, &processes); err != nil {
		return ""
	}

	for _, proc := range processes {
		if proc.PID == pid {
			return proc.Name
		}
	}

	return ""
}

// readLastLines reads the last N lines from a file
func (lc *LogCollector) readLastLines(filePath string, n int) []string {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil
	}

	fileSize := stat.Size()
	if fileSize == 0 {
		return nil
	}

	// Read from the end of the file
	// Estimate: average line length ~100 bytes, read n*150 bytes to be safe
	bufSize := int64(n * 150)
	if bufSize > fileSize {
		bufSize = fileSize
	}

	// Seek to position near end
	startPos := fileSize - bufSize
	if startPos < 0 {
		startPos = 0
	}

	_, err = file.Seek(startPos, 0)
	if err != nil {
		return nil
	}

	// Read the buffer
	buf := make([]byte, bufSize)
	bytesRead, err := file.Read(buf)
	if err != nil {
		return nil
	}

	// Split into lines
	content := string(buf[:bytesRead])
	lines := strings.Split(content, "\n")

	// If we started mid-line, skip the first partial line
	if startPos > 0 && len(lines) > 0 {
		lines = lines[1:]
	}

	// Remove empty last line if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Return last n lines
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return lines
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
