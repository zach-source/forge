package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zach-source/forge/internal/ralph"
)

// TestNewResumeCmd tests that the resume command is created correctly.
func TestNewResumeCmd(t *testing.T) {
	cmd := newResumeCmd()

	if cmd == nil {
		t.Fatal("newResumeCmd() returned nil")
	}

	if cmd.Use != "resume [session-id]" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "resume [session-id]")
	}

	if cmd.Short == "" {
		t.Error("cmd.Short should not be empty")
	}

	if cmd.Long == "" {
		t.Error("cmd.Long should not be empty")
	}
}

// TestResumeCmdFlags tests that all expected flags are defined.
func TestResumeCmdFlags(t *testing.T) {
	cmd := newResumeCmd()

	tests := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"list", "l", "false"},
		{"mcp", "", ""},
		{"mcp-config", "", ""},
		{"no-skip", "", "false"},
		{"context", "c", "50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			if flag == nil {
				t.Errorf("%s flag should exist", tt.name)
				return
			}
			if tt.shorthand != "" && flag.Shorthand != tt.shorthand {
				t.Errorf("%s flag shorthand = %q, want %q", tt.name, flag.Shorthand, tt.shorthand)
			}
			if flag.DefValue != tt.defValue {
				t.Errorf("%s flag default = %q, want %q", tt.name, flag.DefValue, tt.defValue)
			}
		})
	}
}

// TestResumeCmdArgsValidation tests that the command accepts 0 or 1 argument.
func TestResumeCmdArgsValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, false},
		{"one arg", []string{"session-id"}, false},
		{"two args", []string{"arg1", "arg2"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newResumeCmd()
			cmd.SetArgs(tt.args)

			err := cmd.Args(cmd, tt.args)

			if tt.wantErr && err == nil {
				t.Error("expected args validation error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected args validation error: %v", err)
			}
		})
	}
}

// TestTruncateString tests the string truncation helper.
func TestTruncateString(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"abc", 5, "abc"},
		{"hello world", 8, "hello..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// TestShowResumableSessionsEmpty tests showing resumable sessions when none exist.
func TestShowResumableSessionsEmpty(t *testing.T) {
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

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showResumableSessions()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("showResumableSessions() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No resumable sessions found") {
		t.Errorf("expected 'No resumable sessions found' message, got: %s", output)
	}
}

// TestShowResumableSessionsWithSessions tests showing resumable sessions.
func TestShowResumableSessionsWithSessions(t *testing.T) {
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

	// Create a test state file
	state := &ralph.State{
		ID:                "test-resume-123",
		Active:            false,
		Iteration:         5,
		MaxIterations:     50,
		CompletionPromise: "DONE",
		StartedAt:         time.Now().Add(-10 * time.Minute),
		WorkDir:           "/tmp/test",
		LogFile:           "/tmp/test.log",
		Prompt:            "Test task",
	}

	statePath := filepath.Join(sessionsDir, "test-resume-123.state.md")
	ctrl := ralph.NewStateController(statePath)
	if err := ctrl.Write(state); err != nil {
		t.Fatalf("writing state: %v", err)
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showResumableSessions()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("showResumableSessions() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	expectedStrings := []string{
		"test-resume-123",
		"DONE",
		"5/50",
		"1 resumable session",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected %q in output, got: %s", expected, output)
		}
	}
}
