package alerts

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeCPU     AlertType = "cpu"
	AlertTypeMemory  AlertType = "memory"
	AlertTypeDisk    AlertType = "disk"
	AlertTypeProcess AlertType = "process"
	AlertTypeNetwork AlertType = "network"
)

// AlertSeverity represents alert severity level
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertSubType provides more specific alert classification
type AlertSubType string

const (
	SubTypeThreshold   AlertSubType = "threshold"
	SubTypeSuddenSpike AlertSubType = "sudden_spike"
	SubTypeGradualRise AlertSubType = "gradual_rise"
	SubTypeAnomaly     AlertSubType = "anomaly"
	SubTypeRecovery    AlertSubType = "recovery"
)

// Alert represents an alert to be sent
type Alert struct {
	Type        AlertType              `json:"type"`
	SubType     AlertSubType           `json:"sub_type"`
	Severity    AlertSeverity          `json:"severity"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Value       float64                `json:"value"`
	Threshold   float64                `json:"threshold,omitempty"`
	Fingerprint string                 `json:"fingerprint"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ActiveAlert represents an active (firing) alert
type ActiveAlert struct {
	Fingerprint string
	Alert       Alert
	FirstSeen   time.Time
	LastSeen    time.Time
	Count       int
	Notified    bool
}

// AlertManager handles alert deduplication and tracking
type AlertManager struct {
	active            map[string]*ActiveAlert
	mutex             sync.RWMutex
	renotifyInterval  time.Duration // How often to re-send still-active alerts
	resolutionTimeout time.Duration // How long to wait before considering alert resolved
	maxAlerts         int           // Maximum number of active alerts (prevents memory leak)
}

// NotificationDecision indicates whether to send notification and why
type NotificationDecision struct {
	ShouldNotify bool
	Reason       string // "new", "renotify", "duplicate", "resolved"
	Notification string // Formatted notification message
	Alert        *ActiveAlert
}

// NewAlertManager creates a new alert manager
func NewAlertManager(renotifyInterval, resolutionTimeout time.Duration) *AlertManager {
	return &AlertManager{
		active:            make(map[string]*ActiveAlert),
		renotifyInterval:  renotifyInterval,
		resolutionTimeout: resolutionTimeout,
		maxAlerts:         100, // Limit to 100 active alerts to prevent memory leak
	}
}

// ProcessAlert processes an incoming alert and returns notification decision
func (am *AlertManager) ProcessAlert(alert Alert) NotificationDecision {
	fingerprint := am.generateFingerprint(alert)
	alert.Fingerprint = fingerprint

	am.mutex.Lock()
	defer am.mutex.Unlock()

	existing, exists := am.active[fingerprint]

	if !exists {
		// Check if we've hit the max alerts limit (memory leak prevention)
		if len(am.active) >= am.maxAlerts {
			// Clear oldest alerts to make room (FIFO eviction)
			oldestFingerprint := ""
			oldestTime := time.Now()
			for fp, a := range am.active {
				if a.FirstSeen.Before(oldestTime) {
					oldestTime = a.FirstSeen
					oldestFingerprint = fp
				}
			}
			if oldestFingerprint != "" {
				delete(am.active, oldestFingerprint)
			}
		}

		// New alert
		activeAlert := &ActiveAlert{
			Fingerprint: fingerprint,
			Alert:       alert,
			FirstSeen:   time.Now(),
			LastSeen:    time.Now(),
			Count:       1,
			Notified:    true,
		}
		am.active[fingerprint] = activeAlert

		return NotificationDecision{
			ShouldNotify: true,
			Reason:       "new",
			Notification: am.formatAlert(activeAlert, "new"),
			Alert:        activeAlert,
		}
	}

	// Existing alert - update
	existing.LastSeen = time.Now()
	existing.Count++
	existing.Alert = alert // Update with latest values

	// Check if we should re-notify
	timeSinceFirstSeen := time.Since(existing.FirstSeen)
	if timeSinceFirstSeen >= am.renotifyInterval {
		// Re-notify for still-active alert
		existing.FirstSeen = time.Now() // Reset timer
		existing.Notified = true

		return NotificationDecision{
			ShouldNotify: true,
			Reason:       "renotify",
			Notification: am.formatAlert(existing, "ongoing"),
			Alert:        existing,
		}
	}

	// Suppress duplicate
	return NotificationDecision{
		ShouldNotify: false,
		Reason:       "duplicate",
		Alert:        existing,
	}
}

// CheckResolved checks if an alert should be marked as resolved
func (am *AlertManager) CheckResolved(alertType AlertType, subType AlertSubType) *NotificationDecision {
	fingerprint := am.generateFingerprintFromType(alertType, subType)

	am.mutex.Lock()
	defer am.mutex.Unlock()

	alert, exists := am.active[fingerprint]
	if !exists {
		return nil // No active alert to resolve
	}

	// Check if enough time has passed since last seen
	if time.Since(alert.LastSeen) < am.resolutionTimeout {
		return nil // Too soon to declare resolved
	}

	// Mark as resolved
	notification := am.formatAlert(alert, "resolved")

	// Remove from active alerts
	delete(am.active, fingerprint)

	return &NotificationDecision{
		ShouldNotify: true,
		Reason:       "resolved",
		Notification: notification,
		Alert:        alert,
	}
}

// GetActiveAlerts returns all currently active alerts
func (am *AlertManager) GetActiveAlerts() []*ActiveAlert {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	alerts := make([]*ActiveAlert, 0, len(am.active))
	for _, alert := range am.active {
		alerts = append(alerts, alert)
	}
	return alerts
}

// GetActiveCount returns count of active alerts
func (am *AlertManager) GetActiveCount() int {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	return len(am.active)
}

// ClearResolved removes alerts that haven't been seen in a while
func (am *AlertManager) ClearResolved() int {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	removed := 0
	for fingerprint, alert := range am.active {
		if time.Since(alert.LastSeen) > am.resolutionTimeout {
			delete(am.active, fingerprint)
			removed++
		}
	}
	return removed
}

// generateFingerprint creates a unique fingerprint for an alert
func (am *AlertManager) generateFingerprint(alert Alert) string {
	return am.generateFingerprintFromType(alert.Type, alert.SubType)
}

// generateFingerprintFromType creates fingerprint from type and subtype
func (am *AlertManager) generateFingerprintFromType(alertType AlertType, subType AlertSubType) string {
	data := fmt.Sprintf("%s:%s", alertType, subType)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

// formatAlert formats an alert for notification
func (am *AlertManager) formatAlert(alert *ActiveAlert, status string) string {
	emoji := am.getSeverityEmoji(alert.Alert.Severity)

	switch status {
	case "new":
		return fmt.Sprintf("%s %s\n\n%s\n\n⏰ %s",
			emoji,
			alert.Alert.Title,
			alert.Alert.Message,
			alert.FirstSeen.Format("2006-01-02 15:04:05"))

	case "ongoing":
		duration := time.Since(alert.FirstSeen)
		return fmt.Sprintf("%s STILL ACTIVE: %s\n\n%s\n\n⏱ Duration: %s\n🔢 Fired: %d times",
			emoji,
			alert.Alert.Title,
			alert.Alert.Message,
			am.formatDuration(duration),
			alert.Count)

	case "resolved":
		duration := time.Since(alert.FirstSeen)
		return fmt.Sprintf("✅ RESOLVED: %s\n\n%s\n\n⏱ Duration: %s\n🔢 Fired: %d times",
			alert.Alert.Title,
			alert.Alert.Message,
			am.formatDuration(duration),
			alert.Count)

	default:
		return alert.Alert.Message
	}
}

// getSeverityEmoji returns emoji for severity level
func (am *AlertManager) getSeverityEmoji(severity AlertSeverity) string {
	switch severity {
	case SeverityCritical:
		return "🔴"
	case SeverityWarning:
		return "🟡"
	case SeverityInfo:
		return "🔵"
	default:
		return "⚪"
	}
}

// formatDuration formats duration in human-readable format
func (am *AlertManager) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// GroupAlerts groups multiple alerts into a single notification
func GroupAlerts(alerts []*ActiveAlert) string {
	if len(alerts) == 0 {
		return ""
	}

	if len(alerts) == 1 {
		am := NewAlertManager(time.Hour, 2*time.Minute)
		return am.formatAlert(alerts[0], "new")
	}

	// Count by severity
	critical := 0
	warning := 0
	info := 0

	for _, a := range alerts {
		switch a.Alert.Severity {
		case SeverityCritical:
			critical++
		case SeverityWarning:
			warning++
		case SeverityInfo:
			info++
		}
	}

	message := "🚨 MULTIPLE ALERTS\n\n"

	if critical > 0 {
		message += fmt.Sprintf("🔴 Critical: %d\n", critical)
	}
	if warning > 0 {
		message += fmt.Sprintf("🟡 Warning: %d\n", warning)
	}
	if info > 0 {
		message += fmt.Sprintf("🔵 Info: %d\n", info)
	}

	message += "\nDetails:\n"
	for _, a := range alerts {
		emoji := "⚪"
		switch a.Alert.Severity {
		case SeverityCritical:
			emoji = "🔴"
		case SeverityWarning:
			emoji = "🟡"
		case SeverityInfo:
			emoji = "🔵"
		}
		message += fmt.Sprintf("%s %s\n", emoji, a.Alert.Title)
	}

	return message
}

// CreateAlert is a helper function to create an alert
func CreateAlert(alertType AlertType, subType AlertSubType, severity AlertSeverity, title, message string, value, threshold float64, details map[string]interface{}) Alert {
	return Alert{
		Type:      alertType,
		SubType:   subType,
		Severity:  severity,
		Title:     title,
		Message:   message,
		Value:     value,
		Threshold: threshold,
		Details:   details,
		Timestamp: time.Now(),
	}
}
