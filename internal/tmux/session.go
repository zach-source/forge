// Package tmux provides tmux session management for forge agents.
package tmux

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
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

	// Create detached session
	args := []string{
		"new-session",
		"-d",         // Detached
		"-s", s.Name, // Session name
		"-c", s.WorkDir, // Working directory
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
func (s *Session) RunClaude(prompt, mcpConfig string, skipPermissions bool) error {
	// Build claude command
	var cmdParts []string
	cmdParts = append(cmdParts, "claude", "-p", shellQuote(prompt))

	if skipPermissions {
		cmdParts = append(cmdParts, "--dangerously-skip-permissions")
	}

	if mcpConfig != "" {
		cmdParts = append(cmdParts, "--mcp-config", shellQuote(mcpConfig))
	}

	// Add allowed tools
	cmdParts = append(cmdParts, "--allowedTools", `"*"`)

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

	var lastContent string
	stableCount := 0

	for time.Now().Sub(start) < timeout {
		content, err := s.CapturePane()
		if err != nil {
			return "", err
		}

		// Check if Claude has exited by looking for shell prompt
		lines := strings.Split(strings.TrimSpace(content), "\n")
		if len(lines) > 0 {
			lastLine := lines[len(lines)-1]
			// Common prompt patterns indicating Claude exited
			if isShellPrompt(lastLine) && !strings.Contains(content, "claude") {
				return content, nil
			}
		}

		// Also check for content stability (no new output)
		if content == lastContent {
			stableCount++
			if stableCount > 5 && isShellPrompt(lines[len(lines)-1]) {
				return content, nil
			}
		} else {
			stableCount = 0
		}
		lastContent = content

		time.Sleep(pollInterval)
	}

	return lastContent, fmt.Errorf("timeout waiting for Claude to exit")
}

// isShellPrompt checks if a line looks like a shell prompt.
func isShellPrompt(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	// Common prompt endings
	return strings.HasSuffix(line, "$ ") ||
		strings.HasSuffix(line, "# ") ||
		strings.HasSuffix(line, "> ") ||
		strings.HasSuffix(line, "% ") ||
		strings.HasSuffix(line, "$") ||
		strings.HasSuffix(line, "#") ||
		strings.HasSuffix(line, ">") ||
		strings.HasSuffix(line, "%")
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

// ListForgeSessions returns all tmux sessions matching the forge pattern.
func ListForgeSessions() ([]string, error) {
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
		if strings.HasPrefix(name, "forge-") {
			sessions = append(sessions, name)
		}
	}
	return sessions, nil
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
