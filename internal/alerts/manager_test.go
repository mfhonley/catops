package alerts

import (
	"strings"
	"testing"
	"time"
)

func TestAlertManager_Deduplication(t *testing.T) {
	am := NewAlertManager(1*time.Hour, 2*time.Minute)

	alert := CreateAlert(
		AlertTypeCPU,
		SubTypeThreshold,
		SeverityWarning,
		"CPU High",
		"CPU usage is 85%",
		85.0,
		80.0,
		nil,
	)

	// First alert should notify
	decision1 := am.ProcessAlert(alert)
	if !decision1.ShouldNotify {
		t.Error("First alert should trigger notification")
	}
	if decision1.Reason != "new" {
		t.Errorf("Expected reason 'new', got '%s'", decision1.Reason)
	}

	// Second alert (duplicate) should NOT notify
	decision2 := am.ProcessAlert(alert)
	if decision2.ShouldNotify {
		t.Error("Duplicate alert should NOT trigger notification")
	}
	if decision2.Reason != "duplicate" {
		t.Errorf("Expected reason 'duplicate', got '%s'", decision2.Reason)
	}

	// Third alert (still duplicate) should NOT notify
	decision3 := am.ProcessAlert(alert)
	if decision3.ShouldNotify {
		t.Error("Duplicate alert should NOT trigger notification")
	}
}

func TestAlertManager_Renotify(t *testing.T) {
	am := NewAlertManager(100*time.Millisecond, 2*time.Minute) // Short interval for testing

	alert := CreateAlert(
		AlertTypeCPU,
		SubTypeThreshold,
		SeverityWarning,
		"CPU High",
		"CPU usage is 85%",
		85.0,
		80.0,
		nil,
	)

	// Fire initial alert
	decision1 := am.ProcessAlert(alert)
	if !decision1.ShouldNotify || decision1.Reason != "new" {
		t.Fatal("First alert should notify with reason 'new'")
	}

	// Wait for renotify interval
	time.Sleep(150 * time.Millisecond)

	// Fire alert again (should re-notify after interval)
	decision2 := am.ProcessAlert(alert)
	if !decision2.ShouldNotify {
		t.Error("Should re-notify after interval")
	}
	if decision2.Reason != "renotify" {
		t.Errorf("Expected reason 'renotify', got '%s'", decision2.Reason)
	}

	if !strings.Contains(decision2.Notification, "STILL ACTIVE") {
		t.Error("Renotify notification should contain 'STILL ACTIVE'")
	}
}

func TestAlertManager_Resolution(t *testing.T) {
	am := NewAlertManager(1*time.Hour, 100*time.Millisecond) // Short timeout for testing

	alert := CreateAlert(
		AlertTypeCPU,
		SubTypeThreshold,
		SeverityWarning,
		"CPU High",
		"CPU usage is 85%",
		85.0,
		80.0,
		nil,
	)

	// Fire alert
	decision1 := am.ProcessAlert(alert)
	if !decision1.ShouldNotify {
		t.Fatal("First alert should notify")
	}

	// Wait for resolution timeout
	time.Sleep(150 * time.Millisecond)

	// Check resolution
	resolution := am.CheckResolved(AlertTypeCPU, SubTypeThreshold)

	if resolution == nil {
		t.Fatal("Alert should be resolved")
	}

	if !resolution.ShouldNotify {
		t.Error("Resolution should trigger notification")
	}

	if resolution.Reason != "resolved" {
		t.Errorf("Expected reason 'resolved', got '%s'", resolution.Reason)
	}

	if !strings.Contains(resolution.Notification, "RESOLVED") {
		t.Error("Resolution notification should contain 'RESOLVED'")
	}

	// Verify alert was removed from active list
	if am.GetActiveCount() != 0 {
		t.Error("Resolved alert should be removed from active list")
	}
}

func TestAlertManager_MultipleAlertTypes(t *testing.T) {
	am := NewAlertManager(1*time.Hour, 2*time.Minute)

	cpuAlert := CreateAlert(
		AlertTypeCPU,
		SubTypeThreshold,
		SeverityWarning,
		"CPU High",
		"CPU usage is 85%",
		85.0,
		80.0,
		nil,
	)

	memoryAlert := CreateAlert(
		AlertTypeMemory,
		SubTypeThreshold,
		SeverityCritical,
		"Memory High",
		"Memory usage is 95%",
		95.0,
		90.0,
		nil,
	)

	// Fire both alerts
	decision1 := am.ProcessAlert(cpuAlert)
	decision2 := am.ProcessAlert(memoryAlert)

	if !decision1.ShouldNotify || !decision2.ShouldNotify {
		t.Error("Both alerts should trigger notifications")
	}

	// Verify both are active
	if am.GetActiveCount() != 2 {
		t.Errorf("Expected 2 active alerts, got %d", am.GetActiveCount())
	}

	// Fire CPU alert again (should be duplicate)
	decision3 := am.ProcessAlert(cpuAlert)
	if decision3.ShouldNotify {
		t.Error("CPU alert should be deduplicated")
	}

	// Fire memory alert again (should be duplicate)
	decision4 := am.ProcessAlert(memoryAlert)
	if decision4.ShouldNotify {
		t.Error("Memory alert should be deduplicated")
	}
}

