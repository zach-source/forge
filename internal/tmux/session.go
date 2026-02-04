// Package tmux provides tmux session management for forge agents.
package tmux

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSessionExists   = errors.New("tmux session already exists")
	ErrSessionNotFound = errors.New("tmux session not found")
	ErrTmuxNotFound    = errors.New("tmux not found in PATH")
)

// Session represents a tmux session for running a forge agent.
type Session struct {
	Name    string
	WorkDir string
	LogFile string
}

// NewSession creates a new tmux session configuration.
func NewSession(name, workDir, logFile string) *Session {
	return &Session{
		Name:    name,
		WorkDir: workDir,
		LogFile: logFile,
	}
}

// Create creates a new detached tmux session.
func (s *Session) Create() error {
	if !IsTmuxInstalled() {
		return ErrTmuxNotFound
	}

	if s.Exists() {
		return ErrSessionExists
	}

	// Create detached session with bash for simpler, faster startup
	args := []string{
		"new-session",
		"-d",         // Detached
		"-s", s.Name, // Session name
		"-c", s.WorkDir, // Working directory
		"bash", "--norc", "--noprofile", // Use bash with no config for speed
	}

	cmd := exec.Command("tmux", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating tmux session: %w", err)
	}

	// Set up pipe-pane for logging if log file specified
	if s.LogFile != "" {
		pipeCmd := exec.Command("tmux", "pipe-pane", "-t", s.Name, "-o", fmt.Sprintf("cat >> %s", s.LogFile))
		if err := pipeCmd.Run(); err != nil {
			// Non-fatal, just log
			fmt.Printf("Warning: failed to set up pipe-pane: %v\n", err)
		}
	}

	return nil
}

// Exists returns true if the tmux session exists.
func (s *Session) Exists() bool {
	cmd := exec.Command("tmux", "has-session", "-t", s.Name)
	return cmd.Run() == nil
}

// Kill terminates the tmux session.
func (s *Session) Kill() error {
	if !s.Exists() {
		return nil // Already gone
	}

	cmd := exec.Command("tmux", "kill-session", "-t", s.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("killing tmux session: %w", err)
	}
	return nil
}

// SendKeys sends keys to the tmux session.
func (s *Session) SendKeys(keys string) error {
	if !s.Exists() {
		return ErrSessionNotFound
	}

	cmd := exec.Command("tmux", "send-keys", "-t", s.Name, keys, "Enter")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sending keys to tmux: %w", err)
	}
	return nil
}

// RunCommand runs a command in the tmux session.
func (s *Session) RunCommand(command string) error {
	return s.SendKeys(command)
}

// RunClaude runs a Claude command in the tmux session.
// For long prompts, uses a temp file to avoid tmux send-keys limitations.
func (s *Session) RunClaude(prompt, mcpConfig string, skipPermissions bool) error {
	// Write prompt to temp file for reliability with long/complex prompts
	promptFile, err := os.CreateTemp("", "forge-prompt-*.txt")
	if err != nil {
		return fmt.Errorf("creating prompt file: %w", err)
	}
	promptPath := promptFile.Name()

	if _, err := promptFile.WriteString(prompt); err != nil {
		promptFile.Close()
		os.Remove(promptPath)
		return fmt.Errorf("writing prompt file: %w", err)
	}
	promptFile.Close()

	// Build claude command using the temp file
	var cmdParts []string
	cmdParts = append(cmdParts, "claude", "-p", fmt.Sprintf("\"$(cat %s)\"", promptPath))

	if skipPermissions {
		cmdParts = append(cmdParts, "--dangerously-skip-permissions")
	}

	if mcpConfig != "" {
		cmdParts = append(cmdParts, "--mcp-config", shellQuote(mcpConfig))
	}

	// Add allowed tools
	cmdParts = append(cmdParts, "--allowedTools", `"*"`)

	// Add cleanup of temp file after command starts
	cmdParts = append(cmdParts, fmt.Sprintf("; rm -f %s", promptPath))

	cmd := strings.Join(cmdParts, " ")
	return s.SendKeys(cmd)
}

// CapturePane captures the current pane content.
func (s *Session) CapturePane() (string, error) {
	if !s.Exists() {
		return "", ErrSessionNotFound
	}

	cmd := exec.Command("tmux", "capture-pane", "-t", s.Name, "-p")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("capturing pane: %w", err)
	}

	return out.String(), nil
}

