package tmux

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// ============================================================
// Unit Tests - Pure Functions (no tmux required)
// ============================================================

func TestNewSession(t *testing.T) {
	tests := []struct {
		name    string
		workDir string
		logFile string
	}{
		{"test-session", "/tmp", "/tmp/log.txt"},
		{"", "", ""},
		{"my-session", "/home/user", ""},
		{"session-with-log", "", "/var/log/session.log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession(tt.name, tt.workDir, tt.logFile)

			if s == nil {
				t.Fatal("NewSession returned nil")
			}
			if s.Name != tt.name {
				t.Errorf("Name = %q, want %q", s.Name, tt.name)
			}
			if s.WorkDir != tt.workDir {
				t.Errorf("WorkDir = %q, want %q", s.WorkDir, tt.workDir)
			}
			if s.LogFile != tt.logFile {
				t.Errorf("LogFile = %q, want %q", s.LogFile, tt.logFile)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"with'quote", "'with'\"'\"'quote'"},
		{"", "''"},
		{"multiple'quotes'here", "'multiple'\"'\"'quotes'\"'\"'here'"},
		{"path/to/file", "'path/to/file'"},
		{"$variable", "'$variable'"},
		{"back`tick", "'back`tick'"},
		{"double\"quote", "'double\"quote'"},
		{"special!@#$%^&*()", "'special!@#$%^&*()'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.expected {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsShellPrompt(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		// Standard prompts
		{"user@host:~/dir$ ", true},
		{"$ ", true},
		{"# ", true},
		{"> ", true},
		{"% ", true},
		{"$", true},
		{"#", true},
		{">", true},
		{"%", true},

		// Unicode prompts (Starship, etc.)
		{"❯", true},
		{"❯ ", true},
		{"➜", true},
		{"➜ ", true},
		{"→", true},
		{"→ ", true},
		{"›", true},
		{"› ", true},
		{"~/project ❯ ", true},
		{"main ➜ ", true},

		// Non-prompts
		{"running command...", false},
		{"", false},
		{"   ", false},
		{"output line", false},
		{"error: something failed", false},
		{"Building project...", false},
		{"[INFO] Starting server", false},
		{"12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := isShellPrompt(tt.line)
			if got != tt.expected {
				t.Errorf("isShellPrompt(%q) = %v, want %v", tt.line, got, tt.expected)
			}
		})
	}
}

// ============================================================
// Error Constants Tests
// ============================================================

func TestErrorConstants(t *testing.T) {
	// Verify error variables are defined and distinct
	if ErrSessionExists == nil {
		t.Error("ErrSessionExists is nil")
	}
	if ErrSessionNotFound == nil {
		t.Error("ErrSessionNotFound is nil")
	}
	if ErrTmuxNotFound == nil {
		t.Error("ErrTmuxNotFound is nil")
	}

	// Verify they're distinct
	if errors.Is(ErrSessionExists, ErrSessionNotFound) {
		t.Error("ErrSessionExists and ErrSessionNotFound should be distinct")
	}
	if errors.Is(ErrSessionExists, ErrTmuxNotFound) {
		t.Error("ErrSessionExists and ErrTmuxNotFound should be distinct")
	}
	if errors.Is(ErrSessionNotFound, ErrTmuxNotFound) {
		t.Error("ErrSessionNotFound and ErrTmuxNotFound should be distinct")
	}

	// Verify error messages are meaningful
	if !strings.Contains(ErrSessionExists.Error(), "exists") {
		t.Errorf("ErrSessionExists message should contain 'exists': %q", ErrSessionExists.Error())
	}
	if !strings.Contains(ErrSessionNotFound.Error(), "not found") {
		t.Errorf("ErrSessionNotFound message should contain 'not found': %q", ErrSessionNotFound.Error())
	}
	if !strings.Contains(ErrTmuxNotFound.Error(), "not found") {
		t.Errorf("ErrTmuxNotFound message should contain 'not found': %q", ErrTmuxNotFound.Error())
	}
}

// ============================================================
// Session Struct Tests
// ============================================================

func TestSessionStructFields(t *testing.T) {
	s := &Session{
		Name:    "test-session",
		WorkDir: "/tmp/test",
		LogFile: "/tmp/test.log",
	}

	if s.Name != "test-session" {
		t.Errorf("Name = %q, want %q", s.Name, "test-session")
	}
	if s.WorkDir != "/tmp/test" {
		t.Errorf("WorkDir = %q, want %q", s.WorkDir, "/tmp/test")
	}
	if s.LogFile != "/tmp/test.log" {
		t.Errorf("LogFile = %q, want %q", s.LogFile, "/tmp/test.log")
	}
}

func TestSessionInfoStructFields(t *testing.T) {
	now := time.Now()
	info := &SessionInfo{
		Name:     "test-session",
		Created:  now,
		Attached: true,
		Width:    120,
		Height:   40,
		Activity: now,
	}

	if info.Name != "test-session" {
		t.Errorf("Name = %q, want %q", info.Name, "test-session")
	}
	if !info.Attached {
		t.Error("Attached should be true")
	}
	if info.Width != 120 {
		t.Errorf("Width = %d, want %d", info.Width, 120)
	}
	if info.Height != 40 {
		t.Errorf("Height = %d, want %d", info.Height, 40)
	}
}

// ============================================================
// Environment Tests
// ============================================================

func TestIsTmuxInstalled(t *testing.T) {
	// This is an environment-dependent test
	// Just verify it returns consistently and doesn't panic
	result1 := IsTmuxInstalled()
	result2 := IsTmuxInstalled()

	if result1 != result2 {
		t.Error("IsTmuxInstalled should return consistent results")
	}
}

// ============================================================
// Integration Tests (require tmux to be installed)
// ============================================================

// skipIfNoTmux skips the test if tmux is not installed
func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if !IsTmuxInstalled() {
		t.Skip("tmux not installed, skipping integration test")
	}
}

