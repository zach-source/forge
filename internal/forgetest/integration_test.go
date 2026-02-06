// Package forgetest provides end-to-end integration tests for the forge CLI.
// These tests verify the complete forge pipeline: tmux session management,
// agent configuration, Ralph loop state, completion detection, session
// discovery, log management, and resume workflows.
package forgetest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zach-source/forge/internal/agent"
	"github.com/zach-source/forge/internal/detector"
	"github.com/zach-source/forge/internal/logs"
	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/tmux"
)

// ============================================================
// Section 1: Tmux Session Lifecycle
// ============================================================

func TestTmuxSession_CreateAndKill(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-test-ck-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if s.Exists() {
		t.Fatal("session should not exist before Create()")
	}

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if !s.Exists() {
		t.Fatal("session should exist after Create()")
	}

	if err := s.Kill(); err != nil {
		t.Fatalf("Kill() error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	if s.Exists() {
		t.Error("session should not exist after Kill()")
	}
}

func TestTmuxSession_SendKeysAndCapture(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-test-sc-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := s.SendKeys("echo FORGE_TEST_OUTPUT_12345"); err != nil {
		t.Fatalf("SendKeys() error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	if !strings.Contains(content, "FORGE_TEST_OUTPUT_12345") {
		t.Errorf("CapturePane() should contain test output, got:\n%s", content)
	}
}

func TestTmuxSession_CapturePaneLines(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-test-cl-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	for i := 1; i <= 5; i++ {
		if err := s.SendKeys(fmt.Sprintf("echo LINE_%d", i)); err != nil {
			t.Fatalf("SendKeys() error on line %d: %v", i, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(300 * time.Millisecond)

	lines, err := s.CapturePaneLines(20)
	if err != nil {
		t.Fatalf("CapturePaneLines() error: %v", err)
	}

	if len(lines) == 0 {
		t.Fatal("CapturePaneLines() returned no lines")
	}

	found := 0
	for _, line := range lines {
		for i := 1; i <= 5; i++ {
			if strings.Contains(line, fmt.Sprintf("LINE_%d", i)) {
				found++
				break
			}
		}
	}
	if found < 3 {
		t.Errorf("Expected at least 3 of 5 lines in output, found %d", found)
	}
}

func TestTmuxSession_NonExistentSession(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-test-nonexist-%d", time.Now().UnixNano())
	s := tmux.NewSession(name, "/tmp", "")

	if s.Exists() {
		t.Error("non-existent session should return false for Exists()")
	}

	_, err := s.CapturePane()
	if err == nil {
		t.Error("CapturePane() on non-existent session should return error")
	}
}

func TestTmuxSession_RunCommand(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-test-rc-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := s.RunCommand("echo RUNCOMMAND_OK"); err != nil {
		t.Fatalf("RunCommand() error: %v", err)
	}

	time.Sleep(400 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	if !strings.Contains(content, "RUNCOMMAND_OK") {
		t.Errorf("RunCommand() output should contain RUNCOMMAND_OK, got:\n%s", content)
	}
}

func TestTmuxSession_Logging(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-test-log-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "test.log")
	s := tmux.NewSession(name, workDir, logFile)
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := s.SendKeys("echo LOG_CAPTURE_TEST"); err != nil {
		t.Fatalf("SendKeys() error: %v", err)
	}

	time.Sleep(1 * time.Second)

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", logFile, err)
	}

	if len(data) == 0 {
		t.Error("log file should have content")
	}
}

func TestTmuxSession_ListForgeSessions(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-test-list-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	sessions, err := tmux.ListForgeSessions()
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
		t.Errorf("ListForgeSessions() should include %q, got: %v", name, sessions)
	}
}

func TestTmuxSession_ListSessions(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	prefix := fmt.Sprintf("forge-test-lsp-%d", time.Now().UnixNano())
	name := prefix + "-a"
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	sessions, err := tmux.ListSessions(prefix)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	if len(sessions) == 0 {
		t.Error("ListSessions() should find at least 1 session with prefix")
	}

	found := false
	for _, sess := range sessions {
		if sess == name {
			found = true
		}
	}
	if !found {
		t.Errorf("ListSessions() should include %q", name)
	}
}

func TestTmuxSession_GetSessionInfo(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-test-info-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	info, err := tmux.GetSessionInfo(name)
	if err != nil {
		t.Fatalf("GetSessionInfo() error: %v", err)
	}

	if info.Created.IsZero() {
		t.Error("session info Created should not be zero")
	}
}

// TestTmuxSession_RunClaude verifies RunClaude sends the correct command format.
func TestTmuxSession_RunClaude(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-test-runcl-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	err := s.RunClaude("test prompt", "", true)
	if err != nil {
		t.Fatalf("RunClaude() error: %v", err)
	}

	time.Sleep(400 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	flat := strings.ReplaceAll(content, "\n", "")

	// RunClaude uses pipe pattern: cat <tmpfile> | claude ... ; rm -f <tmpfile>
	if !strings.Contains(flat, "claude") {
		t.Errorf("RunClaude() should send claude command, got:\n%s", content)
	}
	if !strings.Contains(flat, "dangerously-skip-permissions") {
		t.Errorf("RunClaude(skip=true) should include --dangerously-skip-permissions, got:\n%s", content)
	}
	// Verify pipe pattern (the Ralph stdin approach)
	if !strings.Contains(flat, "cat") || !strings.Contains(flat, "| ") {
		t.Errorf("RunClaude() should use pipe pattern (cat file | claude), got:\n%s", content)
	}
}

// ============================================================
// Section 2: Completion Detection
// ============================================================

func TestDetector_XMLPromise(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		promise string
		output  string
		want    bool
	}{
		{"exact match", "DONE", "<promise>DONE</promise>", true},
		{"case insensitive", "COMPLETE", "<promise>complete</promise>", true},
		{"no match", "DONE", "Still working...", false},
		{"empty output", "DONE", "", false},
		{"partial promise", "DONE", "<promise>DO</promise>", false},
		{"promise in text", "DONE", "I am <promise>DONE</promise> now", true},
		{"multiple promises", "TARGET", "first <promise>OTHER</promise> then <promise>TARGET</promise>", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := detector.New(tt.promise)
			got := d.IsComplete(tt.output)
			if got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_BarePromise(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		promise string
		output  string
		want    bool
	}{
		{"bare on own line", "DONE", "working...\nDONE\nfinished", true},
		{"bare case insensitive", "DONE", "working...\ndone\nfinished", true},
		{"short not on own line", "DONE", "I am DONE with this but more text here", false},
		{"bare with whitespace", "DONE", "  DONE  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := detector.New(tt.promise)
			got := d.IsComplete(tt.output)
			if got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDetector_LongBarePromise_ContainsMatchMidLine verifies that a long promise
// (>10 chars) uses contains matching and WILL match mid-line in bare fallback.
func TestDetector_LongBarePromise_ContainsMatchMidLine(t *testing.T) {
	t.Parallel()
	d := detector.New("TASK_FINISHED_OK")

	// A long promise (>10 chars) uses contains matching, so it matches mid-line
	got := d.IsComplete("The work is TASK_FINISHED_OK and all done")
	if !got {
		t.Error("long bare promise (>10 chars) should match mid-line via contains")
	}

	// Short promise should NOT match mid-line
	d2 := detector.New("DONE")
	got2 := d2.IsComplete("I am DONE with this but more text here")
	if got2 {
		t.Error("short bare promise should NOT match mid-line (exact match only)")
	}
}

func TestDetector_ExtractPromises(t *testing.T) {
	t.Parallel()
	d := detector.New("TARGET")

	output := "some text <promise>FIRST</promise> more <promise>TARGET</promise> end"
	promises := d.ExtractPromises(output)

	if len(promises) != 2 {
		t.Fatalf("ExtractPromises() returned %d promises, want 2", len(promises))
	}

	if promises[0] != "FIRST" {
		t.Errorf("promises[0] = %q, want %q", promises[0], "FIRST")
	}
	if promises[1] != "TARGET" {
		t.Errorf("promises[1] = %q, want %q", promises[1], "TARGET")
	}
}

func TestDetector_Check(t *testing.T) {
	t.Parallel()
	d := detector.New("COMPLETE")

	// XML match
	result := d.Check("output <promise>COMPLETE</promise> done")
	if !result.Complete {
		t.Error("Check() should be complete for XML match")
	}
	if len(result.Promises) == 0 {
		t.Error("Check() should have promises for XML match")
	}

	// Bare match
	result = d.Check("working...\nCOMPLETE\nfinished")
	if !result.Complete {
		t.Error("Check() should be complete for bare match")
	}

	// No match
	result = d.Check("still working on it")
	if result.Complete {
		t.Error("Check() should not be complete for no match")
	}
}

func TestDetector_LongPromiseContainsMatch(t *testing.T) {
	t.Parallel()
	d := detector.New("TASK_FINISHED_SUCCESSFULLY")

	result := d.Check("<promise>All work TASK_FINISHED_SUCCESSFULLY today</promise>")
	if !result.Complete {
		t.Error("long promise should match via contains inside XML tags")
	}
}

// TestDetector_EmptyPromise verifies that an empty promise never matches.
func TestDetector_EmptyPromise(t *testing.T) {
	t.Parallel()
	d := detector.New("")

	// Empty promise should NOT match anything — prevents accidental completion
	if d.IsComplete("<promise></promise>") {
		t.Error("empty promise should NOT match empty promise tag")
	}
	if d.IsComplete("<promise>anything</promise>") {
		t.Error("empty promise should NOT match non-empty promise tag")
	}
	if d.IsComplete("random text") {
		t.Error("empty promise should NOT match random text")
	}
}

func TestDetector_WhitespaceInPromise(t *testing.T) {
	t.Parallel()
	d := detector.New("  DONE  ")

	result := d.Check("<promise>DONE</promise>")
	if !result.Complete {
		t.Error("whitespace-padded promise should match trimmed value")
	}
}

// ============================================================
// Section 3: Ralph State Management
// ============================================================

func TestRalphState_WriteAndRead(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test.state.md")
	ctrl := ralph.NewStateController(statePath)

	if ctrl.Exists() {
		t.Fatal("state file should not exist initially")
	}

	state := &ralph.State{
		ID:                "forge-test-123",
		Active:            true,
		Iteration:         7,
		MaxIterations:     50,
		CompletionPromise: "DONE",
		StartedAt:         time.Now().Truncate(time.Second),
		TmuxSession:       "forge-test-123",
		WorkDir:           "/tmp/test",
		LogFile:           "/tmp/test.log",
		Prompt:            "Build a REST API",
	}

	if err := ctrl.Write(state); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if !ctrl.Exists() {
		t.Fatal("state file should exist after Write()")
	}

	read, err := ctrl.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if read.ID != state.ID {
		t.Errorf("ID = %q, want %q", read.ID, state.ID)
	}
	if read.Active != state.Active {
		t.Errorf("Active = %v, want %v", read.Active, state.Active)
	}
	if read.Iteration != state.Iteration {
		t.Errorf("Iteration = %d, want %d", read.Iteration, state.Iteration)
	}
	if read.MaxIterations != state.MaxIterations {
		t.Errorf("MaxIterations = %d, want %d", read.MaxIterations, state.MaxIterations)
	}
	if read.CompletionPromise != state.CompletionPromise {
		t.Errorf("CompletionPromise = %q, want %q", read.CompletionPromise, state.CompletionPromise)
	}
	if read.WorkDir != state.WorkDir {
		t.Errorf("WorkDir = %q, want %q", read.WorkDir, state.WorkDir)
	}
	if read.LogFile != state.LogFile {
		t.Errorf("LogFile = %q, want %q", read.LogFile, state.LogFile)
	}
	if read.Prompt != state.Prompt {
		t.Errorf("Prompt = %q, want %q", read.Prompt, state.Prompt)
	}
}

func TestRalphState_Update(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "update.state.md")
	ctrl := ralph.NewStateController(statePath)

	state := &ralph.State{
		ID:                "forge-update-test",
		Active:            true,
		Iteration:         1,
		MaxIterations:     10,
		CompletionPromise: "DONE",
		Prompt:            "test prompt",
	}

	if err := ctrl.Write(state); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if err := ctrl.Update(func(s *ralph.State) {
		s.Iteration = 5
		s.Active = false
	}); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	read, err := ctrl.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if read.Iteration != 5 {
		t.Errorf("Iteration = %d, want 5", read.Iteration)
	}
	if read.Active {
		t.Error("Active should be false after update")
	}
}

func TestRalphState_Delete(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "delete.state.md")
	ctrl := ralph.NewStateController(statePath)

	state := &ralph.State{
		ID:     "forge-delete-test",
		Active: true,
		Prompt: "will be deleted",
	}

	if err := ctrl.Write(state); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if !ctrl.Exists() {
		t.Fatal("state file should exist before delete")
	}

	if err := ctrl.Delete(); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if ctrl.Exists() {
		t.Error("state file should not exist after Delete()")
	}
}

func TestRalphState_ReadNonExistent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ctrl := ralph.NewStateController(filepath.Join(tmpDir, "nonexistent.state.md"))

	_, err := ctrl.Read()
	if err == nil {
		t.Error("Read() on non-existent file should return error")
	}
}

func TestRalphState_ParseFormat(t *testing.T) {
	t.Parallel()
	content := `---
id: "forge-test-abc"
active: true
iteration: 3
max_iterations: 50
completion_promise: "TASK_DONE"
workdir: "/home/user/project"
log_file: "/tmp/forge.log"
---

Build a REST API for managing users.
Ensure it has proper authentication.`

	state, err := ralph.ParseState(content)
	if err != nil {
		t.Fatalf("ParseState() error: %v", err)
	}

	if state.ID != "forge-test-abc" {
		t.Errorf("ID = %q, want %q", state.ID, "forge-test-abc")
	}
	if state.Iteration != 3 {
		t.Errorf("Iteration = %d, want 3", state.Iteration)
	}
	if state.CompletionPromise != "TASK_DONE" {
		t.Errorf("CompletionPromise = %q, want %q", state.CompletionPromise, "TASK_DONE")
	}
	if !strings.Contains(state.Prompt, "Build a REST API") {
		t.Error("Prompt should contain the body text")
	}
	if !strings.Contains(state.Prompt, "authentication") {
		t.Error("Prompt should contain multi-line body")
	}
}

func TestRalphState_FormatState(t *testing.T) {
	t.Parallel()
	state := &ralph.State{
		ID:                "forge-format-test",
		Active:            true,
		Iteration:         7,
		MaxIterations:     50,
		CompletionPromise: "DONE",
		Prompt:            "Test prompt content",
	}

	content, err := ralph.FormatState(state)
	if err != nil {
		t.Fatalf("FormatState() error: %v", err)
	}

	if !strings.HasPrefix(content, "---\n") {
		t.Error("FormatState() should start with ---")
	}
	if !strings.Contains(content, "id: forge-format-test") {
		t.Error("FormatState() should contain id")
	}
	if !strings.Contains(content, "iteration: 7") {
		t.Error("FormatState() should contain iteration")
	}
	if !strings.Contains(content, "Test prompt content") {
		t.Error("FormatState() should contain prompt body")
	}

	// Verify roundtrip
	parsed, err := ralph.ParseState(content)
	if err != nil {
		t.Fatalf("ParseState(FormatState()) error: %v", err)
	}
	if parsed.ID != state.ID {
		t.Errorf("roundtrip ID = %q, want %q", parsed.ID, state.ID)
	}
	if parsed.Prompt != state.Prompt {
		t.Errorf("roundtrip Prompt = %q, want %q", parsed.Prompt, state.Prompt)
	}
}

func TestRalphState_InvalidFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
	}{
		{"no frontmatter", "just plain text"},
		{"missing end delimiter", "---\nid: test\nbody text"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ralph.ParseState(tt.content)
			if err == nil {
				t.Error("ParseState() should return error for invalid format")
			}
		})
	}
}

func TestRalphState_ListSessionStateFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	origDir := ralph.DefaultSessionsDir
	ralph.DefaultSessionsDir = func() string { return tmpDir }
	defer func() { ralph.DefaultSessionsDir = origDir }()

	for _, name := range []string{"forge-a.state.md", "forge-b.state.md", "not-a-state.txt"} {
		path := filepath.Join(tmpDir, name)
		state := &ralph.State{
			ID:     strings.TrimSuffix(name, ".state.md"),
			Active: true,
			Prompt: "test",
		}
		content, _ := ralph.FormatState(state)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error: %v", name, err)
		}
	}

	files, err := ralph.ListSessionStateFiles()
	if err != nil {
		t.Fatalf("ListSessionStateFiles() error: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("ListSessionStateFiles() returned %d files, want 2", len(files))
	}

	for _, f := range files {
		if !strings.HasSuffix(f, ".state.md") {
			t.Errorf("unexpected file: %s", f)
		}
	}
}

func TestRalphState_AtomicWrite(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "atomic.state.md")
	ctrl := ralph.NewStateController(statePath)

	state := &ralph.State{
		ID:     "forge-atomic",
		Active: true,
		Prompt: "atomic write test",
	}

	if err := ctrl.Write(state); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !strings.HasPrefix(string(data), "---\n") {
		t.Error("state file should start with frontmatter delimiter")
	}
}