// CapturePaneLines captures the last n lines from the pane.
func (s *Session) CapturePaneLines(n int) ([]string, error) {
	if !s.Exists() {
		return nil, ErrSessionNotFound
	}

	// Use -S to start from n lines before the end
	cmd := exec.Command("tmux", "capture-pane", "-t", s.Name, "-p", "-S", fmt.Sprintf("-%d", n))
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("capturing pane: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}

// WaitForPrompt waits for the shell prompt to return (indicating command finished).
// It polls the pane content looking for a prompt pattern.
func (s *Session) WaitForPrompt(timeout time.Duration, pollInterval time.Duration) error {
	if !s.Exists() {
		return ErrSessionNotFound
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Check if pane is idle by looking at cursor position
		// When a command is running, the cursor is usually at the bottom
		// When it finishes, we see a prompt

		content, err := s.CapturePane()
		if err != nil {
			return err
		}

		// Look for common shell prompt patterns at the end
		lines := strings.Split(strings.TrimSpace(content), "\n")
		if len(lines) > 0 {
			lastLine := lines[len(lines)-1]
			// Check for common prompt endings
			if strings.HasSuffix(lastLine, "$ ") ||
				strings.HasSuffix(lastLine, "# ") ||
				strings.HasSuffix(lastLine, "> ") ||
				strings.HasSuffix(lastLine, "% ") {
				return nil
			}
		}

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("timeout waiting for prompt")
}

// WaitForClaudeExit waits for Claude to exit by monitoring the pane.
func (s *Session) WaitForClaudeExit(timeout time.Duration) (string, error) {
	start := time.Now()
	pollInterval := 2 * time.Second

	var initialContent string
	var lastContent string
	var lastValidContent string
	stableCount := 0
	firstPoll := true

	for time.Now().Sub(start) < timeout {
		content, err := s.CapturePane()
		if err != nil {
			// Session disappeared - use last valid content if we have it
			if lastValidContent != "" {
				return lastValidContent, nil
			}
			return "", err
		}

		lastValidContent = content

		// Capture initial content on first poll
		if firstPoll {
			initialContent = content
			firstPoll = false
			time.Sleep(pollInterval)
			continue
		}

		lines := strings.Split(strings.TrimSpace(content), "\n")
		if len(lines) == 0 {
			time.Sleep(pollInterval)
			continue
		}

		lastLine := lines[len(lines)-1]

		// Claude has done something if content changed from initial
		contentChanged := content != initialContent

		// Check for shell prompt at the end - indicates Claude exited
		// We need content to have changed (meaning Claude ran and finished)
		if isShellPrompt(lastLine) && contentChanged {
			// Wait for stability to ensure Claude is really done
			if content == lastContent {
				stableCount++
				if stableCount >= 2 {
					return content, nil
				}
			} else {
				stableCount = 1
			}
		} else {
			stableCount = 0
		}

		lastContent = content
		time.Sleep(pollInterval)
	}

	return lastValidContent, fmt.Errorf("timeout waiting for Claude to exit")
}

// isShellPrompt checks if a line looks like a shell prompt.
func isShellPrompt(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	// Common prompt endings (including Unicode prompts like Starship's ❯)
	promptEndings := []string{
		"$ ", "# ", "> ", "% ",
		"$", "#", ">", "%",
		"❯", "➜", "→", "›", // Unicode prompt characters
		"❯ ", "➜ ", "→ ", "› ",
	}
	for _, ending := range promptEndings {
		if strings.HasSuffix(line, ending) {
			return true
		}
	}
	return false
}

// IsClaudeRunning checks if Claude appears to be running in the session.
// Uses tmux's pane_current_command to reliably detect if Claude is running.
func (s *Session) IsClaudeRunning() bool {
	if !s.Exists() {
		return false
	}

	// Get the current command running in the pane
	cmd := exec.Command("tmux", "list-panes", "-t", s.Name, "-F", "#{pane_current_command}")
	out, err := cmd.Output()
	if err != nil {
		return true // Assume running if we can't check
	}

	command := strings.TrimSpace(string(out))

	// Claude shows as version number (e.g., "2.1.30") or "claude"
	// Shell shows as "zsh", "bash", "sh", etc.
	shellCommands := []string{"zsh", "bash", "sh", "fish", "tcsh", "csh", "ksh"}
	for _, shell := range shellCommands {
		if command == shell {
			return false // Shell is running, Claude has exited
		}
	}

	return true // Claude or other process is running
}

// shellQuote quotes a string for safe shell usage.
func shellQuote(s string) string {
	// Use single quotes and escape any single quotes in the string
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// IsTmuxInstalled checks if tmux is available.
func IsTmuxInstalled() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// ListSessions returns all tmux session names matching a prefix.
// If prefix is empty, returns all sessions.
func ListSessions(prefix string) ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		// No sessions is not an error
		return nil, nil
	}

	var sessions []string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		name := scanner.Text()
		if prefix == "" || strings.HasPrefix(name, prefix) {
			sessions = append(sessions, name)
		}
	}
	return sessions, nil
}

// ListForgeSessions returns all tmux sessions matching the forge pattern.
func ListForgeSessions() ([]string, error) {
	return ListSessions("forge-")
}

// GetSessionInfo returns information about a tmux session.
type SessionInfo struct {
	Name     string
	Created  time.Time
	Attached bool
	Width    int
	Height   int
	Activity time.Time
}

// GetSessionInfo gets info about a specific session.
func GetSessionInfo(name string) (*SessionInfo, error) {
	format := "#{session_name}|#{session_created}|#{session_attached}|#{session_width}|#{session_height}|#{session_activity}"
	cmd := exec.Command("tmux", "list-sessions", "-F", format, "-f", fmt.Sprintf("#{==:#{session_name},%s}", name))
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, ErrSessionNotFound
	}

	line := strings.TrimSpace(out.String())
	if line == "" {
		return nil, ErrSessionNotFound
	}

	parts := strings.Split(line, "|")
	if len(parts) != 6 {
		return nil, fmt.Errorf("unexpected session info format")
	}

	created, _ := strconv.ParseInt(parts[1], 10, 64)
	attached := parts[2] == "1"
	width, _ := strconv.Atoi(parts[3])
	height, _ := strconv.Atoi(parts[4])
	activity, _ := strconv.ParseInt(parts[5], 10, 64)

	return &SessionInfo{
		Name:     parts[0],
		Created:  time.Unix(created, 0),
		Attached: attached,
		Width:    width,
		Height:   height,
		Activity: time.Unix(activity, 0),
	}, nil
}

// AttachSession attaches to a tmux session (blocking, for CLI use).
func AttachSession(name string) error {
	cmd := exec.Command("tmux", "attach", "-t", name)
	cmd.Stdin = nil // Will be set by the caller
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
