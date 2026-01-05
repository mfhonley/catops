package metrics

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// Cache for process CPU times (for delta calculation)
var (
	procCpuTimesCache   = make(map[int32]procCpuCacheEntry)
	procCpuTimesCacheMu sync.RWMutex
)

type procCpuCacheEntry struct {
	userTime   float64
	sysTime    float64
	sampleTime time.Time
}

// getProcessCPUPercent calculates CPU% for a process using delta from cached values.
// Non-blocking - uses process.Times() which just reads /proc/[pid]/stat.
func getProcessCPUPercent(proc *process.Process) float64 {
	times, err := proc.Times()
	if err != nil || times == nil {
		return 0
	}

	now := time.Now()
	pid := proc.Pid
	currentTotal := times.User + times.System

	procCpuTimesCacheMu.Lock()
	defer procCpuTimesCacheMu.Unlock()

	prev, exists := procCpuTimesCache[pid]
	if !exists {
		// First call - store baseline, return 0
		procCpuTimesCache[pid] = procCpuCacheEntry{
			userTime:   times.User,
			sysTime:    times.System,
			sampleTime: now,
		}
		return 0
	}

	elapsed := now.Sub(prev.sampleTime).Seconds()
	if elapsed <= 0 {
		return 0
	}

	prevTotal := prev.userTime + prev.sysTime
	cpuDelta := currentTotal - prevTotal

	// Update cache
	procCpuTimesCache[pid] = procCpuCacheEntry{
		userTime:   times.User,
		sysTime:    times.System,
		sampleTime: now,
	}

	// CPU% = (cpu time delta / wall time delta) * 100
	cpuPercent := (cpuDelta / elapsed) * 100
	if cpuPercent < 0 {
		cpuPercent = 0
	}
	if cpuPercent > 100 {
		cpuPercent = 100
	}

	return cpuPercent
}

// ListeningPort represents a port that is being listened on
type ListeningPort struct {
	Port        int         `json:"port"`
	Protocol    string      `json:"protocol"` // tcp, udp
	PID         int         `json:"pid"`
	ProcessName string      `json:"process_name"`
	ServiceType ServiceType `json:"service_type"`
}

// ServiceDetector detects services running on the system
type ServiceDetector struct {
	listeningPorts map[int]ListeningPort // port -> ListeningPort
}

// NewServiceDetector creates a new ServiceDetector
func NewServiceDetector() *ServiceDetector {
	return &ServiceDetector{
		listeningPorts: make(map[int]ListeningPort),
	}
}