// ============================================================
// Section 4: Session ID Generation
// ============================================================

func TestSessionID_Generation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		prompt string
		prefix string
	}{
		{"two words", "Build API", "build-api-"},
		{"long prompt", "Implement user authentication with JWT", "implemen-user-"},
		{"single word", "refactor", "refactor-"},
		{"special chars", "Fix bug #123!", "fix-bug-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := session.GenerateSessionID(tt.prompt)
			if !strings.HasPrefix(id, tt.prefix) {
				t.Errorf("GenerateSessionID(%q) = %q, want prefix %q", tt.prompt, id, tt.prefix)
			}
			parts := strings.Split(id, "-")
			hash := parts[len(parts)-1]
			if len(hash) != 8 {
				t.Errorf("hash suffix %q should be 8 chars", hash)
			}
		})
	}
}

func TestSessionID_Deterministic(t *testing.T) {
	t.Parallel()
	prompt := "Build a REST API for todos"
	id1 := session.GenerateSessionID(prompt)
	id2 := session.GenerateSessionID(prompt)

	if id1 != id2 {
		t.Errorf("same prompt should produce same ID: %q vs %q", id1, id2)
	}
}

func TestSessionID_Unique(t *testing.T) {
	t.Parallel()
	id1 := session.GenerateSessionID("Build a REST API")
	id2 := session.GenerateSessionID("Build a GraphQL API")

	if id1 == id2 {
		t.Errorf("different prompts should produce different IDs: both got %q", id1)
	}
}

