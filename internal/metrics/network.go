package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/net"
)

// Global bandwidth cache - updated by background goroutine every second
var (
	cachedBandwidth      *bandwidthMeasurement
	bandwidthMutex       sync.RWMutex
	bandwidthInitialized bool
)

// NetworkMetrics represents network monitoring data
type NetworkMetrics struct {
	// Bandwidth (measured over 1 second interval)
	InboundBytesPerSec  int64 `json:"inbound_bytes_per_sec"`
	OutboundBytesPerSec int64 `json:"outbound_bytes_per_sec"`

	// Connection states
	ConnectionsEstablished int `json:"connections_established"`
	ConnectionsTimeWait    int `json:"connections_time_wait"`
	ConnectionsCloseWait   int `json:"connections_close_wait"`
	ConnectionsSynSent     int `json:"connections_syn_sent"`
	ConnectionsSynRecv     int `json:"connections_syn_recv"`
	ConnectionsFinWait1    int `json:"connections_fin_wait1"`
	ConnectionsFinWait2    int `json:"connections_fin_wait2"`
	ConnectionsListen      int `json:"connections_listen"`

	// Total connections
	TotalConnections int `json:"total_connections"`

	// Top connections (by bytes transferred)
	TopConnections []NetworkConnection `json:"top_connections"`
}

// NetworkConnection represents a single network connection
type NetworkConnection struct {
	RemoteIP        string `json:"remote_ip"`
	RemotePort      int    `json:"remote_port"`
	LocalPort       int    `json:"local_port"`
	Protocol        string `json:"protocol"` // TCP, UDP
	State           string `json:"state"`
	PID             int32  `json:"pid"`
	Family          string `json:"family"` // IPv4, IPv6
	ImportanceScore int    `json:"importance_score"`
}

// StartBandwidthMonitoring starts background goroutine to measure bandwidth continuously
// This prevents blocking the main metrics collection loop
func StartBandwidthMonitoring() {
	bandwidthMutex.Lock()
	if bandwidthInitialized {
		bandwidthMutex.Unlock()
		return
	}
	bandwidthInitialized = true
	bandwidthMutex.Unlock()

	// Initialize with zero values
	cachedBandwidth = &bandwidthMeasurement{}

	// Start background measurement
	go func() {
		// CRITICAL: Recover from panic to prevent daemon crash
		defer func() {
			if r := recover(); r != nil {
				// Log panic but don't crash daemon
				// Bandwidth measurement will just stop working
			}
		}()

		for {
			bandwidth, err := measureBandwidth()
			if err == nil {
				bandwidthMutex.Lock()
				cachedBandwidth = bandwidth
				bandwidthMutex.Unlock()
			}
			// Sleep handled inside measureBandwidth, so loop continues immediately
		}
	}()
}

// GetNetworkMetrics collects network metrics
func GetNetworkMetrics() (*NetworkMetrics, error) {
	metrics := &NetworkMetrics{
		TopConnections: []NetworkConnection{},
	}

	// Get cached bandwidth (non-blocking)
	bandwidthMutex.RLock()
	if cachedBandwidth != nil {
		metrics.InboundBytesPerSec = cachedBandwidth.InboundBytes
		metrics.OutboundBytesPerSec = cachedBandwidth.OutboundBytes
	}
	bandwidthMutex.RUnlock()

	// Get all connections
	connections, err := net.Connections("all")
	if err != nil {
		return nil, fmt.Errorf("failed to get connections: %w", err)
	}

	metrics.TotalConnections = len(connections)

	// Count connections by state
	for _, conn := range connections {
		switch conn.Status {
		case "ESTABLISHED":
			metrics.ConnectionsEstablished++
		case "TIME_WAIT":
			metrics.ConnectionsTimeWait++
		case "CLOSE_WAIT":
			metrics.ConnectionsCloseWait++
		case "SYN_SENT":
			metrics.ConnectionsSynSent++
		case "SYN_RECV":
			metrics.ConnectionsSynRecv++
		case "FIN_WAIT1":
			metrics.ConnectionsFinWait1++
		case "FIN_WAIT2":
			metrics.ConnectionsFinWait2++
		case "LISTEN":
			metrics.ConnectionsListen++
		}
	}

	// Get top connections (limit to 20 for performance)
	topConns := getTopConnections(connections, 20)
	metrics.TopConnections = topConns

	return metrics, nil
}