// DetectServices detects all running services on the system
func (d *ServiceDetector) DetectServices() ([]ServiceInfo, error) {
	// First, collect all listening ports
	if err := d.collectListeningPorts(); err != nil {
		// Continue even if we can't get ports - we can still detect by cmdline
	}

	// Get all processes (use cached from collector)
	allProcesses, err := getCachedProcesses()
	if err != nil {
		return nil, fmt.Errorf("failed to get processes: %w", err)
	}

	var services []ServiceInfo

	for _, proc := range allProcesses {
		name, err := proc.Name()
		if err != nil {
			continue
		}

		// Quick check by name first - skip unknown processes early
		// This avoids expensive Cmdline() call for most processes
		serviceType, framework := d.detectServiceTypeByName(name)
		if serviceType == ServiceTypeUnknown {
			continue // Skip unknown services - no need for Cmdline()
		}

		// Only get cmdline for known service types (for framework detection)
		cmdline, _ := proc.Cmdline()
		if cmdline == "" {
			cmdline = name
		}

		// Refine detection with cmdline if needed
		if framework == "" {
			_, framework = d.detectServiceType(name, cmdline)
		}

		// Get process stats - only for detected services (not all 200+ processes)
		cpuPercent := getProcessCPUPercent(proc)
		memoryPercent, _ := proc.MemoryPercent()
		memoryInfo, _ := proc.MemoryInfo()
		status, _ := proc.Status()

		var memoryKB int64
		if memoryInfo != nil {
			memoryKB = int64(memoryInfo.RSS / 1024)
		}

		statusChar := "R"
		if len(status) > 0 {
			statusChar = string(status[0])
		}

		// Find listening ports for this process
		ports := d.getPortsForPID(int(proc.Pid))
		primaryPort := 0
		if len(ports) > 0 {
			primaryPort = ports[0]
		}

		// Generate service name
		serviceName := d.generateServiceName(serviceType, framework, primaryPort)

		// Check if running in container
		isContainer, containerID := d.detectContainer(int(proc.Pid))

		// Note: cmdline is used for service detection but not stored in ServiceInfo
		_ = cmdline

		// Convert ports to uint16
		portsU16 := make([]uint16, len(ports))
		for i, p := range ports {
			portsU16[i] = uint16(p)
		}

		service := ServiceInfo{
			ServiceType:   serviceType,
			ServiceName:   serviceName,
			PID:           int(proc.Pid),
			PIDs:          []int{int(proc.Pid)},
			Ports:         portsU16,
			Protocol:      "tcp",
			BindAddress:   "0.0.0.0",
			CPUPercent:    cpuPercent,
			MemoryPercent: float64(memoryPercent),
			MemoryBytes:   uint64(memoryKB * 1024),
			Version:       "",
			ConfigPath:    "",
			Status:        statusChar,
			IsContainer:   isContainer,
			ContainerID:   containerID,
			ContainerName: "",
			HealthStatus:  "",
		}

		services = append(services, service)
	}

	return services, nil
}

// collectListeningPorts collects all listening ports on the system
func (d *ServiceDetector) collectListeningPorts() error {
	connections, err := getCachedConnections()
	if err != nil {
		return fmt.Errorf("failed to get connections: %w", err)
	}

	d.listeningPorts = make(map[int]ListeningPort)

	for _, conn := range connections {
		// Only interested in LISTEN state
		if conn.Status != "LISTEN" {
			continue
		}

		port := int(conn.Laddr.Port)
		d.listeningPorts[port] = ListeningPort{
			Port:     port,
			Protocol: "tcp",
			PID:      int(conn.Pid),
		}
	}

	return nil
}

// getPortsForPID returns all listening ports for a given PID
func (d *ServiceDetector) getPortsForPID(pid int) []int {
	var ports []int
	for port, lp := range d.listeningPorts {
		if lp.PID == pid {
			ports = append(ports, port)
		}
	}
	return ports
}

// detectServiceTypeByName is a fast check using only process name (no cmdline syscall)
// Returns ServiceTypeUnknown for processes that definitely aren't services
func (d *ServiceDetector) detectServiceTypeByName(name string) (ServiceType, string) {
	nameLower := strings.ToLower(name)

	// Known service process names - quick lookup
	switch nameLower {
	case "nginx":
		return ServiceTypeNginx, ""
	case "apache2", "httpd":
		return ServiceTypeApache, ""
	case "redis-server":
		return ServiceTypeRedis, ""
	case "postgres", "postgresql":
		return ServiceTypePostgres, ""
	case "mysqld", "mariadbd":
		return ServiceTypeMySQL, ""
	case "mongod":
		return ServiceTypeMongoDB, ""
	case "node", "nodejs":
		return ServiceTypeNodeApp, ""
	case "python", "python3", "python3.8", "python3.9", "python3.10", "python3.11", "python3.12":
		return ServiceTypePythonApp, ""
	case "java":
		return ServiceTypeJavaApp, ""
	case "containerd", "dockerd", "docker":
		return ServiceTypeDocker, ""
	case "kubelet", "kube-proxy", "kube-apiserver":
		return ServiceTypeKubernetes, ""
	}

	return ServiceTypeUnknown, ""
}