// testSessionName generates a unique test session name
func testSessionName(t *testing.T) string {
	return "forge-test-" + strings.ReplaceAll(t.Name(), "/", "-") + "-" + time.Now().Format("150405")
}

func TestSessionExists_NonExistent(t *testing.T) {
	skipIfNoTmux(t)

	s := &Session{Name: "nonexistent-forge-test-session-xyz123"}
	if s.Exists() {
		t.Error("Expected nonexistent session to return false")
	}
}

func TestSessionCreateKillCycle(t *testing.T) {
	skipIfNoTmux(t)

	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	// Ensure cleanup
	defer func() {
		_ = s.Kill()
	}()

	// Test session doesn't exist initially
	if s.Exists() {
		t.Fatal("Session should not exist before Create")
	}

	// Create session
	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Verify it exists
	if !s.Exists() {
		t.Fatal("Session should exist after Create")
	}

	// Kill session
	if err := s.Kill(); err != nil {
		t.Fatalf("Kill() error: %v", err)
	}

	// Verify it's gone
	if s.Exists() {
		t.Fatal("Session should not exist after Kill")
	}
}

func TestSessionCreate_AlreadyExists(t *testing.T) {
	skipIfNoTmux(t)

	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	// Create first session
	if err := s.Create(); err != nil {
		t.Fatalf("First Create() error: %v", err)
	}

	// Try to create again - should fail
	err := s.Create()
	if err == nil {
		t.Fatal("Second Create() should return error")
	}
	if !errors.Is(err, ErrSessionExists) {
		t.Errorf("Expected ErrSessionExists, got: %v", err)
	}
}

func TestSessionKill_NonExistent(t *testing.T) {
	skipIfNoTmux(t)

	s := &Session{Name: "nonexistent-session-for-kill-test"}

	// Killing a non-existent session should not error (idempotent)
	if err := s.Kill(); err != nil {
		t.Errorf("Kill() on non-existent session should not error: %v", err)
	}
}

