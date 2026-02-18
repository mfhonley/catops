package metrics

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxLogLines   = 500  // Maximum log lines to collect per container
	logTimeout    = 10   // Timeout in seconds for log collection
	maxLogLineLen = 2000 // Maximum length per log line
)

// Package-level singleton for log collector to maintain deduplication state across collection cycles
var (
	globalLogCollector     *LogCollector
	globalLogCollectorOnce sync.Once
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

	// Deduplication: track sent log hashes to avoid sending same logs twice
	sentLogHashes   map[string]time.Time // hash -> when it was sent
	sentLogHashesMu sync.Mutex
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
		sentLogHashes:    make(map[string]time.Time),
	}
	// Pre-load docker containers
	lc.loadDockerContainers()
	return lc
}

// GetLogCollector returns the global log collector singleton
// This ensures deduplication state is maintained across collection cycles
func GetLogCollector() *LogCollector {
	globalLogCollectorOnce.Do(func() {
		globalLogCollector = NewLogCollector()
		// Background goroutine to periodically clean stale log hashes
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				globalLogCollector.sentLogHashesMu.Lock()
				cutoff := time.Now().Add(-10 * time.Minute)
				for hash, sentAt := range globalLogCollector.sentLogHashes {
					if sentAt.Before(cutoff) {
						delete(globalLogCollector.sentLogHashes, hash)
					}
				}
				globalLogCollector.sentLogHashesMu.Unlock()
			}
		}()
	})
	// Refresh docker containers list on each call (they might have changed)
	globalLogCollector.loadDockerContainers()
	return globalLogCollector
}

// loadDockerContainers loads all running docker containers
func (lc *LogCollector) loadDockerContainers() {
	// Reset cache before reloading to avoid stale entries from stopped containers
	lc.dockerContainers = make(map[string]DockerContainer)
	lc.dockerLoaded = false

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
	for key, container := range lc.dockerContainers {
		nameLower := strings.ToLower(container.Name)
		if strings.Contains(nameLower, serviceLower) ||
			strings.Contains(serviceLower, nameLower) {
			// Return pointer to map entry, not to the range variable
			c := lc.dockerContainers[key]
			return &c
		}
	}

	return nil
}

// CollectServiceLogs collects logs for a service
func (lc *LogCollector) CollectServiceLogs(service *ServiceInfo) ([]string, string) {
	// 1. Try to find docker container for this service
	container := lc.findContainerForService(service)
	if container != nil {
		logs, _ := lc.collectDockerLogs(container.ID)
		if len(logs) > 0 {
			service.ContainerID = container.ID
			service.IsContainer = true
			return logs, "docker"
		}
	}

	// 2. If explicitly marked as container, try by container ID
	if service.IsContainer && service.ContainerID != "" {
		logs, err := lc.collectDockerLogs(service.ContainerID)
		if err == nil && len(logs) > 0 {
			return logs, "docker"
		}
	}

	// 3. Try PM2 logs for Node.js services
	if service.ServiceType == ServiceTypeNodeApp {
		logs := lc.collectPM2Logs(service.PID)
		if len(logs) > 0 {
			return logs, "pm2"
		}
	}

	return nil, ""
}

// CollectContainerLogs collects logs directly from a container by ID
// This is the simple approach like self-hosted - just get docker logs
func (lc *LogCollector) CollectContainerLogs(containerID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(logTimeout)*time.Second)
	defer cancel()

	// Get last N lines of logs with timestamps
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", maxLogLines), "--timestamps", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	// Filter for error/warning lines, then deduplicate
	filtered := lc.filterLogLines(string(output))
	return lc.deduplicateLogs(filtered), nil
}

// collectDockerLogs collects recent logs from a Docker container (legacy method for services)
func (lc *LogCollector) collectDockerLogs(containerID string) ([]string, error) {
	return lc.CollectContainerLogs(containerID)
}

// hashLogLine creates a hash of a log line for deduplication
func (lc *LogCollector) hashLogLine(line string) string {
	hash := md5.Sum([]byte(line))
	return hex.EncodeToString(hash[:])
}

// deduplicateLogs filters out logs that have already been sent
func (lc *LogCollector) deduplicateLogs(logs []string) []string {
	lc.sentLogHashesMu.Lock()
	defer lc.sentLogHashesMu.Unlock()

	// Clean up old hashes (older than 10 minutes) to prevent memory growth
	cutoff := time.Now().Add(-10 * time.Minute)
	for hash, sentAt := range lc.sentLogHashes {
		if sentAt.Before(cutoff) {
			delete(lc.sentLogHashes, hash)
		}
	}

	var newLogs []string
	now := time.Now()

	for _, log := range logs {
		hash := lc.hashLogLine(log)
		if _, alreadySent := lc.sentLogHashes[hash]; !alreadySent {
			// This is a new log line, mark it as sent
			lc.sentLogHashes[hash] = now
			newLogs = append(newLogs, log)
		}
	}

	return newLogs
}

// pm2Process represents a pm2 process from jlist output
type pm2Process struct {
	Name     string `json:"name"`
	PID      int    `json:"pid"`
	PM2Env   struct {
		PM2Home    string `json:"PM2_HOME"`
		ErrLogPath string `json:"pm_err_log_path"`
		OutLogPath string `json:"pm_out_log_path"`
	} `json:"pm2_env"`
}

