package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zach-source/forge/internal/ralph"
)

// TestDefaultResumeConfig tests that default config has sensible values.
func TestDefaultResumeConfig(t *testing.T) {
	cfg := DefaultResumeConfig()

	if cfg.ContextLines != 50 {
		t.Errorf("ContextLines = %d, want 50", cfg.ContextLines)
	}
	if !cfg.SkipPermissions {
		t.Error("SkipPermissions should be true by default")
	}
	if cfg.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", cfg.Timeout)
	}
}

// TestReadLogContext tests reading context from log files.
func TestReadLogContext(t *testing.T) {
	// Create temp log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Write some test content
	lines := []string{
		"Line 1",
		"Line 2",
		"Line 3",
		"Line 4",
		"Line 5",
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing log file: %v", err)
	}

	tests := []struct {
		name     string
		n        int
		wantLen  int
		contains string
	}{
		{"last 2 lines", 2, 2, "Line 4"},
		{"last 3 lines", 3, 3, "Line 3"},
		{"all 5 lines", 5, 5, "Line 1"},
		{"more than available", 10, 5, "Line 1"},
		{"zero lines", 0, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := readLogContext(logPath, tt.n)

			if tt.n == 0 {
				if result != "" {
					t.Errorf("expected empty result for n=0, got %q", result)
				}
				return
			}

			gotLines := strings.Split(result, "\n")
			if len(gotLines) != tt.wantLen {
				t.Errorf("got %d lines, want %d", len(gotLines), tt.wantLen)
			}

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("expected result to contain %q, got %q", tt.contains, result)
			}
		})
	}
}

// TestReadLogContextNonExistent tests reading from non-existent file.
func TestReadLogContextNonExistent(t *testing.T) {
	result := readLogContext("/nonexistent/path/file.log", 10)
	if result != "" {
		t.Errorf("expected empty result for non-existent file, got %q", result)
	}
}

// TestReadLogContextEmptyFile tests reading from empty file.
func TestReadLogContextEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "empty.log")

	// Create empty file
	if err := os.WriteFile(logPath, []byte{}, 0o644); err != nil {
		t.Fatalf("creating empty file: %v", err)
	}

	result := readLogContext(logPath, 10)
	if result != "" {
		t.Errorf("expected empty result for empty file, got %q", result)
	}
}

// TestBuildResumePrompt tests the resume prompt building.
func TestBuildResumePrompt(t *testing.T) {
	a := &Agent{
		config: Config{
			CompletionPromise: "DONE",
		},
	}

	state := &ralph.State{
		Iteration:         5,
		CompletionPromise: "DONE",
		Prompt:            "Original task description",
	}

	tests := []struct {
		name       string
		logContext string
		contains   []string
	}{
		{
			name:       "with context",
			logContext: "Previous output here",
			contains: []string{
				"iteration 5",
				"resuming from a previous session",
				"Previous Session Context",
				"Previous output here",
				"Original Task",
				"Original task description",
				"<promise>DONE</promise>",
			},
		},
		{
			name:       "without context",
			logContext: "",
			contains: []string{
				"iteration 5",
				"resuming from a previous session",
				"Original Task",
				"Original task description",
				"<promise>DONE</promise>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := a.buildResumePrompt(state, tt.logContext)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected prompt to contain %q, got:\n%s", expected, result)
				}
			}

			if tt.logContext == "" && strings.Contains(result, "Previous Session Context") {
				t.Error("should not include context section when logContext is empty")
			}
		})
	}
}

// TestListResumableSessions tests listing resumable sessions.
func TestListResumableSessions(t *testing.T) {
	// Override ralph session directory to a temp dir
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("creating sessions dir: %v", err)
	}

	// Save original and restore after test
	origDir := ralph.DefaultSessionsDir
	ralph.DefaultSessionsDir = func() string { return sessionsDir }
	defer func() { ralph.DefaultSessionsDir = origDir }()

	// Create test state files
	states := []*ralph.State{
		{
			ID:                "session-1",
			Active:            false,
			Iteration:         5,
			CompletionPromise: "DONE",
			StartedAt:         time.Now(),
			WorkDir:           "/tmp",
			Prompt:            "Task 1",
		},
		{
			ID:                "session-2",
			Active:            false,
			Iteration:         10,
			CompletionPromise: "COMPLETE",
			StartedAt:         time.Now(),
			WorkDir:           "/tmp",
			Prompt:            "Task 2",
		},
	}

	for _, state := range states {
		statePath := filepath.Join(sessionsDir, state.ID+".state.md")
		ctrl := ralph.NewStateController(statePath)
		if err := ctrl.Write(state); err != nil {
			t.Fatalf("writing state %s: %v", state.ID, err)
		}
	}

	// Get resumable sessions
	resumable, err := ListResumableSessions()
	if err != nil {
		t.Fatalf("ListResumableSessions() error = %v", err)
	}

	if len(resumable) != 2 {
		t.Errorf("got %d resumable sessions, want 2", len(resumable))
	}

	// Verify both sessions are found
	found := make(map[string]bool)
	for _, s := range resumable {
		found[s.ID] = true
	}

	if !found["session-1"] {
		t.Error("session-1 not found in resumable sessions")
	}
	if !found["session-2"] {
		t.Error("session-2 not found in resumable sessions")
	}
}

// TestResumeNoStateFile tests that Resume returns error for missing state.
func TestResumeNoStateFile(t *testing.T) {
	// Override ralph session directory to a temp dir
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("creating sessions dir: %v", err)
	}

	origDir := ralph.DefaultSessionsDir
	ralph.DefaultSessionsDir = func() string { return sessionsDir }
	defer func() { ralph.DefaultSessionsDir = origDir }()

	cfg := DefaultResumeConfig()
	cfg.SessionID = "nonexistent-session"

	err := Resume(nil, cfg)
	if err != ErrNoStateFile {
		t.Errorf("Resume() error = %v, want ErrNoStateFile", err)
	}
}
