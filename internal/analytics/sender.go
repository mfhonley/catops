// Package analytics handles sending service events to the CatOps backend.
// Metrics are now sent via OpenTelemetry Collector - see otelcol package.
// Alerts are processed on the backend based on received metrics.
package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	constants "catops/config"
	"catops/internal/config"
	"catops/internal/logger"
	"catops/pkg/utils"
)

// Shared HTTP client
var sharedHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	},
}

// Sender handles sending events to the backend
type Sender struct {
	cfg        *config.Config
	version    string
	httpClient *http.Client
}

// NewSender creates a new analytics sender
func NewSender(cfg *config.Config, version string) *Sender {
	return &Sender{
		cfg:        cfg,
		version:    version,
		httpClient: sharedHTTPClient,
	}
}

// SendEvent sends a service event asynchronously (non-blocking)
func (s *Sender) SendEvent(eventType string) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("PANIC in SendEvent: %v", r)
			}
		}()

		s.sendEvent(eventType)
	}()
}

// SendEventSync sends a service event synchronously (blocking)
func (s *Sender) SendEventSync(eventType string) {
	if s.cfg.AuthToken == "" || s.cfg.ServerID == "" {
		return
	}

	s.sendEvent(eventType)
}

// sendEvent sends event to backend
func (s *Sender) sendEvent(eventType string) {
	eventData := s.buildEventData(eventType)
	jsonData, err := json.Marshal(eventData)
	if err != nil {
		logger.Error("Failed to marshal event data: %v", err)
		return
	}

	logger.Info("Sending event: %s", eventType)

	req, err := utils.CreateCLIRequest("POST", constants.EVENTS_URL, bytes.NewBuffer(jsonData), s.version)
	if err != nil {
		logger.Error("Failed to create event request: %v", err)
		return
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to send event: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		logger.Info("Event sent successfully: %s", eventType)
	} else {
		logger.Warning("Event response: HTTP %d", resp.StatusCode)
	}
}

// buildEventData creates event payload
func (s *Sender) buildEventData(eventType string) map[string]interface{} {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	pid := os.Getpid()

	// Map event types
	backendEventType := eventType
	switch eventType {
	case "service_start", "service_stop", "service_restart", "update_installed", "config_change":
		// Valid types
	default:
		backendEventType = "service_start"
	}

	// Determine severity
	severity := "info"
	if eventType == "service_stop" || eventType == "config_change" {
		severity = "warning"
	}

	// Create message
	messages := map[string]string{
		"service_start":    "CatOps monitoring started",
		"service_stop":     "CatOps monitoring stopped",
		"service_restart":  "CatOps restarted",
		"update_installed": "CatOps updated",
		"config_change":    "CatOps config changed",
	}
	message := messages[eventType]
	if message == "" {
		message = fmt.Sprintf("CatOps event: %s", eventType)
	}

	eventModel := map[string]interface{}{
		"timestamp":     time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"server_id":     s.cfg.ServerID,
		"event_type":    backendEventType,
		"service_name":  "catops",
		"process_name":  "catops",
		"pid":           pid,
		"message":       message,
		"severity":      severity,
		"error_message": nil,
		"tags": map[string]string{
			"hostname":       hostname,
			"catops_version": s.version,
		},
	}

	return map[string]interface{}{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().Unix()),
		"user_token": s.cfg.AuthToken,
		"events":     []map[string]interface{}{eventModel},
	}
}
