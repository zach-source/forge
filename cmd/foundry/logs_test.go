package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zach-source/forge/internal/logs"
)

// setupTestLogs creates a temporary log directory with test log files.
func setupTestLogs(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "foundry-logs-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}

	// Create log directory structure
	dirs := []string{
		filepath.Join(tmpDir, "sessions"),
		filepath.Join(tmpDir, "workers"),
		filepath.Join(tmpDir, "leaders"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			os.RemoveAll(tmpDir)
			t.Fatalf("creating dir %s: %v", dir, err)
		}
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// createTestLog creates a test log file with the given content and modification time.
func createTestLog(t *testing.T, path string, content string, modTime time.Time) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test log %s: %v", path, err)
	}
	if !modTime.IsZero() {
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("setting mod time for %s: %v", path, err)
		}
	}
}

// TestNewLogsCmd tests that the logs command is created correctly.
func TestNewLogsCmd(t *testing.T) {
	cmd := newLogsCmd()

	if cmd == nil {
		t.Fatal("newLogsCmd() returned nil")
	}

	if cmd.Use != "logs [session-id]" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "logs [session-id]")
	}

	if cmd.Short == "" {
		t.Error("cmd.Short should not be empty")
	}

	// Check flags exist
	flags := []string{"type", "clean", "age", "dry-run", "tail", "all"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("flag %q should exist", flag)
		}
	}
}

// TestRunLogsList tests the runLogsList function.
func TestRunLogsList(t *testing.T) {
	origLogDir := logs.LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	logs.LogDir = func() string { return tmpDir }
	defer func() { logs.LogDir = origLogDir }()

	// Create test logs
	now := time.Now()
	createTestLog(t, filepath.Join(tmpDir, "sessions", "session1.log"), "session content", now)
	createTestLog(t, filepath.Join(tmpDir, "workers", "alpha.log"), "worker content", now)
	createTestLog(t, filepath.Join(tmpDir, "leaders", "planner.log"), "leader content", now)

	t.Run("list all", func(t *testing.T) {
		err := runLogsList("", false)
		if err != nil {
			t.Fatalf("runLogsList() error = %v", err)
		}
	})

	t.Run("filter by session type", func(t *testing.T) {
		err := runLogsList("session", false)
		if err != nil {
			t.Fatalf("runLogsList(session) error = %v", err)
		}
	})

	t.Run("filter by sessions plural", func(t *testing.T) {
		err := runLogsList("sessions", false)
		if err != nil {
			t.Fatalf("runLogsList(sessions) error = %v", err)
		}
	})

	t.Run("filter by worker type", func(t *testing.T) {
		err := runLogsList("worker", false)
		if err != nil {
			t.Fatalf("runLogsList(worker) error = %v", err)
		}
	})

	t.Run("filter by workers plural", func(t *testing.T) {
		err := runLogsList("workers", false)
		if err != nil {
			t.Fatalf("runLogsList(workers) error = %v", err)
		}
	})

	t.Run("filter by leader type", func(t *testing.T) {
		err := runLogsList("leader", false)
		if err != nil {
			t.Fatalf("runLogsList(leader) error = %v", err)
		}
	})

	t.Run("filter by leaders plural", func(t *testing.T) {
		err := runLogsList("leaders", false)
		if err != nil {
			t.Fatalf("runLogsList(leaders) error = %v", err)
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		err := runLogsList("invalid", false)
		if err == nil {
			t.Error("runLogsList(invalid) should return error")
		}
		if !strings.Contains(err.Error(), "invalid log type") {
			t.Errorf("error should mention invalid log type, got: %v", err)
		}
	})
}

// TestRunLogsListEmpty tests runLogsList with no logs.
func TestRunLogsListEmpty(t *testing.T) {
	origLogDir := logs.LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	logs.LogDir = func() string { return tmpDir }
	defer func() { logs.LogDir = origLogDir }()

	// No logs created
	err := runLogsList("", false)
	if err != nil {
		t.Fatalf("runLogsList() error = %v", err)
	}
}

// TestRunLogsView tests the runLogsView function.
func TestRunLogsView(t *testing.T) {
	origLogDir := logs.LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	logs.LogDir = func() string { return tmpDir }
	defer func() { logs.LogDir = origLogDir }()

	// Create test log
	logContent := "line1\nline2\nline3\nline4\nline5\n"
	createTestLog(t, filepath.Join(tmpDir, "sessions", "test-session.log"), logContent, time.Time{})

	t.Run("view full log", func(t *testing.T) {
		err := runLogsView("test-session", 0)
		if err != nil {
			t.Fatalf("runLogsView() error = %v", err)
		}
	})

	t.Run("view with tail", func(t *testing.T) {
		err := runLogsView("test-session", 2)
		if err != nil {
			t.Fatalf("runLogsView(tail=2) error = %v", err)
		}
	})

	t.Run("view non-existent", func(t *testing.T) {
		err := runLogsView("nonexistent", 0)
		if err == nil {
			t.Error("runLogsView(nonexistent) should return error")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error should mention not found, got: %v", err)
		}
	})
}

// TestRunLogsViewWorker tests viewing worker logs.
func TestRunLogsViewWorker(t *testing.T) {
	origLogDir := logs.LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	logs.LogDir = func() string { return tmpDir }
	defer func() { logs.LogDir = origLogDir }()

	// Create test log
	logContent := "worker log content"
	createTestLog(t, filepath.Join(tmpDir, "workers", "alpha.log"), logContent, time.Time{})

	err := runLogsView("alpha", 0)
	if err != nil {
		t.Fatalf("runLogsView(alpha) error = %v", err)
	}
}

// TestRunLogsViewLeader tests viewing leader logs.
func TestRunLogsViewLeader(t *testing.T) {
	origLogDir := logs.LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	logs.LogDir = func() string { return tmpDir }
	defer func() { logs.LogDir = origLogDir }()

	// Create test log
	logContent := "leader log content"
	createTestLog(t, filepath.Join(tmpDir, "leaders", "planner.log"), logContent, time.Time{})

	err := runLogsView("planner", 0)
	if err != nil {
		t.Fatalf("runLogsView(planner) error = %v", err)
	}
}

// TestRunLogsViewDirectPath tests viewing logs by direct path.
func TestRunLogsViewDirectPath(t *testing.T) {
	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	// Create test log
	logPath := filepath.Join(tmpDir, "direct.log")
	logContent := "direct log content"
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("writing test log: %v", err)
	}

	err := runLogsView(logPath, 0)
	if err != nil {
		t.Fatalf("runLogsView(direct path) error = %v", err)
	}
}