// bandwidthMeasurement holds bandwidth data
type bandwidthMeasurement struct {
	InboundBytes  int64
	OutboundBytes int64
}

// measureBandwidth measures network bandwidth over 1 second interval
func measureBandwidth() (*bandwidthMeasurement, error) {
	// Get initial counters
	io1, err := net.IOCounters(false)
	if err != nil {
		return nil, err
	}

	if len(io1) == 0 {
		return &bandwidthMeasurement{}, nil
	}

	// Wait 1 second
	time.Sleep(1 * time.Second)

	// Get counters again
	io2, err := net.IOCounters(false)
	if err != nil {
		return nil, err
	}

	if len(io2) == 0 {
		return &bandwidthMeasurement{}, nil
	}

	// Calculate bytes per second
	inboundBytes := int64(io2[0].BytesRecv - io1[0].BytesRecv)
	outboundBytes := int64(io2[0].BytesSent - io1[0].BytesSent)

	return &bandwidthMeasurement{
		InboundBytes:  inboundBytes,
		OutboundBytes: outboundBytes,
	}, nil
}

// connectionWithScore is used for sorting connections
type connectionWithScore struct {
	conn  net.ConnectionStat
	score int
}

// getTopConnections returns top N connections by importance
// Priority: ESTABLISHED > SYN_* > TIME_WAIT > others
func getTopConnections(connections []net.ConnectionStat, limit int) []NetworkConnection {
	// Score connections by importance
	scored := make([]connectionWithScore, 0, len(connections))
	for _, conn := range connections {
		score := getConnectionScore(conn)
		if score > 0 { // Only include scored connections
			scored = append(scored, connectionWithScore{conn: conn, score: score})
		}
	}

	// Sort by score (descending)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Convert to NetworkConnection and limit
	result := make([]NetworkConnection, 0, limit)
	for i := 0; i < len(scored) && i < limit; i++ {
		conn := scored[i].conn
		score := scored[i].score

		// Determine protocol
		protocol := "TCP"
		if conn.Type == 2 { // SOCK_DGRAM
			protocol = "UDP"
		}

		// Determine IP family
		family := "IPv4"
		if conn.Family == 10 { // AF_INET6
			family = "IPv6"
		}

		result = append(result, NetworkConnection{
			RemoteIP:        anonymizeIP(conn.Raddr.IP), // GDPR: Anonymize IP before storage
			RemotePort:      int(conn.Raddr.Port),
			LocalPort:       int(conn.Laddr.Port),
			Protocol:        protocol,
			State:           conn.Status,
			PID:             conn.Pid,
			Family:          family,
			ImportanceScore: score,
		})
	}

	return result
}

// getConnectionScore assigns importance score to connections
func getConnectionScore(conn net.ConnectionStat) int {
	// Skip if no remote address (listening sockets, etc.)
	if conn.Raddr.IP == "" || conn.Raddr.IP == "0.0.0.0" || conn.Raddr.IP == "::" {
		return 0
	}

	// Score by state
	switch conn.Status {
	case "ESTABLISHED":
		return 100 // Highest priority - active connections
	case "SYN_SENT", "SYN_RECV":
		return 80 // Connection being established
	case "CLOSE_WAIT", "FIN_WAIT1", "FIN_WAIT2":
		return 60 // Connection closing
	case "TIME_WAIT":
		return 40 // Recently closed
	default:
		return 20 // Other states
	}
}

// anonymizeIP masks IP addresses for GDPR compliance (Article 25 - Data Protection by Design)
// IPv4: 192.168.1.100 → 192.168.1.0 (masks last octet)
// IPv6: 2001:db8::1 → 2001:db8:: (masks last 64 bits)
// This pseudonymization preserves subnet information for security analysis while protecting privacy
func anonymizeIP(ip string) string {
	if ip == "" {
		return ""
	}

	// Check if IPv6
	if strings.Contains(ip, ":") {
		// IPv6 - mask last 64 bits (interface identifier)
		parts := strings.Split(ip, ":")
		if len(parts) <= 4 {
			// Already short format, mask last part
			return strings.Join(parts[:len(parts)-1], ":") + ":"
		}
		// Keep first 4 parts (network prefix /64), mask the rest
		return strings.Join(parts[:4], ":") + "::"
	}

	// IPv4 - mask last octet
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		// Invalid IPv4, return as-is
		return ip
	}
	// Keep first 3 octets, set last to 0
	return parts[0] + "." + parts[1] + "." + parts[2] + ".0"
}
