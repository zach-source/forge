package monitor

import (
	"testing"
	"time"

	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/worker"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"exactly10!", 10, "exactly10!"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestNewModel(t *testing.T) {
	m := NewModel("/tmp")

	if m.sessions == nil {
		t.Error("Expected sessions manager to be initialized")
	}
	if m.selected != 0 {
		t.Errorf("Expected selected to be 0, got %d", m.selected)
	}
	if m.workDir != "/tmp" {
		t.Errorf("Expected workDir to be /tmp, got %s", m.workDir)
	}
}

func TestComputeWorkerHealth_IdleWorker(t *testing.T) {
	workers := []*worker.Worker{
		{
			ID:         "w-test1234",
			Name:       "alpha",
			Status:     worker.StatusIdle,
			LastActive: time.Now().Add(-5 * time.Minute),
		},
	}
	sessions := []*session.Session{}

	health := computeWorkerHealth(workers, sessions)

	h, ok := health["w-test1234"]
	if !ok {
		t.Fatal("Expected health entry for worker")
	}

	if h.IsStuck {
		t.Error("Idle worker should not be marked as stuck")
	}
	if h.SessionActive {
		t.Error("Idle worker should not have active session")
	}
	if h.PromiseStatus != PromiseNone {
		t.Errorf("Expected PromiseNone, got %v", h.PromiseStatus)
	}
}

func TestComputeWorkerHealth_ActiveWorker(t *testing.T) {
	workers := []*worker.Worker{
		{
			ID:         "w-test5678",
			Name:       "bravo",
			Status:     worker.StatusActive,
			SessionID:  "forge-bravo-test5678",
			LastActive: time.Now().Add(-10 * time.Minute),
		},
	}
	sessions := []*session.Session{}

	health := computeWorkerHealth(workers, sessions)

	h, ok := health["w-test5678"]
	if !ok {
		t.Fatal("Expected health entry for worker")
	}

	// Active worker with recent activity should not be stuck
	if h.IsStuck {
		t.Error("Active worker with recent activity should not be stuck")
	}
}

func TestComputeWorkerHealth_StuckWorker(t *testing.T) {
	workers := []*worker.Worker{
		{
			ID:         "w-stuck123",
			Name:       "charlie",
			Status:     worker.StatusActive,
			SessionID:  "forge-charlie-stuck123",
			LastActive: time.Now().Add(-45 * time.Minute), // >30m = stuck
		},
	}
	sessions := []*session.Session{}

	health := computeWorkerHealth(workers, sessions)

	h, ok := health["w-stuck123"]
	if !ok {
		t.Fatal("Expected health entry for worker")
	}

	if !h.IsStuck {
		t.Error("Worker with >30m inactivity should be marked as stuck")
	}
	if h.StuckDuration < 30*time.Minute {
		t.Errorf("Expected stuck duration >30m, got %v", h.StuckDuration)
	}
}

func TestPromiseStatus_Constants(t *testing.T) {
	// Verify promise status constants are distinct
	if PromiseNone == PromisePending {
		t.Error("PromiseNone and PromisePending should be different")
	}
	if PromisePending == PromiseDetected {
		t.Error("PromisePending and PromiseDetected should be different")
	}
	if PromiseNone == PromiseDetected {
		t.Error("PromiseNone and PromiseDetected should be different")
	}
}

func TestStuckThreshold(t *testing.T) {
	if StuckThreshold != 30*time.Minute {
		t.Errorf("Expected stuck threshold to be 30m, got %v", StuckThreshold)
	}
}

func TestFormatDurationLong(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{5 * time.Minute, "5m"},
		{65 * time.Minute, "1h 5m"},
		{2 * time.Hour, "2h"},
		{25 * time.Hour, "1d 1h"},
		{48 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		got := formatDurationLong(tt.input)
		if got != tt.want {
			t.Errorf("formatDurationLong(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