// detectServiceType determines the service type from process name and command line
func (d *ServiceDetector) detectServiceType(name, cmdline string) (ServiceType, string) {
	nameLower := strings.ToLower(name)
	cmdLower := strings.ToLower(cmdline)

	// Web servers
	if nameLower == "nginx" || strings.Contains(cmdLower, "nginx") {
		return ServiceTypeNginx, ""
	}
	if nameLower == "apache2" || nameLower == "httpd" || strings.Contains(cmdLower, "apache") {
		return ServiceTypeApache, ""
	}

	// Databases
	if nameLower == "redis-server" || strings.Contains(cmdLower, "redis-server") {
		return ServiceTypeRedis, ""
	}
	if nameLower == "postgres" || strings.Contains(cmdLower, "postgres") || strings.Contains(cmdLower, "postgresql") {
		return ServiceTypePostgres, ""
	}
	if nameLower == "mysqld" || nameLower == "mariadbd" || strings.Contains(cmdLower, "mysqld") {
		return ServiceTypeMySQL, ""
	}
	if nameLower == "mongod" || strings.Contains(cmdLower, "mongod") {
		return ServiceTypeMongoDB, ""
	}

	// Python applications
	if nameLower == "python" || nameLower == "python3" || strings.HasPrefix(nameLower, "python") {
		framework := d.detectPythonFramework(cmdLower)
		if framework != "" {
			return ServiceTypePythonApp, framework
		}
		// Only report if it has listening ports (likely a server)
		return ServiceTypePythonApp, "python"
	}

	// Node.js applications
	if nameLower == "node" || strings.Contains(cmdLower, "node ") {
		framework := d.detectNodeFramework(cmdLower)
		if framework != "" {
			return ServiceTypeNodeApp, framework
		}
		return ServiceTypeNodeApp, "node"
	}

	// Go applications (hard to detect, but some patterns)
	if strings.Contains(cmdLower, "go run") || strings.Contains(cmdLower, "go-") {
		return ServiceTypeGoApp, "go"
	}

	// Java applications
	if nameLower == "java" || strings.Contains(cmdLower, "java ") {
		framework := d.detectJavaFramework(cmdLower)
		return ServiceTypeJavaApp, framework
	}

	// Docker/Kubernetes
	if nameLower == "dockerd" || nameLower == "docker" {
		return ServiceTypeDocker, ""
	}
	if nameLower == "kubelet" || nameLower == "kube-apiserver" || strings.HasPrefix(nameLower, "kube-") {
		return ServiceTypeKubernetes, ""
	}

	return ServiceTypeUnknown, ""
}

// detectPythonFramework detects the Python web framework from command line
func (d *ServiceDetector) detectPythonFramework(cmdLower string) string {
	if strings.Contains(cmdLower, "gunicorn") {
		if strings.Contains(cmdLower, "uvicorn") || strings.Contains(cmdLower, "fastapi") {
			return "gunicorn+uvicorn"
		}
		return "gunicorn"
	}
	if strings.Contains(cmdLower, "uvicorn") {
		return "uvicorn"
	}
	if strings.Contains(cmdLower, "flask") {
		return "flask"
	}
	if strings.Contains(cmdLower, "django") {
		return "django"
	}
	if strings.Contains(cmdLower, "fastapi") {
		return "fastapi"
	}
	if strings.Contains(cmdLower, "celery") {
		return "celery"
	}
	if strings.Contains(cmdLower, "manage.py runserver") {
		return "django"
	}
	return ""
}

// detectNodeFramework detects the Node.js framework from command line
func (d *ServiceDetector) detectNodeFramework(cmdLower string) string {
	if strings.Contains(cmdLower, "next") {
		return "next.js"
	}
	if strings.Contains(cmdLower, "nuxt") {
		return "nuxt"
	}
	if strings.Contains(cmdLower, "express") {
		return "express"
	}
	if strings.Contains(cmdLower, "nest") {
		return "nestjs"
	}
	if strings.Contains(cmdLower, "pm2") {
		return "pm2"
	}
	if strings.Contains(cmdLower, "vite") {
		return "vite"
	}
	if strings.Contains(cmdLower, "webpack") {
		return "webpack"
	}
	return ""
}