// TestRunLogsClean tests the runLogsClean function.
func TestRunLogsClean(t *testing.T) {
	origLogDir := logs.LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	logs.LogDir = func() string { return tmpDir }
	defer func() { logs.LogDir = origLogDir }()

	// Create old and recent logs
	now := time.Now()
	oldTime := now.Add(-10 * 24 * time.Hour)
	recentTime := now.Add(-1 * time.Hour)

	createTestLog(t, filepath.Join(tmpDir, "sessions", "old-session.log"), "old content", oldTime)
	createTestLog(t, filepath.Join(tmpDir, "sessions", "recent-session.log"), "recent content", recentTime)

	t.Run("dry run", func(t *testing.T) {
		err := runLogsClean(7*24*time.Hour, true)
		if err != nil {
			t.Fatalf("runLogsClean(dry-run) error = %v", err)
		}

		// Verify old log still exists (dry run)
		if _, err := os.Stat(filepath.Join(tmpDir, "sessions", "old-session.log")); os.IsNotExist(err) {
			t.Error("dry run should not delete files")
		}
	})

	t.Run("actual clean", func(t *testing.T) {
		err := runLogsClean(7*24*time.Hour, false)
		if err != nil {
			t.Fatalf("runLogsClean() error = %v", err)
		}

		// Verify old log is deleted
		if _, err := os.Stat(filepath.Join(tmpDir, "sessions", "old-session.log")); !os.IsNotExist(err) {
			t.Error("old log should be deleted")
		}

		// Verify recent log still exists
		if _, err := os.Stat(filepath.Join(tmpDir, "sessions", "recent-session.log")); os.IsNotExist(err) {
			t.Error("recent log should not be deleted")
		}
	})
}

// TestRunLogsCleanEmpty tests runLogsClean with no old logs.
func TestRunLogsCleanEmpty(t *testing.T) {
	origLogDir := logs.LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	logs.LogDir = func() string { return tmpDir }
	defer func() { logs.LogDir = origLogDir }()

	// Create only recent log
	recentTime := time.Now().Add(-1 * time.Hour)
	createTestLog(t, filepath.Join(tmpDir, "sessions", "recent.log"), "content", recentTime)

	err := runLogsClean(7*24*time.Hour, false)
	if err != nil {
		t.Fatalf("runLogsClean() error = %v", err)
	}
}

// TestTruncate tests the truncate helper function.
func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short", "abcdefghij", 6, "abc..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

// TestSplitLines tests the splitLines helper function.
func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty string", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 1},
		{"two lines", "hello\nworld", 2},
		{"two lines with trailing newline", "hello\nworld\n", 2},
		{"multiple lines", "a\nb\nc\nd", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitLines(%q) returned %d lines, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

// TestSplitLinesContent tests the actual content from splitLines.
func TestSplitLinesContent(t *testing.T) {
	input := "line1\nline2\nline3"
	lines := splitLines(input)

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	expected := []string{"line1", "line2", "line3"}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], want)
		}
	}
}

// TestLogsCommandExecution tests the full command execution.
func TestLogsCommandExecution(t *testing.T) {
	origLogDir := logs.LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	logs.LogDir = func() string { return tmpDir }
	defer func() { logs.LogDir = origLogDir }()

	// Create test logs
	now := time.Now()
	createTestLog(t, filepath.Join(tmpDir, "sessions", "test.log"), "test content", now)

	cmd := newLogsCmd()

	t.Run("list command", func(t *testing.T) {
		// Capture output
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})

	t.Run("view command", func(t *testing.T) {
		cmd := newLogsCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"test"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})

	t.Run("clean dry-run command", func(t *testing.T) {
		cmd := newLogsCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--clean", "--dry-run"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})
}

// TestLogsCommandWithTypeFlag tests the --type flag.
func TestLogsCommandWithTypeFlag(t *testing.T) {
	origLogDir := logs.LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	logs.LogDir = func() string { return tmpDir }
	defer func() { logs.LogDir = origLogDir }()

	createTestLog(t, filepath.Join(tmpDir, "sessions", "s1.log"), "session", time.Now())
	createTestLog(t, filepath.Join(tmpDir, "workers", "w1.log"), "worker", time.Now())

	cmd := newLogsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--type", "worker"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

// TestLogsCommandInvalidType tests error handling for invalid type.
func TestLogsCommandInvalidType(t *testing.T) {
	cmd := newLogsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--type", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid type")
	}
}