func TestAlertManager_DifferentSubTypes(t *testing.T) {
	am := NewAlertManager(1*time.Hour, 2*time.Minute)

	thresholdAlert := CreateAlert(
		AlertTypeCPU,
		SubTypeThreshold,
		SeverityWarning,
		"CPU High",
		"CPU usage is 85%",
		85.0,
		80.0,
		nil,
	)

	spikeAlert := CreateAlert(
		AlertTypeCPU,
		SubTypeSuddenSpike,
		SeverityCritical,
		"CPU Spike",
		"CPU jumped from 30% to 92%",
		92.0,
		0,
		nil,
	)

	// Fire both (different subtypes, should both notify)
	decision1 := am.ProcessAlert(thresholdAlert)
	decision2 := am.ProcessAlert(spikeAlert)

	if !decision1.ShouldNotify {
		t.Error("Threshold alert should notify")
	}

	if !decision2.ShouldNotify {
		t.Error("Spike alert should notify (different subtype)")
	}

	// Verify both are active (different fingerprints)
	if am.GetActiveCount() != 2 {
		t.Errorf("Expected 2 active alerts (different subtypes), got %d", am.GetActiveCount())
	}
}

func TestAlertManager_AlertCount(t *testing.T) {
	am := NewAlertManager(1*time.Hour, 2*time.Minute)

	alert := CreateAlert(
		AlertTypeCPU,
		SubTypeThreshold,
		SeverityWarning,
		"CPU High",
		"CPU usage is 85%",
		85.0,
		80.0,
		nil,
	)

	// Fire alert multiple times
	am.ProcessAlert(alert)
	am.ProcessAlert(alert)
	am.ProcessAlert(alert)

	activeAlerts := am.GetActiveAlerts()
	if len(activeAlerts) != 1 {
		t.Fatalf("Expected 1 active alert, got %d", len(activeAlerts))
	}

	if activeAlerts[0].Count != 3 {
		t.Errorf("Expected count=3, got %d", activeAlerts[0].Count)
	}
}

func TestAlertManager_ClearResolved(t *testing.T) {
	am := NewAlertManager(1*time.Hour, 50*time.Millisecond) // Short timeout

	alert := CreateAlert(
		AlertTypeCPU,
		SubTypeThreshold,
		SeverityWarning,
		"CPU High",
		"CPU usage is 85%",
		85.0,
		80.0,
		nil,
	)

	// Fire alert
	am.ProcessAlert(alert)

	// Verify it's active
	if am.GetActiveCount() != 1 {
		t.Fatal("Expected 1 active alert")
	}

	// Wait for resolution timeout
	time.Sleep(100 * time.Millisecond)

	// Clear resolved
	removed := am.ClearResolved()

	if removed != 1 {
		t.Errorf("Expected to remove 1 alert, removed %d", removed)
	}

	if am.GetActiveCount() != 0 {
		t.Error("All resolved alerts should be cleared")
	}
}

func TestGroupAlerts(t *testing.T) {
	alerts := []*ActiveAlert{
		{
			Alert: Alert{
				Type:     AlertTypeCPU,
				Severity: SeverityCritical,
				Title:    "CPU High on server-1",
			},
		},
		{
			Alert: Alert{
				Type:     AlertTypeMemory,
				Severity: SeverityWarning,
				Title:    "Memory High on server-2",
			},
		},
		{
			Alert: Alert{
				Type:     AlertTypeDisk,
				Severity: SeverityWarning,
				Title:    "Disk High on server-3",
			},
		},
	}

	grouped := GroupAlerts(alerts)

	if !strings.Contains(grouped, "MULTIPLE ALERTS") {
		t.Error("Grouped message should contain 'MULTIPLE ALERTS'")
	}

	if !strings.Contains(grouped, "Critical: 1") {
		t.Error("Grouped message should show critical count")
	}

	if !strings.Contains(grouped, "Warning: 2") {
		t.Error("Grouped message should show warning count")
	}

	if !strings.Contains(grouped, "CPU High") {
		t.Error("Grouped message should list individual alerts")
	}
}

func TestCreateAlert(t *testing.T) {
	alert := CreateAlert(
		AlertTypeCPU,
		SubTypeSuddenSpike,
		SeverityCritical,
		"CPU Spike Detected",
		"CPU jumped from 30% to 95%",
		95.0,
		80.0,
		map[string]interface{}{
			"previous_value": 30.0,
			"change_percent": 65.0,
		},
	)

	if alert.Type != AlertTypeCPU {
		t.Errorf("Expected type CPU, got %s", alert.Type)
	}

	if alert.SubType != SubTypeSuddenSpike {
		t.Errorf("Expected subtype sudden_spike, got %s", alert.SubType)
	}

	if alert.Severity != SeverityCritical {
		t.Errorf("Expected severity critical, got %s", alert.Severity)
	}

	if alert.Value != 95.0 {
		t.Errorf("Expected value 95.0, got %.1f", alert.Value)
	}

	if alert.Details["change_percent"] != 65.0 {
		t.Error("Details not properly set")
	}
}

func TestAlertManager_EmptyFingerprint(t *testing.T) {
	am := NewAlertManager(1*time.Hour, 2*time.Minute)

	// Alerts with same type/subtype should have same fingerprint
	alert1 := CreateAlert(
		AlertTypeCPU,
		SubTypeThreshold,
		SeverityWarning,
		"CPU High",
		"CPU usage is 85%",
		85.0,
		80.0,
		nil,
	)

	alert2 := CreateAlert(
		AlertTypeCPU,
		SubTypeThreshold,
		SeverityWarning,
		"CPU High",
		"CPU usage is 87%", // Different value
		87.0,
		80.0,
		nil,
	)

	decision1 := am.ProcessAlert(alert1)
	decision2 := am.ProcessAlert(alert2)

	if !decision1.ShouldNotify {
		t.Error("First alert should notify")
	}

	if decision2.ShouldNotify {
		t.Error("Second alert should be deduplicated (same fingerprint)")
	}

	// Verify only 1 active (they have same fingerprint)
	if am.GetActiveCount() != 1 {
		t.Errorf("Expected 1 active alert (same fingerprint), got %d", am.GetActiveCount())
	}
}