func TestSessionSendKeys_NonExistent(t *testing.T) {
	skipIfNoTmux(t)

	s := &Session{Name: "nonexistent-session-for-sendkeys-test"}

	err := s.SendKeys("echo hello")
	if err == nil {
		t.Fatal("SendKeys() on non-existent session should error")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionCapturePane_NonExistent(t *testing.T) {
	skipIfNoTmux(t)

	s := &Session{Name: "nonexistent-session-for-capture-test"}

	_, err := s.CapturePane()
	if err == nil {
		t.Fatal("CapturePane() on non-existent session should error")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionCapturePaneLines_NonExistent(t *testing.T) {
	skipIfNoTmux(t)

	s := &Session{Name: "nonexistent-session-for-capturelines-test"}

	_, err := s.CapturePaneLines(10)
	if err == nil {
		t.Fatal("CapturePaneLines() on non-existent session should error")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionWaitForPrompt_NonExistent(t *testing.T) {
	skipIfNoTmux(t)

	s := &Session{Name: "nonexistent-session-for-waitprompt-test"}

	err := s.WaitForPrompt(100*time.Millisecond, 50*time.Millisecond)
	if err == nil {
		t.Fatal("WaitForPrompt() on non-existent session should error")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionIsClaudeRunning_NonExistent(t *testing.T) {
	skipIfNoTmux(t)

	s := &Session{Name: "nonexistent-session-for-clauderunning-test"}

	// Non-existent session should return false
	if s.IsClaudeRunning() {
		t.Error("IsClaudeRunning() on non-existent session should return false")
	}
}

func TestSessionSendKeysAndCapture(t *testing.T) {
	skipIfNoTmux(t)

	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Send a simple command
	if err := s.SendKeys("echo TESTMARKER123"); err != nil {
		t.Fatalf("SendKeys() error: %v", err)
	}

	// Wait a bit for command to execute
	time.Sleep(500 * time.Millisecond)

	// Capture pane content
	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	if !strings.Contains(content, "TESTMARKER123") {
		t.Errorf("CapturePane should contain 'TESTMARKER123', got: %q", content)
	}
}

func TestSessionCapturePaneLines(t *testing.T) {
	skipIfNoTmux(t)

	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Send multiple commands to generate output
	for i := 0; i < 5; i++ {
		_ = s.SendKeys("echo line" + string(rune('A'+i)))
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(300 * time.Millisecond)

	// Capture last 10 lines
	lines, err := s.CapturePaneLines(10)
	if err != nil {
		t.Fatalf("CapturePaneLines() error: %v", err)
	}

	if len(lines) == 0 {
		t.Error("CapturePaneLines should return some lines")
	}
}

func TestSessionRunCommand(t *testing.T) {
	skipIfNoTmux(t)

	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// RunCommand is an alias for SendKeys
	if err := s.RunCommand("echo RUNCOMMAND_TEST"); err != nil {
		t.Fatalf("RunCommand() error: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	if !strings.Contains(content, "RUNCOMMAND_TEST") {
		t.Errorf("Output should contain 'RUNCOMMAND_TEST', got: %q", content)
	}
}

func TestSessionIsClaudeRunning_ShellOnly(t *testing.T) {
	skipIfNoTmux(t)

	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Give shell time to start
	time.Sleep(200 * time.Millisecond)

	// With just a shell running, IsClaudeRunning should return false
	if s.IsClaudeRunning() {
		t.Error("IsClaudeRunning() should return false when only shell is running")
	}
}

func TestSessionCreateWithLogFile(t *testing.T) {
	skipIfNoTmux(t)

	name := testSessionName(t)
	workDir := t.TempDir()
	logFile := workDir + "/session.log"
	s := NewSession(name, workDir, logFile)

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() with log file error: %v", err)
	}

	if !s.Exists() {
		t.Fatal("Session should exist after Create with log file")
	}

	// Send some output
	_ = s.SendKeys("echo LOG_TEST_OUTPUT")
	time.Sleep(500 * time.Millisecond)

	// Check if log file was created (may take a moment for pipe-pane)
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		// Log file may not be created immediately - this is not a failure
		t.Log("Log file not created yet (this is acceptable)")
	}
}

func TestListSessions(t *testing.T) {
	skipIfNoTmux(t)

	// Create a test session to ensure there's at least one
	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// List all sessions
	sessions, err := ListSessions("")
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	// Our session should be in the list
	found := false
	for _, sess := range sessions {
		if sess == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Session %q not found in list: %v", name, sessions)
	}
}

func TestListSessionsWithPrefix(t *testing.T) {
	skipIfNoTmux(t)

	// Create a test session with known prefix
	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// List with matching prefix
	sessions, err := ListSessions("forge-test-")
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	// Our session should be in the list
	found := false
	for _, sess := range sessions {
		if sess == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Session %q not found in filtered list: %v", name, sessions)
	}

	// List with non-matching prefix
	sessions, err = ListSessions("nonexistent-prefix-xyz-")
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	for _, sess := range sessions {
		if sess == name {
			t.Errorf("Session %q should not be in list with non-matching prefix", name)
		}
	}
}

func TestListForgeSessions(t *testing.T) {
	skipIfNoTmux(t)

	// Create a forge-prefixed session
	name := "forge-test-listforge-" + time.Now().Format("150405")
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	sessions, err := ListForgeSessions()
	if err != nil {
		t.Fatalf("ListForgeSessions() error: %v", err)
	}

	found := false
	for _, sess := range sessions {
		if sess == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Session %q not found in forge sessions list: %v", name, sessions)
	}
}

func TestGetSessionInfo(t *testing.T) {
	skipIfNoTmux(t)

	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	info, err := GetSessionInfo(name)
	if err != nil {
		t.Fatalf("GetSessionInfo() error: %v", err)
	}

	if info.Name != name {
		t.Errorf("info.Name = %q, want %q", info.Name, name)
	}

	// Session should not be attached (we're not attached)
	if info.Attached {
		t.Error("info.Attached should be false for detached session")
	}

	// Width and height may be 0 for detached sessions in some tmux configurations
	// Just verify they're non-negative
	if info.Width < 0 {
		t.Errorf("info.Width should be non-negative, got %d", info.Width)
	}
	if info.Height < 0 {
		t.Errorf("info.Height should be non-negative, got %d", info.Height)
	}

	// Created time should be recent
	if time.Since(info.Created) > 10*time.Second {
		t.Errorf("info.Created too old: %v", info.Created)
	}
}

func TestGetSessionInfo_NonExistent(t *testing.T) {
	skipIfNoTmux(t)

	_, err := GetSessionInfo("nonexistent-session-info-test")
	if err == nil {
		t.Fatal("GetSessionInfo() on non-existent session should error")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionRunClaude_NonExistent(t *testing.T) {
	skipIfNoTmux(t)

	s := &Session{Name: "nonexistent-session-for-runclaude-test"}

	err := s.RunClaude("test prompt", "", false)
	if err == nil {
		t.Fatal("RunClaude() on non-existent session should error")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionWaitForPrompt_Timeout(t *testing.T) {
	skipIfNoTmux(t)

	name := testSessionName(t)
	workDir := t.TempDir()
	s := NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Send a command that keeps running
	_ = s.SendKeys("sleep 10")

	// Wait with very short timeout - should fail
	err := s.WaitForPrompt(200*time.Millisecond, 50*time.Millisecond)
	if err == nil {
		t.Error("WaitForPrompt should timeout when command is running")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Error should mention timeout, got: %v", err)
	}
}