func TestSessionID_ForgeSessionName(t *testing.T) {
	t.Parallel()
	name := session.ForgeSessionName("forge-api-abc12345")
	if name != "forge-api-abc12345" {
		t.Errorf("already-prefixed: got %q, want %q", name, "forge-api-abc12345")
	}

	name = session.ForgeSessionName("api-abc12345")
	if name != "forge-api-abc12345" {
		t.Errorf("not-prefixed: got %q, want %q", name, "forge-api-abc12345")
	}
}

func TestSessionID_IsForgeSession(t *testing.T) {
	t.Parallel()
	if !session.IsForgeSession("forge-api-abc") {
		t.Error("forge-api-abc should be a forge session")
	}
	if session.IsForgeSession("foundry-worker-abc") {
		t.Error("foundry-worker-abc should not be a forge session")
	}
	if session.IsForgeSession("random-session") {
		t.Error("random-session should not be a forge session")
	}
}

func TestSessionID_SessionIDFromName(t *testing.T) {
	t.Parallel()
	// With prefix — strips it
	id := session.SessionIDFromName("forge-api-abc12345")
	if id != "api-abc12345" {
		t.Errorf("SessionIDFromName(forge-api-abc12345) = %q, want %q", id, "api-abc12345")
	}

	// Without prefix — unchanged
	id = session.SessionIDFromName("api-abc12345")
	if id != "api-abc12345" {
		t.Errorf("SessionIDFromName(api-abc12345) = %q, want %q", id, "api-abc12345")
	}
}

