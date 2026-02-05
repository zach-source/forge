package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/session"
)

// TestShowStatusSummaryEmpty tests status summary with no sessions.
func TestShowStatusSummaryEmpty(t *testing.T) {
	mgr := session.NewManager()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showStatusSummary(mgr)

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("showStatusSummary() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No forge sessions found") {
		t.Errorf("expected 'No forge sessions found' message, got: %s", output)
	}
}

// TestShowStatusSummaryWithSessions tests status summary with mock sessions.
func TestShowStatusSummaryWithSessions(t *testing.T) {
	mgr := session.NewManager()

	// Add mock session
	sess := &session.Session{
		ID:        "test-session-123",
		Status:    session.StatusActive,
		Tmux:      "test-session-123",
		StartedAt: time.Now().Add(-5 * time.Minute),
		State: &ralph.State{
			CompletionPromise: "DONE",
			Iteration:         5,
			MaxIterations:     50,
		},
	}
	mgr.Add(sess)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showStatusSummary(mgr)

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("showStatusSummary() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "test-session-123") {
		t.Errorf("expected session ID in output, got: %s", output)
	}
	if !strings.Contains(output, "DONE") {
		t.Errorf("expected promise in output, got: %s", output)
	}
	if !strings.Contains(output, "1 active / 1 total") {
		t.Errorf("expected summary line in output, got: %s", output)
	}
}

// TestShowStatusSummaryLongPromise tests truncation of long promises.
func TestShowStatusSummaryLongPromise(t *testing.T) {
	mgr := session.NewManager()

	longPromise := "This is a very long completion promise that should be truncated"
	sess := &session.Session{
		ID:        "test-session",
		Status:    session.StatusActive,
		StartedAt: time.Now(),
		State: &ralph.State{
			CompletionPromise: longPromise,
			Iteration:         1,
			MaxIterations:     10,
		},
	}
	mgr.Add(sess)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showStatusSummary(mgr)

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("showStatusSummary() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Long promises should be truncated with "..."
	if !strings.Contains(output, "...") {
		t.Errorf("expected truncated promise with '...' in output, got: %s", output)
	}
}

// TestShowDetailedStatusNotFound tests detailed status for non-existent session.
func TestShowDetailedStatusNotFound(t *testing.T) {
	mgr := session.NewManager()

	err := showDetailedStatus(mgr, "nonexistent-session")

	if err == nil {
		t.Error("expected error for non-existent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// TestShowDetailedStatusWithState tests detailed status with full state.
func TestShowDetailedStatusWithState(t *testing.T) {
	mgr := session.NewManager()

	sess := &session.Session{
		ID:        "detailed-test",
		Status:    session.StatusActive,
		Tmux:      "detailed-test",
		StartedAt: time.Now().Add(-10 * time.Minute),
		State: &ralph.State{
			CompletionPromise: "COMPLETE",
			Iteration:         7,
			MaxIterations:     50,
			WorkDir:           "/tmp/test",
			LogFile:           "/tmp/test.log",
		},
	}
	mgr.Add(sess)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showDetailedStatus(mgr, "detailed-test")

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("showDetailedStatus() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Verify detailed output contains expected elements
	expectedStrings := []string{
		"Session: detailed-test",
		"Status:",
		"Elapsed:",
		"Iteration:",
		"Promise:",
		"COMPLETE",
		"WorkDir:",
		"LogFile:",
		"Tmux Session:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected %q in output, got: %s", expected, output)
		}
	}
}

// TestShowDetailedStatusNoState tests detailed status without state.
func TestShowDetailedStatusNoState(t *testing.T) {
	mgr := session.NewManager()

	sess := &session.Session{
		ID:        "no-state-test",
		Status:    session.StatusCompleted,
		StartedAt: time.Now(),
	}
	mgr.Add(sess)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showDetailedStatus(mgr, "no-state-test")

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("showDetailedStatus() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should still show basic info
	if !strings.Contains(output, "Session: no-state-test") {
		t.Errorf("expected session ID in output, got: %s", output)
	}
}

// TestCancelSessionWithForce tests cancel with force flag.
func TestCancelSessionWithForce(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Create a fake state file
	sessionsDir := filepath.Join(tmpDir, ".forge", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("creating sessions dir: %v", err)
	}

	// We can't fully test without mocking, but we can test the force path
	// that skips confirmation
	fakeID := "test-cancel-" + filepath.Base(tmpDir)

	// Capture stdout to suppress output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cancelSession(fakeID, true)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should complete without error (tmux session won't exist)
	if err != nil {
		t.Errorf("cancelSession() error = %v", err)
	}

	// Should print cancellation message
	if !strings.Contains(output, "cancelled") {
		t.Errorf("expected 'cancelled' in output, got: %s", output)
	}
}