// GetPM2AppName returns the PM2 app name by reading ~/.pm2/dump.pm2 directly.
// This avoids calling pm2 jlist which fails under systemd due to nvm PATH issues.
func GetPM2AppName(pid int) string {
	// dump.pm2 is the persisted process list, always readable as a file
	candidates := []string{"/root/.pm2/dump.pm2"}
	if home := os.Getenv("HOME"); home != "" && home != "/root" {
		candidates = append([]string{filepath.Join(home, ".pm2", "dump.pm2")}, candidates...)
	}

	type dumpEntry struct {
		Name   string `json:"name"`
		PID    int    `json:"pid"`
		PM2Env struct {
			Status string `json:"status"`
		} `json:"pm2_env"`
	}
	type dumpFile struct {
		List []dumpEntry `json:"list"`
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var dump dumpFile
		if err := json.Unmarshal(data, &dump); err != nil {
			continue
		}
		// Walk parent chain to match pid to a pm2 app
		current := pid
		for depth := 0; depth < 5 && current > 1; depth++ {
			for _, entry := range dump.List {
				if entry.PID == current {
					return entry.Name
				}
			}
			current = getPPid(current)
		}
		// Fallback: return first app name if any pm2 app exists
		if len(dump.List) > 0 {
			return dump.List[0].Name
		}
	}
	return ""
}

// getPPid reads the parent PID of a process from /proc/<pid>/status
func getPPid(pid int) int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ppid, err := strconv.Atoi(fields[1])
				if err == nil {
					return ppid
				}
			}
		}
	}
	return 0
}

// findPM2Binary returns the path to pm2 binary, searching common locations
func findPM2Binary() string {
	// Try PATH first
	if path, err := exec.LookPath("pm2"); err == nil {
		return path
	}
	// Common locations when installed via nvm or npm globally
	candidates := []string{
		"/usr/local/bin/pm2",
		"/usr/bin/pm2",
	}
	// Also search nvm paths under common home dirs
	for _, home := range []string{"/root", "/home/ubuntu", "/home/node"} {
		nvmBase := home + "/.nvm/versions/node"
		if entries, err := os.ReadDir(nvmBase); err == nil {
			for _, e := range entries {
				candidates = append(candidates, nvmBase+"/"+e.Name()+"/bin/pm2")
			}
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// globalGetPM2AppByPID finds pm2 process by walking up the parent PID chain (up to 4 levels)
func globalGetPM2AppByPID(pid int) *pm2Process {
	pm2bin := findPM2Binary()
	if pm2bin == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, pm2bin, "jlist")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	raw := string(output)
	start := strings.Index(raw, "[")
	if start == -1 {
		return nil
	}
	raw = raw[start:]

	var processes []pm2Process
	if err := json.Unmarshal([]byte(raw), &processes); err != nil {
		return nil
	}

	// Walk up parent chain: node(2933) -> sh(2932) -> enrichment(2921) -> PM2 God(2276)
	current := pid
	for depth := 0; depth < 5 && current > 1; depth++ {
		for i := range processes {
			if processes[i].PID == current {
				return &processes[i]
			}
		}
		current = getPPid(current)
	}

	// Fallback: if we found pm2 is running and this is a node process,
	// return the first pm2 app that has log files (handles edge cases where
	// PID chain doesn't match, e.g. due to process replacement)
	if len(processes) > 0 {
		for i := range processes {
			if processes[i].PM2Env.ErrLogPath != "" || processes[i].PM2Env.OutLogPath != "" {
				return &processes[i]
			}
		}
	}

	return nil
}

// collectPM2Logs collects logs for Node.js apps managed by PM2.
// Scans ~/.pm2/logs/ directly to avoid pm2 jlist which fails in systemd
// due to nvm PATH not being available.
func (lc *LogCollector) collectPM2Logs(pid int) []string {
	// Build list of candidate pm2 log dirs
	pm2LogsDirs := []string{"/root/.pm2/logs"}
	if home := os.Getenv("HOME"); home != "" && home != "/root" {
		pm2LogsDirs = append(pm2LogsDirs, filepath.Join(home, ".pm2", "logs"))
	}

	var allLogs []string
	seen := map[string]bool{}

	for _, logsDir := range pm2LogsDirs {
		entries, err := os.ReadDir(logsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, "-error.log") && !strings.HasSuffix(name, "-out.log") {
				continue
			}
			fullPath := filepath.Join(logsDir, name)
			if seen[fullPath] {
				continue
			}
			seen[fullPath] = true
			if logs := lc.readLastLines(fullPath, 50); len(logs) > 0 {
				allLogs = append(allLogs, logs...)
			}
		}
	}

	if len(allLogs) == 0 {
		return nil
	}
	return lc.filterLogLines(strings.Join(allLogs, "\n"))
}

// findPM2AppByPID finds pm2 process by its PID using pm2 jlist
// Also checks parent PID to handle forked worker processes
func (lc *LogCollector) findPM2AppByPID(pid int) *pm2Process {
	return globalGetPM2AppByPID(pid)
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

// GetAllServiceLogs collects logs for all detected services
func (lc *LogCollector) GetAllServiceLogs(services []ServiceInfo) []ServiceInfo {
	for i := range services {
		logs, source := lc.CollectServiceLogs(&services[i])
		services[i].RecentLogs = logs
		services[i].LogSource = source
	}
	return services
}
