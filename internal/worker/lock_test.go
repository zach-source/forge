package worker

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLock(t *testing.T) {
	// Create temp directory for locks
	tmpDir, err := os.MkdirTemp("", "forge-lock-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Override lock path for testing
	SetLocksDir(tmpDir)
	defer SetLocksDir("")

	// Test acquire
	err = AcquireLock("merge", "worker-1")
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}

	// Test IsLocked
	locked, holder := IsLocked("merge")
	if !locked {
		t.Errorf("IsLocked() = false, want true")
	}
	if holder != "worker-1" {
		t.Errorf("IsLocked() holder = %q, want %q", holder, "worker-1")
	}

	// Test acquire by same worker (should succeed)
	err = AcquireLock("merge", "worker-1")
	if err != nil {
		t.Errorf("AcquireLock() by same worker should succeed, got error = %v", err)
	}

	// Test acquire by different worker (should fail)
	err = AcquireLock("merge", "worker-2")
	if err == nil {
		t.Errorf("AcquireLock() by different worker should fail")
	}

	// Test GetLock
	lock := GetLock("merge")
	if lock == nil {
		t.Fatalf("GetLock() returned nil")
	}
	if lock.Resource != "merge" {
		t.Errorf("GetLock().Resource = %q, want %q", lock.Resource, "merge")
	}
	if lock.HolderID != "worker-1" {
		t.Errorf("GetLock().HolderID = %q, want %q", lock.HolderID, "worker-1")
	}

	// Test release by wrong worker (should fail)
	err = ReleaseLock("merge", "worker-2")
	if err == nil {
		t.Errorf("ReleaseLock() by wrong worker should fail")
	}

	// Test release by correct worker
	err = ReleaseLock("merge", "worker-1")
	if err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}

	locked, _ = IsLocked("merge")
	if locked {
		t.Errorf("IsLocked() after release = true, want false")
	}

	// Test ListLocks
	_ = AcquireLock("merge", "worker-1")
	_ = AcquireLock("deploy", "worker-2")

	locks, err := ListLocks()
	if err != nil {
		t.Fatalf("ListLocks() error = %v", err)
	}
	if len(locks) != 2 {
		t.Errorf("ListLocks() length = %d, want 2", len(locks))
	}

	// Test ForceReleaseLock
	err = ForceReleaseLock("merge")
	if err != nil {
		t.Fatalf("ForceReleaseLock() error = %v", err)
	}

	locked, _ = IsLocked("merge")
	if locked {
		t.Errorf("IsLocked() after force release = true, want false")
	}
}

func TestCleanStaleLocks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-lock-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	SetLocksDir(tmpDir)
	defer SetLocksDir("")

	// Create a stale lock by writing directly with old timestamp
	lock := &Lock{
		Resource:   "stale",
		HolderID:   "worker-1",
		AcquiredAt: time.Now().Add(-2 * time.Hour),
	}
	_ = writeLock(filepath.Join(tmpDir, "stale.lock"), lock)

	// Create a fresh lock
	_ = AcquireLock("fresh", "worker-2")

	// Clean locks older than 1 hour
	err = CleanStaleLocks(1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanStaleLocks() error = %v", err)
	}

	// Stale lock should be gone
	if locked, _ := IsLocked("stale"); locked {
		t.Errorf("Stale lock should be cleaned")
	}

	// Fresh lock should remain
	if locked, _ := IsLocked("fresh"); !locked {
		t.Errorf("Fresh lock should remain")
	}
}

func TestValidResource(t *testing.T) {
	tests := []struct {
		resource string
		want     bool
	}{
		{"merge", true},
		{"deploy", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			got := ValidResource(tt.resource)
			if got != tt.want {
				t.Errorf("ValidResource(%q) = %v, want %v", tt.resource, got, tt.want)
			}
		})
	}
}