// detectJavaFramework detects the Java framework from command line
func (d *ServiceDetector) detectJavaFramework(cmdLower string) string {
	if strings.Contains(cmdLower, "spring") || strings.Contains(cmdLower, "boot") {
		return "spring-boot"
	}
	if strings.Contains(cmdLower, "tomcat") {
		return "tomcat"
	}
	if strings.Contains(cmdLower, "jetty") {
		return "jetty"
	}
	if strings.Contains(cmdLower, "quarkus") {
		return "quarkus"
	}
	return "java"
}

// generateServiceName generates a human-readable service name
func (d *ServiceDetector) generateServiceName(serviceType ServiceType, framework string, port int) string {
	switch serviceType {
	case ServiceTypeNginx:
		if port > 0 {
			return fmt.Sprintf("nginx (port %d)", port)
		}
		return "nginx"
	case ServiceTypeApache:
		if port > 0 {
			return fmt.Sprintf("Apache HTTP (port %d)", port)
		}
		return "Apache HTTP"
	case ServiceTypeRedis:
		if port > 0 {
			return fmt.Sprintf("Redis (port %d)", port)
		}
		return "Redis"
	case ServiceTypePostgres:
		if port > 0 {
			return fmt.Sprintf("PostgreSQL (port %d)", port)
		}
		return "PostgreSQL"
	case ServiceTypeMySQL:
		if port > 0 {
			return fmt.Sprintf("MySQL (port %d)", port)
		}
		return "MySQL"
	case ServiceTypeMongoDB:
		if port > 0 {
			return fmt.Sprintf("MongoDB (port %d)", port)
		}
		return "MongoDB"
	case ServiceTypePythonApp:
		if framework != "" && framework != "python" {
			if port > 0 {
				return fmt.Sprintf("Python %s (port %d)", framework, port)
			}
			return fmt.Sprintf("Python %s", framework)
		}
		if port > 0 {
			return fmt.Sprintf("Python app (port %d)", port)
		}
		return "Python app"
	case ServiceTypeNodeApp:
		if framework != "" && framework != "node" {
			if port > 0 {
				return fmt.Sprintf("Node.js %s (port %d)", framework, port)
			}
			return fmt.Sprintf("Node.js %s", framework)
		}
		if port > 0 {
			return fmt.Sprintf("Node.js app (port %d)", port)
		}
		return "Node.js app"
	case ServiceTypeGoApp:
		if port > 0 {
			return fmt.Sprintf("Go app (port %d)", port)
		}
		return "Go app"
	case ServiceTypeJavaApp:
		if framework != "" && framework != "java" {
			if port > 0 {
				return fmt.Sprintf("Java %s (port %d)", framework, port)
			}
			return fmt.Sprintf("Java %s", framework)
		}
		if port > 0 {
			return fmt.Sprintf("Java app (port %d)", port)
		}
		return "Java app"
	case ServiceTypeDocker:
		return "Docker daemon"
	case ServiceTypeKubernetes:
		return "Kubernetes"
	default:
		return "Unknown service"
	}
}

// detectContainer checks if a process is running inside a container
func (d *ServiceDetector) detectContainer(pid int) (bool, string) {
	// Check if running under containerd or docker
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false, ""
	}

	parent, err := proc.Parent()
	if err != nil {
		return false, ""
	}

	if parent != nil {
		parentName, _ := parent.Name()
		parentNameLower := strings.ToLower(parentName)
		if strings.Contains(parentNameLower, "containerd") ||
			strings.Contains(parentNameLower, "docker") ||
			strings.Contains(parentNameLower, "runc") {
			return true, ""
		}
	}

	return false, ""
}

// GetServices is a convenience function to detect services
func GetServices() ([]ServiceInfo, error) {
	detector := NewServiceDetector()
	services, err := detector.DetectServices()
	if err != nil {
		return nil, err
	}

	// Collect logs for each service (using singleton to maintain deduplication state)
	logCollector := GetLogCollector()
	services = logCollector.GetAllServiceLogs(services)

	return services, nil
}