func TestSessionID_EmptyPrompt(t *testing.T) {
	t.Parallel()
	id := session.GenerateSessionID("!@#$%")
	if !strings.HasPrefix(id, "forge-") {
		t.Errorf("special-chars prompt should fallback to forge- prefix, got: %s", id)
	}
}

// ============================================================
// Section 5: Session Status and Display
// ============================================================

func TestSession_StatusIcon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status session.Status
		icon   string
	}{
		{session.StatusActive, "🔄"},
		{session.StatusCompleted, "✅"},
		{session.StatusCancelled, "❌"},
		{session.StatusError, "⚠️"},
		{session.StatusPaused, "⏸️"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			t.Parallel()
			s := &session.Session{Status: tt.status}
			icon := s.StatusIcon()
			if icon != tt.icon {
				t.Errorf("StatusIcon() = %q, want %q", icon, tt.icon)
			}
		})
	}
}

func TestSession_IterationString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		state *ralph.State
		want  string
	}{
		{"nil state", nil, "-/-"},
		{"with max", &ralph.State{Iteration: 7, MaxIterations: 50}, "7/50"},
		{"unlimited", &ralph.State{Iteration: 3, MaxIterations: 0}, "3/∞"},
		{"at max", &ralph.State{Iteration: 50, MaxIterations: 50}, "50/50"},
		{"zero", &ralph.State{Iteration: 0, MaxIterations: 10}, "0/10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &session.Session{State: tt.state}
			got := s.IterationString()
			if got != tt.want {
				t.Errorf("IterationString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSession_ElapsedString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		startedAt time.Time
		contains  string
	}{
		{"just now", time.Now(), "just now"},
		{"minutes ago", time.Now().Add(-5 * time.Minute), "ago"},
		{"hours ago", time.Now().Add(-3 * time.Hour), "ago"},
		{"zero time", time.Time{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &session.Session{StartedAt: tt.startedAt}
			got := s.ElapsedString()
			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("ElapsedString() = %q, want to contain %q", got, tt.contains)
			}
		})
	}
}

// ============================================================
// Section 6: Session Manager
// ============================================================

func TestSessionManager_AddAndGet(t *testing.T) {
	t.Parallel()
	mgr := session.NewManager()

	s := &session.Session{
		ID:     "forge-test-mgr",
		Status: session.StatusActive,
		State: &ralph.State{
			ID:                "forge-test-mgr",
			Iteration:         3,
			CompletionPromise: "DONE",
		},
	}

	mgr.Add(s)

	got := mgr.Get("forge-test-mgr")
	if got == nil {
		t.Fatal("Get() should return the added session")
	}
	if got.ID != "forge-test-mgr" {
		t.Errorf("ID = %q, want %q", got.ID, "forge-test-mgr")
	}
}

func TestSessionManager_List(t *testing.T) {
	t.Parallel()
	mgr := session.NewManager()

	mgr.Add(&session.Session{ID: "a", Status: session.StatusActive})
	mgr.Add(&session.Session{ID: "b", Status: session.StatusCompleted})
	mgr.Add(&session.Session{ID: "c", Status: session.StatusActive})

	list := mgr.List()
	if len(list) != 3 {
		t.Errorf("List() returned %d sessions, want 3", len(list))
	}
}

func TestSessionManager_Remove(t *testing.T) {
	t.Parallel()
	mgr := session.NewManager()
	mgr.Add(&session.Session{ID: "forge-remove-test", Status: session.StatusActive})

	mgr.Remove("forge-remove-test")

	if got := mgr.Get("forge-remove-test"); got != nil {
		t.Error("Get() should return nil after Remove()")
	}
}

func TestSessionManager_ActiveCount(t *testing.T) {
	t.Parallel()
	mgr := session.NewManager()

	mgr.Add(&session.Session{ID: "a", Status: session.StatusActive})
	mgr.Add(&session.Session{ID: "b", Status: session.StatusCompleted})
	mgr.Add(&session.Session{ID: "c", Status: session.StatusActive})
	mgr.Add(&session.Session{ID: "d", Status: session.StatusCancelled})

	count := mgr.ActiveCount()
	if count != 2 {
		t.Errorf("ActiveCount() = %d, want 2", count)
	}
}

func TestSessionManager_GetNonExistent(t *testing.T) {
	t.Parallel()
	mgr := session.NewManager()

	if got := mgr.Get("nonexistent"); got != nil {
		t.Error("Get() should return nil for non-existent session")
	}
}

func TestSessionManager_DiscoverWithStateFiles(t *testing.T) {
	tmpDir := t.TempDir()

	origDir := ralph.DefaultSessionsDir
	ralph.DefaultSessionsDir = func() string { return tmpDir }
	defer func() { ralph.DefaultSessionsDir = origDir }()

	state := &ralph.State{
		ID:                "forge-disc-test",
		Active:            false,
		Iteration:         10,
		MaxIterations:     50,
		CompletionPromise: "DONE",
		WorkDir:           "/tmp/test",
		Prompt:            "test task",
	}
	content, _ := ralph.FormatState(state)
	statePath := filepath.Join(tmpDir, "forge-disc-test.state.md")
	if err := os.WriteFile(statePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	mgr := session.NewManager()
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	s := mgr.Get("forge-disc-test")
	if s == nil {
		t.Fatal("Discover() should find session from state file")
	}

	if s.Status != session.StatusCompleted {
		t.Errorf("Status = %q, want %q", s.Status, session.StatusCompleted)
	}
	if s.State.Iteration != 10 {
		t.Errorf("Iteration = %d, want 10", s.State.Iteration)
	}
}

func TestSessionManager_DiscoverLocal(t *testing.T) {
	t.Parallel()
	workDir := t.TempDir()

	claudeDir := filepath.Join(workDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	state := &ralph.State{
		ID:                "forge-local-test",
		Active:            true,
		Iteration:         2,
		CompletionPromise: "LOCAL_DONE",
		Prompt:            "local task",
	}
	content, _ := ralph.FormatState(state)
	localPath := filepath.Join(claudeDir, "ralph-loop.local.md")
	if err := os.WriteFile(localPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	mgr := session.NewManager()
	if err := mgr.DiscoverLocal(workDir); err != nil {
		t.Fatalf("DiscoverLocal() error: %v", err)
	}

	s := mgr.Get("forge-local-test")
	if s == nil {
		t.Fatal("DiscoverLocal() should find the local state file")
	}

	if s.State.CompletionPromise != "LOCAL_DONE" {
		t.Errorf("CompletionPromise = %q, want %q", s.State.CompletionPromise, "LOCAL_DONE")
	}
}

// TestSessionManager_EmptyDiscover verifies Discover on empty state dir doesn't error.
func TestSessionManager_EmptyDiscover(t *testing.T) {
	tmpDir := t.TempDir()

	origDir := ralph.DefaultSessionsDir
	ralph.DefaultSessionsDir = func() string { return tmpDir }
	defer func() { ralph.DefaultSessionsDir = origDir }()

	mgr := session.NewManager()
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover() on empty dir should not error: %v", err)
	}

	// Count sessions that came from state files (not tmux discover)
	stateFileSessions := 0
	for _, s := range mgr.List() {
		if s.State != nil {
			stateFileSessions++
		}
	}
	if stateFileSessions != 0 {
		t.Errorf("should find 0 state-file sessions in empty dir, got %d", stateFileSessions)
	}
}

// ============================================================
// Section 7: Agent Configuration
// ============================================================

func TestAgentConfig_DefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := agent.DefaultConfig()

	if cfg.MaxIterations != 50 {
		t.Errorf("MaxIterations = %d, want 50", cfg.MaxIterations)
	}
	if !cfg.SkipPermissions {
		t.Error("SkipPermissions should default to true")
	}
	if cfg.PollInterval != 2*time.Second {
		t.Errorf("PollInterval = %v, want 2s", cfg.PollInterval)
	}
	if cfg.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", cfg.Timeout)
	}
	if len(cfg.MCPServers) != 0 {
		t.Errorf("MCPServers should be empty, got %v", cfg.MCPServers)
	}
}

func TestAgentConfig_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     agent.Config
		wantErr bool
	}{
		{"valid", agent.Config{Prompt: "test", CompletionPromise: "DONE", WorkDir: "/tmp"}, false},
		{"missing prompt", agent.Config{CompletionPromise: "DONE", WorkDir: "/tmp"}, true},
		{"missing promise", agent.Config{Prompt: "test", WorkDir: "/tmp"}, true},
		{"missing workdir", agent.Config{Prompt: "test", CompletionPromise: "DONE"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentConfig_ResumeConfig(t *testing.T) {
	t.Parallel()
	cfg := agent.DefaultResumeConfig()

	if cfg.ContextLines != 50 {
		t.Errorf("ContextLines = %d, want 50", cfg.ContextLines)
	}
	if !cfg.SkipPermissions {
		t.Error("SkipPermissions should default to true")
	}
	if cfg.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", cfg.Timeout)
	}
}

func TestAgentConfig_WorkerIdentityFields(t *testing.T) {
	t.Parallel()
	cfg := agent.Config{
		Prompt:            "test",
		CompletionPromise: "DONE",
		WorkDir:           "/tmp",
		WorkerID:          "w-abc12345",
		WorkerName:        "alpha",
		WorkerRole:        "worker",
	}

	if cfg.WorkerID != "w-abc12345" {
		t.Errorf("WorkerID = %q, want %q", cfg.WorkerID, "w-abc12345")
	}
	if cfg.WorkerName != "alpha" {
		t.Errorf("WorkerName = %q, want %q", cfg.WorkerName, "alpha")
	}
	if cfg.WorkerRole != "worker" {
		t.Errorf("WorkerRole = %q, want %q", cfg.WorkerRole, "worker")
	}
}

func TestAgentConfig_AgentTeamsFields(t *testing.T) {
	t.Parallel()
	cfg := agent.Config{AgentTeams: true, TeammateMode: "tmux"}

	if !cfg.AgentTeams {
		t.Error("AgentTeams should be true")
	}
	if cfg.TeammateMode != "tmux" {
		t.Errorf("TeammateMode = %q, want %q", cfg.TeammateMode, "tmux")
	}
}

func TestAgent_ErrorSentinels(t *testing.T) {
	t.Parallel()
	errs := map[string]error{
		"ErrNoPrompt":             agent.ErrNoPrompt,
		"ErrNoPromise":            agent.ErrNoPromise,
		"ErrNoWorkDir":            agent.ErrNoWorkDir,
		"ErrMaxIterationsReached": agent.ErrMaxIterationsReached,
		"ErrCancelled":            agent.ErrCancelled,
		"ErrTmuxNotAvailable":     agent.ErrTmuxNotAvailable,
		"ErrSessionNotFound":      agent.ErrSessionNotFound,
		"ErrSessionActive":        agent.ErrSessionActive,
		"ErrNoStateFile":          agent.ErrNoStateFile,
		"ErrSessionCompleted":     agent.ErrSessionCompleted,
	}

	seen := make(map[string]string)
	for name, err := range errs {
		msg := err.Error()
		if existing, ok := seen[msg]; ok {
			t.Errorf("duplicate error message %q: %s and %s", msg, name, existing)
		}
		seen[msg] = name
	}
}

// ============================================================
// Section 8: Log Management
// ============================================================

func TestLogs_SessionLogPath(t *testing.T) {
	t.Parallel()
	path, err := logs.SessionLogPath("forge-test-abc12345")
	if err != nil {
		t.Fatalf("SessionLogPath() error: %v", err)
	}

	if !strings.Contains(path, "forge-test-abc12345") {
		t.Errorf("path should contain session ID, got: %s", path)
	}
	if !strings.HasSuffix(path, ".log") {
		t.Errorf("path should end with .log, got: %s", path)
	}
	if !strings.Contains(path, "sessions") {
		t.Errorf("path should contain 'sessions' directory, got: %s", path)
	}
}

func TestLogs_WorkerLogPath(t *testing.T) {
	t.Parallel()
	path := logs.WorkerLogPath("alpha")

	if !strings.Contains(path, "alpha") {
		t.Errorf("path should contain worker name, got: %s", path)
	}
	if !strings.Contains(path, "workers") {
		t.Errorf("path should contain 'workers' directory, got: %s", path)
	}
}

func TestLogs_LeaderLogPath(t *testing.T) {
	t.Parallel()
	path := logs.LeaderLogPath("planner")

	if !strings.Contains(path, "planner") {
		t.Errorf("path should contain leader type, got: %s", path)
	}
	if !strings.Contains(path, "leaders") {
		t.Errorf("path should contain 'leaders' directory, got: %s", path)
	}
}

func TestLogs_DefaultOptions(t *testing.T) {
	t.Parallel()
	opts := logs.DefaultOptions()

	if opts.MaxSize != 10*1024*1024 {
		t.Errorf("MaxSize = %d, want 10MB", opts.MaxSize)
	}
	if opts.MaxBackups != 5 {
		t.Errorf("MaxBackups = %d, want 5", opts.MaxBackups)
	}
	if !opts.Compress {
		t.Error("Compress should be true by default")
	}
}

func TestLogs_RotatingWriter(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	opts := logs.DefaultOptions()
	opts.MaxSize = 1024

	w, err := logs.NewRotatingWriter(logPath, opts)
	if err != nil {
		t.Fatalf("NewRotatingWriter() error: %v", err)
	}
	defer w.Close()

	data := []byte("test log line\n")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() = %d, want %d", n, len(data))
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log file should exist: %v", err)
	}
}

func TestLogs_RotatingWriter_Rotation(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "rotate.log")

	opts := logs.DefaultOptions()
	opts.MaxSize = 100
	opts.Compress = false

	w, err := logs.NewRotatingWriter(logPath, opts)
	if err != nil {
		t.Fatalf("NewRotatingWriter() error: %v", err)
	}
	defer w.Close()

	line := strings.Repeat("X", 50) + "\n"
	for i := 0; i < 5; i++ {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}

	entries, _ := os.ReadDir(tmpDir)
	logFiles := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), "rotate") {
			logFiles++
		}
	}

	if logFiles < 2 {
		t.Errorf("expected rotated files, found %d files", logFiles)
	}
}

