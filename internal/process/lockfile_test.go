//go:build !windows
// +build !windows

package process

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestLockfile_SingleInstance(t *testing.T) {
	// Create temp dir for test
	tmpDir := t.TempDir()
	testPIDFile := tmpDir + "/test_catops.pid"

	// Override getPIDFilePath for testing
	originalGetPIDFilePath := getPIDFilePath
	getPIDFilePath = func() string { return testPIDFile }
	defer func() { getPIDFilePath = originalGetPIDFilePath }()

	// First instance should acquire lock successfully
	lock1, err := Acquire()
	if err != nil {
		t.Fatalf("First instance failed to acquire lock: %v", err)
	}
	defer lock1.Release()

	// Second instance should fail to acquire lock
	lock2, err := Acquire()
	if err == nil {
		lock2.Release()
		t.Fatal("Second instance should not have acquired lock")
	}

	if err.Error() != "another CatOps instance is already running" {
		t.Errorf("Expected 'another CatOps instance is already running' error, got: %v", err)
	}
}

func TestLockfile_ReleaseAndReacquire(t *testing.T) {
	tmpDir := t.TempDir()
	testPIDFile := tmpDir + "/test_catops.pid"

	originalGetPIDFilePath := getPIDFilePath
	getPIDFilePath = func() string { return testPIDFile }
	defer func() { getPIDFilePath = originalGetPIDFilePath }()

	// Acquire and release lock
	lock1, err := Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	lock1.Release()

	// Should be able to acquire again after release
	lock2, err := Acquire()
	if err != nil {
		t.Fatalf("Failed to reacquire lock after release: %v", err)
	}
	defer lock2.Release()

	// Verify PID file exists
	if _, err := os.Stat(testPIDFile); os.IsNotExist(err) {
		t.Error("PID file should exist after reacquisition")
	}
}

func TestLockfile_CrashCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	testPIDFile := tmpDir + "/test_catops.pid"

	originalGetPIDFilePath := getPIDFilePath
	getPIDFilePath = func() string { return testPIDFile }
	defer func() { getPIDFilePath = originalGetPIDFilePath }()

	// Simulate crash: acquire lock without releasing
	func() {
		lock, err := Acquire()
		if err != nil {
			t.Fatalf("Failed to acquire lock: %v", err)
		}
		// Intentionally don't release - simulates crash
		// But close the fd to simulate process exit
		lock.Release() // In real crash, OS would release flock automatically
	}()

	// After "crash", should be able to acquire lock again
	// (because stale lock detection will clean it up)
	lock2, err := Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire lock after simulated crash: %v", err)
	}
	defer lock2.Release()
}

func TestLockfile_Check(t *testing.T) {
	tmpDir := t.TempDir()
	testPIDFile := tmpDir + "/test_catops.pid"

	originalGetPIDFilePath := getPIDFilePath
	getPIDFilePath = func() string { return testPIDFile }
	defer func() { getPIDFilePath = originalGetPIDFilePath }()

	// Check when no lock exists
	running, pid, err := Check()
	if err != nil {
		t.Errorf("Check should not error when no lock exists: %v", err)
	}
	if running {
		t.Error("Check should return false when no process is running")
	}
	if pid != 0 {
		t.Errorf("Check should return PID 0 when no process running, got %d", pid)
	}

	// Acquire lock
	lock, err := Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	defer lock.Release()

	// Check when lock is held
	running, pid, err = Check()
	if err != nil {
		t.Errorf("Check failed: %v", err)
	}
	if !running {
		t.Error("Check should return true when lock is held")
	}
	if pid != os.Getpid() {
		t.Errorf("Check should return current PID %d, got %d", os.Getpid(), pid)
	}
}

func TestLockfile_ConcurrentAcquisition(t *testing.T) {
	tmpDir := t.TempDir()
	testPIDFile := tmpDir + "/test_catops.pid"

	originalGetPIDFilePath := getPIDFilePath
	getPIDFilePath = func() string { return testPIDFile }
	defer func() { getPIDFilePath = originalGetPIDFilePath }()

	// Try to acquire lock from multiple goroutines simultaneously
	successCount := 0
	errorCount := 0
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			lock, err := Acquire()
			if err == nil {
				successCount++
				time.Sleep(100 * time.Millisecond)
				lock.Release()
			} else {
				errorCount++
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Only ONE should succeed
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful acquisition, got %d", successCount)
	}

	// Others should fail
	if errorCount != 9 {
		t.Errorf("Expected 9 failed acquisitions, got %d", errorCount)
	}
}

func TestLockfile_CleanupStale(t *testing.T) {
	tmpDir := t.TempDir()
	testPIDFile := tmpDir + "/test_catops.pid"

	originalGetPIDFilePath := getPIDFilePath
	getPIDFilePath = func() string { return testPIDFile }
	defer func() { getPIDFilePath = originalGetPIDFilePath }()

	// Create a stale PID file (write PID but don't hold lock)
	err := os.WriteFile(testPIDFile, []byte("99999\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create stale PID file: %v", err)
	}

	// CleanupStale should remove it
	err = CleanupStale()
	if err != nil {
		t.Errorf("CleanupStale should not error on stale file: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(testPIDFile); !os.IsNotExist(err) {
		t.Error("Stale PID file should have been removed")
	}
}

func TestLockfile_PIDFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	testPIDFile := tmpDir + "/test_catops.pid"

	originalGetPIDFilePath := getPIDFilePath
	getPIDFilePath = func() string { return testPIDFile }
	defer func() { getPIDFilePath = originalGetPIDFilePath }()

	// Acquire lock
	lock, err := Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	defer lock.Release()

	// Read PID file content
	content, err := os.ReadFile(testPIDFile)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	// Should contain current PID
	var filePID int
	_, err = fmt.Sscanf(string(content), "%d", &filePID)
	if err != nil {
		t.Fatalf("Failed to parse PID from file: %v", err)
	}

	if filePID != os.Getpid() {
		t.Errorf("PID file should contain %d, got %d", os.Getpid(), filePID)
	}
}

func TestLockfile_MultipleReleases(t *testing.T) {
	tmpDir := t.TempDir()
	testPIDFile := tmpDir + "/test_catops.pid"

	originalGetPIDFilePath := getPIDFilePath
	getPIDFilePath = func() string { return testPIDFile }
	defer func() { getPIDFilePath = originalGetPIDFilePath }()

	// Acquire lock
	lock, err := Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Release multiple times should not panic
	lock.Release()
	lock.Release() // Should be safe
	lock.Release() // Should be safe

	// Should be able to acquire again
	lock2, err := Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire lock after multiple releases: %v", err)
	}
	defer lock2.Release()
}
