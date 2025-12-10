package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/net"
)

var (
	cachedBandwidth   *bandwidthMeasurement
	bandwidthMutex    sync.RWMutex
	bandwidthOnce     sync.Once     // Ensures initialization happens only once
	bandwidthStopChan chan struct{} // Channel for graceful shutdown
	bandwidthStopOnce sync.Once     // Ensures channel is closed only once
)

// NetworkMetrics represents network monitoring data
type NetworkMetrics struct {
	InboundBytesPerSec  int64 `json:"inbound_bytes_per_sec"`
	OutboundBytesPerSec int64 `json:"outbound_bytes_per_sec"`

	ConnectionsEstablished int `json:"connections_established"`
	ConnectionsTimeWait    int `json:"connections_time_wait"`
	ConnectionsCloseWait   int `json:"connections_close_wait"`
	ConnectionsSynSent     int `json:"connections_syn_sent"`
	ConnectionsSynRecv     int `json:"connections_syn_recv"`
	ConnectionsFinWait1    int `json:"connections_fin_wait1"`
	ConnectionsFinWait2    int `json:"connections_fin_wait2"`
	ConnectionsListen      int `json:"connections_listen"`

	TotalConnections int                 `json:"total_connections"`
	TopConnections   []NetworkConnection `json:"top_connections"`

	// Aggregated stats for legacy API
	TotalBytesIn   uint64          `json:"total_bytes_in"`
	TotalBytesOut  uint64          `json:"total_bytes_out"`
	TotalPacketsIn  uint64         `json:"total_packets_in"`
	TotalPacketsOut uint64         `json:"total_packets_out"`
	TotalErrorsIn   int64          `json:"total_errors_in"`
	TotalErrorsOut  int64          `json:"total_errors_out"`
	Interfaces      []InterfaceInfo `json:"interfaces"`
}

// InterfaceInfo represents network interface info for legacy API
type InterfaceInfo struct {
	Name        string   `json:"name"`
	BytesIn     uint64   `json:"bytes_in"`
	BytesOut    uint64   `json:"bytes_out"`
	PacketsIn   uint64   `json:"packets_in"`
	PacketsOut  uint64   `json:"packets_out"`
	ErrorsIn    int64    `json:"errors_in"`
	ErrorsOut   int64    `json:"errors_out"`
	IsUp        bool     `json:"is_up"`
	MTU         int      `json:"mtu"`
	Speed       int64    `json:"speed"`
	IPAddresses []string `json:"ip_addresses"`
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
// Thread-safe: Uses sync.Once to ensure initialization happens only once
func StartBandwidthMonitoring() {
	bandwidthOnce.Do(func() {
		// Initialize stop channel and cached bandwidth
		bandwidthStopChan = make(chan struct{})
		cachedBandwidth = &bandwidthMeasurement{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but don't try to close channel - let StopBandwidthMonitoring handle it
					// This prevents double-close panics
				}
			}()

			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					bandwidth, err := measureBandwidth()
					if err == nil {
						bandwidthMutex.Lock()
						cachedBandwidth = bandwidth
						bandwidthMutex.Unlock()
					}
				case <-bandwidthStopChan:
					return
				}
			}
		}()
	})
}

// StopBandwidthMonitoring gracefully stops the bandwidth monitoring goroutine
// Thread-safe: Uses sync.Once to ensure channel is closed only once
func StopBandwidthMonitoring() {
	bandwidthStopOnce.Do(func() {
		if bandwidthStopChan != nil {
			close(bandwidthStopChan)
		}
	})
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

var prevNetIOCounters []net.IOCountersStat
var prevNetIOMutex sync.RWMutex

// measureBandwidth calculates bandwidth from delta between measurements
func measureBandwidth() (*bandwidthMeasurement, error) {
	currentIO, err := net.IOCounters(false)
	if err != nil {
		return nil, err
	}

	if len(currentIO) == 0 {
		return &bandwidthMeasurement{}, nil
	}

	prevNetIOMutex.Lock()
	defer prevNetIOMutex.Unlock()

	if len(prevNetIOCounters) == 0 {
		prevNetIOCounters = currentIO
		return &bandwidthMeasurement{}, nil
	}

	inboundBytes := int64(currentIO[0].BytesRecv - prevNetIOCounters[0].BytesRecv)
	outboundBytes := int64(currentIO[0].BytesSent - prevNetIOCounters[0].BytesSent)
	prevNetIOCounters = currentIO

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
// Memory optimized: keeps only top N in memory instead of all connections
func getTopConnections(connections []net.ConnectionStat, limit int) []NetworkConnection {
	// Keep only top N scored connections in memory (avoid allocating for all connections)
	topScored := make([]connectionWithScore, 0, limit+1)
	minScore := 0

	for _, conn := range connections {
		score := getConnectionScore(conn)
		if score == 0 {
			continue
		}

		// If we haven't filled the buffer yet, or this score beats the minimum
		if len(topScored) < limit || score > minScore {
			topScored = append(topScored, connectionWithScore{conn: conn, score: score})

			// If buffer exceeded, remove the lowest scored one
			if len(topScored) > limit {
				// Find and remove minimum
				minIdx := 0
				for i := 1; i < len(topScored); i++ {
					if topScored[i].score < topScored[minIdx].score {
						minIdx = i
					}
				}
				// Remove by swapping with last and truncating
				topScored[minIdx] = topScored[len(topScored)-1]
				topScored = topScored[:len(topScored)-1]

				// Update minScore
				minScore = topScored[0].score
				for i := 1; i < len(topScored); i++ {
					if topScored[i].score < minScore {
						minScore = topScored[i].score
					}
				}
			}
		}
	}

	// Sort the top N by score (descending)
	sort.Slice(topScored, func(i, j int) bool {
		return topScored[i].score > topScored[j].score
	})

	// Convert to NetworkConnection
	result := make([]NetworkConnection, 0, len(topScored))
	for _, scored := range topScored {
		conn := scored.conn

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
			ImportanceScore: scored.score,
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