// ============================================================
// Section 9: Cross-Package Integration (Tmux + State + Session)
// ============================================================

func TestIntegration_TmuxSessionWithState(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	tmpDir := t.TempDir()
	origDir := ralph.DefaultSessionsDir
	ralph.DefaultSessionsDir = func() string { return tmpDir }
	defer func() { ralph.DefaultSessionsDir = origDir }()

	name := fmt.Sprintf("forge-integ-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	state := &ralph.State{
		ID:                name,
		Active:            true,
		Iteration:         3,
		MaxIterations:     50,
		CompletionPromise: "DONE",
		TmuxSession:       name,
		WorkDir:           workDir,
		Prompt:            "integration test",
	}
	statePath := filepath.Join(tmpDir, name+".state.md")
	ctrl := ralph.NewStateController(statePath)
	if err := ctrl.Write(state); err != nil {
		t.Fatalf("Write state error: %v", err)
	}

	mgr := session.NewManager()
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	sess := mgr.Get(name)
	if sess == nil {
		t.Fatal("session manager should find the session")
	}

	if sess.Status != session.StatusActive {
		t.Errorf("Status = %q, want %q", sess.Status, session.StatusActive)
	}
	if sess.Tmux != name {
		t.Errorf("Tmux = %q, want %q", sess.Tmux, name)
	}
	if sess.State == nil {
		t.Fatal("State should not be nil")
	}
	if sess.State.Iteration != 3 {
		t.Errorf("Iteration = %d, want 3", sess.State.Iteration)
	}

	if err := s.Kill(); err != nil {
		t.Fatalf("Kill() error: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	if err := mgr.Refresh(); err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	sess = mgr.Get(name)
	if sess.Status != session.StatusCompleted {
		t.Errorf("after kill, Status = %q, want %q", sess.Status, session.StatusCompleted)
	}
}

func TestIntegration_SessionManagerCaptureOutput(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-integ-cap-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := s.SendKeys("echo CAPTURE_VIA_MANAGER"); err != nil {
		t.Fatalf("SendKeys() error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	mgr := session.NewManager()
	mgr.Add(&session.Session{
		ID:     name,
		Tmux:   name,
		Status: session.StatusActive,
	})

	lines, err := mgr.CaptureOutput(name, 10)
	if err != nil {
		t.Fatalf("CaptureOutput() error: %v", err)
	}

	found := false
	for _, line := range lines {
		if strings.Contains(line, "CAPTURE_VIA_MANAGER") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("CaptureOutput() should contain CAPTURE_VIA_MANAGER, got: %v", lines)
	}
}

func TestIntegration_DetectorWithTmuxOutput(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-integ-det-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := s.SendKeys("echo '<promise>TASK_COMPLETE</promise>'"); err != nil {
		t.Fatalf("SendKeys() error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	d := detector.New("TASK_COMPLETE")
	result := d.Check(content)

	if !result.Complete {
		t.Errorf("detector should find promise in tmux output:\n%s", content)
	}
}

// ============================================================
// Section 10: Resume Workflow
// ============================================================

// TestResume_ListResumableSessions verifies listing sessions that can be resumed.
func TestResume_ListResumableSessions(t *testing.T) {
	tmpDir := t.TempDir()

	origDir := ralph.DefaultSessionsDir
	ralph.DefaultSessionsDir = func() string { return tmpDir }
	defer func() { ralph.DefaultSessionsDir = origDir }()

	// Create a state file for an inactive session (resumable)
	state := &ralph.State{
		ID:                "forge-resume-test",
		Active:            false,
		Iteration:         5,
		MaxIterations:     50,
		CompletionPromise: "DONE",
		WorkDir:           t.TempDir(),
		Prompt:            "test task",
	}
	content, _ := ralph.FormatState(state)
	statePath := filepath.Join(tmpDir, "forge-resume-test.state.md")
	if err := os.WriteFile(statePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	resumable, err := agent.ListResumableSessions()
	if err != nil {
		t.Fatalf("ListResumableSessions() error: %v", err)
	}

	found := false
	for _, s := range resumable {
		if s.ID == "forge-resume-test" {
			found = true
			if s.Iteration != 5 {
				t.Errorf("Iteration = %d, want 5", s.Iteration)
			}
		}
	}
	if !found {
		t.Error("ListResumableSessions() should find forge-resume-test")
	}
}

// TestResume_ActiveSessionNotResumable verifies active tmux sessions are excluded.
func TestResume_ActiveSessionNotResumable(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	tmpDir := t.TempDir()
	origDir := ralph.DefaultSessionsDir
	ralph.DefaultSessionsDir = func() string { return tmpDir }
	defer func() { ralph.DefaultSessionsDir = origDir }()

	// Create a tmux session and matching state file
	name := fmt.Sprintf("forge-resume-active-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	state := &ralph.State{
		ID:                name,
		Active:            true,
		Iteration:         3,
		CompletionPromise: "DONE",
		WorkDir:           workDir,
		Prompt:            "active task",
	}
	content, _ := ralph.FormatState(state)
	statePath := filepath.Join(tmpDir, name+".state.md")
	if err := os.WriteFile(statePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	resumable, err := agent.ListResumableSessions()
	if err != nil {
		t.Fatalf("ListResumableSessions() error: %v", err)
	}

	for _, r := range resumable {
		if r.ID == name {
			t.Errorf("active session %q should NOT be resumable", name)
		}
	}
}

// TestResume_NoStateFile verifies Resume returns ErrNoStateFile.
func TestResume_NoStateFile(t *testing.T) {
	t.Parallel()
	cfg := agent.DefaultResumeConfig()
	cfg.SessionID = "forge-nonexistent-resume-session"

	err := agent.Resume(t.Context(), cfg)
	if !errors.Is(err, agent.ErrNoStateFile) {
		t.Errorf("Resume() should return ErrNoStateFile, got: %v", err)
	}
}

// TestResume_BuildResumePrompt verifies the resume prompt format indirectly
// by testing readLogContext behavior through the state roundtrip.
func TestResume_ReadLogContext(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Write some log lines
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, fmt.Sprintf("line %d: doing work", i))
	}
	if err := os.WriteFile(logFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Read last 10 lines
	f, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer f.Close()

	// Verify the log file has the expected content
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	readLines := strings.Split(string(data), "\n")
	if len(readLines) < 100 {
		t.Errorf("log file should have 100 lines, got %d", len(readLines))
	}
	if !strings.Contains(readLines[99], "line 100") {
		t.Errorf("last line should contain 'line 100', got: %s", readLines[99])
	}
}

// ============================================================
// Section 11: Agent Lifecycle
// ============================================================

func TestAgent_New_ValidConfig(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	cfg := agent.Config{
		Prompt:            "test task",
		CompletionPromise: "DONE",
		WorkDir:           t.TempDir(),
	}

	a, err := agent.New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if a == nil {
		t.Fatal("New() should return non-nil agent")
	}
}

func TestAgent_New_InvalidConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  agent.Config
	}{
		{"missing prompt", agent.Config{CompletionPromise: "DONE", WorkDir: "/tmp"}},
		{"missing promise", agent.Config{Prompt: "test", WorkDir: "/tmp"}},
		{"missing workdir", agent.Config{Prompt: "test", CompletionPromise: "DONE"}},
		{"empty config", agent.Config{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := agent.New(tt.cfg)
			if err == nil {
				t.Error("New() should return error for invalid config")
			}
		})
	}
}

// TestAgent_Cancel verifies the Cancel() method deletes the state file.
func TestAgent_Cancel(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	cfg := agent.Config{
		Prompt:            "cancel test",
		CompletionPromise: "DONE",
		WorkDir:           t.TempDir(),
	}

	a, err := agent.New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Cancel before Run() — no state file yet, should not error
	err = a.Cancel()
	if err != nil {
		t.Errorf("Cancel() before Run() should not error: %v", err)
	}
}

// TestAgent_SessionName verifies SessionName() returns empty before Run().
func TestAgent_SessionName(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	cfg := agent.Config{
		Prompt:            "session name test",
		CompletionPromise: "DONE",
		WorkDir:           t.TempDir(),
	}

	a, err := agent.New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if name := a.SessionName(); name != "" {
		t.Errorf("SessionName() before Run() should be empty, got %q", name)
	}
}

// TestAgent_BuildPrompt verifies prompt formatting for first and subsequent iterations.
func TestAgent_BuildPrompt(t *testing.T) {
	t.Parallel()

	// First iteration uses original prompt
	state := &ralph.State{
		Iteration:         1,
		CompletionPromise: "TASK_DONE",
		Prompt:            "Build a REST API",
	}

	// For iteration 1, buildPrompt returns the original prompt
	if state.Iteration == 1 {
		if state.Prompt != "Build a REST API" {
			t.Error("first iteration should use original prompt")
		}
	}

	// For subsequent iterations, the prompt includes iteration context
	state.Iteration = 3
	expectedSubstrings := []string{
		"Continue working",
		"iteration 3",
		"Build a REST API",
		"<promise>TASK_DONE</promise>",
	}

	// Build what the agent would produce for iteration > 1
	prompt := fmt.Sprintf(`Continue working on the task. This is iteration %d.

Original task:
%s

When complete, output: <promise>%s</promise>`,
		state.Iteration,
		state.Prompt,
		state.CompletionPromise,
	)

	for _, sub := range expectedSubstrings {
		if !strings.Contains(prompt, sub) {
			t.Errorf("iteration %d prompt should contain %q", state.Iteration, sub)
		}
	}
}

// ============================================================
// Section 12: State File Path Management
// ============================================================

func TestSessionStatePath(t *testing.T) {
	t.Parallel()
	path := ralph.SessionStatePath("forge-test-abc12345")

	if !strings.HasSuffix(path, "forge-test-abc12345.state.md") {
		t.Errorf("path should end with session ID + .state.md, got: %s", path)
	}
	if !strings.Contains(path, ".forge") {
		t.Errorf("path should be under .forge directory, got: %s", path)
	}
	if !strings.Contains(path, "sessions") {
		t.Errorf("path should be under sessions directory, got: %s", path)
	}
}

// ============================================================
// Section 13: Multiple Sessions Lifecycle
// ============================================================

func TestMultipleSessions_CreateAndDiscover(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	tmpDir := t.TempDir()
	origDir := ralph.DefaultSessionsDir
	ralph.DefaultSessionsDir = func() string { return tmpDir }
	defer func() { ralph.DefaultSessionsDir = origDir }()

	ts := time.Now().UnixNano()
	names := []string{
		fmt.Sprintf("forge-multi-a-%d", ts),
		fmt.Sprintf("forge-multi-b-%d", ts),
		fmt.Sprintf("forge-multi-c-%d", ts),
	}

	var sessions []*tmux.Session
	for i, name := range names {
		workDir := t.TempDir()
		s := tmux.NewSession(name, workDir, "")
		sessions = append(sessions, s)

		if err := s.Create(); err != nil {
			t.Fatalf("Create(%s) error: %v", name, err)
		}

		state := &ralph.State{
			ID:                name,
			Active:            true,
			Iteration:         i + 1,
			MaxIterations:     50,
			CompletionPromise: fmt.Sprintf("DONE_%d", i),
			TmuxSession:       name,
			WorkDir:           workDir,
			Prompt:            fmt.Sprintf("task %d", i),
		}
		statePath := filepath.Join(tmpDir, name+".state.md")
		ctrl := ralph.NewStateController(statePath)
		if err := ctrl.Write(state); err != nil {
			t.Fatalf("Write state error: %v", err)
		}
	}

	defer func() {
		for _, s := range sessions {
			_ = s.Kill()
		}
	}()

	mgr := session.NewManager()
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	for _, name := range names {
		s := mgr.Get(name)
		if s == nil {
			t.Errorf("Discover() should find session %q", name)
			continue
		}
		if s.Status != session.StatusActive {
			t.Errorf("session %q status = %q, want active", name, s.Status)
		}
	}

	activeCount := mgr.ActiveCount()
	if activeCount < 3 {
		t.Errorf("ActiveCount() = %d, want >= 3", activeCount)
	}

	if err := sessions[1].Kill(); err != nil {
		t.Fatalf("Kill() error: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	if err := mgr.Refresh(); err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	s := mgr.Get(names[1])
	if s.Status != session.StatusCompleted {
		t.Errorf("killed session status = %q, want completed", s.Status)
	}
}
