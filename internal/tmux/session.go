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

// SocketName is the tmux socket name for forge sessions.
// Using a separate socket isolates forge sessions from user's regular tmux.
// Default is "forge" which creates socket at /tmp/tmux-<uid>/forge
var SocketName = "forge"

// tmuxCmd creates an exec.Command for tmux with the forge socket.
// All tmux operations should use this to ensure session isolation.
func tmuxCmd(args ...string) *exec.Cmd {
	// Prepend -L <socket> to use separate tmux server
	fullArgs := append([]string{"-L", SocketName}, args...)
	return exec.Command("tmux", fullArgs...)
}

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

	cmd := tmuxCmd(args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating tmux session: %w", err)
	}

	// Set up pipe-pane for logging if log file specified
	if s.LogFile != "" {
		// Use tee for better buffering and reliability
		pipeCmd := tmuxCmd("pipe-pane", "-t", s.Name, "-o", fmt.Sprintf("tee -a %s", s.LogFile))
		if err := pipeCmd.Run(); err != nil {
			return fmt.Errorf("setting up pipe-pane: %w", err)
		}

		// Increase history-limit as safety net for scrollback
		histCmd := tmuxCmd("set-option", "-t", s.Name, "history-limit", "50000")
		_ = histCmd.Run() // Non-fatal
	}

	return nil
}

// Exists returns true if the tmux session exists.
func (s *Session) Exists() bool {
	cmd := tmuxCmd("has-session", "-t", s.Name)
	return cmd.Run() == nil
}

// Kill terminates the tmux session.
func (s *Session) Kill() error {
	if !s.Exists() {
		return nil // Already gone
	}

	cmd := tmuxCmd("kill-session", "-t", s.Name)
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

	cmd := tmuxCmd("send-keys", "-t", s.Name, keys, "Enter")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sending keys to tmux: %w", err)
	}
	return nil
}

// RunCommand runs a command in the tmux session.
func (s *Session) RunCommand(command string) error {
	return s.SendKeys(command)
}

// RunClaudeOptions configures how Claude is launched in a tmux session.
type RunClaudeOptions struct {
	Prompt          string
	MCPConfig       string
	SkipPermissions bool
	AgentTeams      bool
	TeammateMode    string // "tmux", "in-process", "auto"
}

// RunClaude runs Claude in interactive mode in the tmux session.
// Uses a temp file and stdin redirect to pass the prompt reliably.
func (s *Session) RunClaude(prompt, mcpConfig string, skipPermissions bool) error {
	return s.RunClaudeWithOptions(RunClaudeOptions{
		Prompt:          prompt,
		MCPConfig:       mcpConfig,
		SkipPermissions: skipPermissions,
	})
}

// RunClaudeWithOptions runs Claude with extended options (agent teams, etc).
// Uses a temp file and stdin redirect to pass the prompt reliably (unattended Ralph loop mode).
func (s *Session) RunClaudeWithOptions(opts RunClaudeOptions) error {
	if !s.Exists() {
		return ErrSessionNotFound
	}

	// Write prompt to temp file for reliable delivery
	promptFile, err := os.CreateTemp("", "forge-prompt-*.txt")
	if err != nil {
		return fmt.Errorf("creating prompt file: %w", err)
	}
	promptPath := promptFile.Name()

	if _, err := promptFile.WriteString(opts.Prompt); err != nil {
		_ = promptFile.Close()
		_ = os.Remove(promptPath)
		return fmt.Errorf("writing prompt file: %w", err)
	}
	_ = promptFile.Close()

	// Build claude command (interactive mode, no -p flag)
	var cmdParts []string

	// Prepend agent teams env var if enabled
	if opts.AgentTeams {
		cmdParts = append(cmdParts, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1")
	}

	cmdParts = append(cmdParts, "claude")

	if opts.SkipPermissions {
		cmdParts = append(cmdParts, "--dangerously-skip-permissions")
	}
	if opts.MCPConfig != "" {
		cmdParts = append(cmdParts, "--mcp-config", shellQuote(opts.MCPConfig))
	}
	if opts.TeammateMode != "" {
		cmdParts = append(cmdParts, "--teammate-mode", opts.TeammateMode)
	}
	cmdParts = append(cmdParts, "--allowedTools", "'*'")

	// Pipe prompt from file to claude stdin, then clean up
	// Format: cat file | claude flags; rm file
	cmd := fmt.Sprintf("cat %s | %s; rm -f %s", promptPath, strings.Join(cmdParts, " "), promptPath)

	return s.SendKeys(cmd)
}

// ClaudeTeamOptions configures an interactive Claude session with agent teams.
type ClaudeTeamOptions struct {
	MCPConfig    string
	TeammateMode string // "tmux" (default for foundry), "in-process", "auto"
}

// RunClaudeInteractive starts Claude in full interactive mode (no stdin pipe).
// Used for foundry team sessions where the user interacts directly with Claude.
func (s *Session) RunClaudeInteractive(opts ClaudeTeamOptions) error {
	if !s.Exists() {
		return ErrSessionNotFound
	}

	// Build claude command for interactive use
	var cmdParts []string

	// Agent teams env var
	cmdParts = append(cmdParts, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1")
	cmdParts = append(cmdParts, "claude")

	if opts.MCPConfig != "" {
		cmdParts = append(cmdParts, "--mcp-config", shellQuote(opts.MCPConfig))
	}
	if opts.TeammateMode != "" {
		cmdParts = append(cmdParts, "--teammate-mode", opts.TeammateMode)
	}
	cmdParts = append(cmdParts, "--allowedTools", "'*'")

	// NO stdin pipe - Claude starts in full interactive mode
	// NO --dangerously-skip-permissions - user is present to approve
	return s.SendKeys(strings.Join(cmdParts, " "))
}

// CapturePane captures the current pane content.
func (s *Session) CapturePane() (string, error) {
	if !s.Exists() {
		return "", ErrSessionNotFound
	}

	cmd := tmuxCmd("capture-pane", "-t", s.Name, "-p")
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
	cmd := tmuxCmd("capture-pane", "-t", s.Name, "-p", "-S", fmt.Sprintf("-%d", n))
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

	for time.Since(start) < timeout {
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

		// Claude has done something if content changed from initial
		contentChanged := content != initialContent

		// Primary detection: Check if Claude process has exited via pane_current_command
		// This is more reliable than content-based detection because Claude Code's
		// status bar can appear below the shell prompt
		if contentChanged && !s.IsClaudeRunning() {
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
			// Fallback: Check for shell prompt in recent lines (not just last line)
			// Claude Code's status bar can render below the prompt
			lines := strings.Split(strings.TrimSpace(content), "\n")
			promptFound := false
			// Check last 10 lines for shell prompt
			startIdx := len(lines) - 10
			if startIdx < 0 {
				startIdx = 0
			}
			for i := startIdx; i < len(lines); i++ {
				if isShellPrompt(lines[i]) {
					promptFound = true
					break
				}
			}

			if promptFound && contentChanged {
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
// Checks both pane_current_command and child processes of the pane.
func (s *Session) IsClaudeRunning() bool {
	if !s.Exists() {
		return false
	}

	// Get the pane PID and current command
	cmd := tmuxCmd("list-panes", "-t", s.Name, "-F", "#{pane_current_command}|#{pane_pid}")
	out, err := cmd.Output()
	if err != nil {
		return true // Assume running if we can't check
	}

	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
	if len(parts) != 2 {
		return true // Unexpected format, assume running
	}

	command := parts[0]
	panePID := parts[1]

	// Claude shows as version number (e.g., "2.1.30") or "claude"
	// If current command is Claude, it's definitely running
	if strings.Contains(strings.ToLower(command), "claude") || strings.Contains(command, ".") {
		// Version numbers like "2.1.30" indicate claude is the foreground process
		if strings.Count(command, ".") >= 1 {
			return true
		}
	}

	// If current command is a shell, Claude might still be running as a child
	// (e.g., when using `cat file | claude`)
	shellCommands := []string{"zsh", "bash", "sh", "fish", "tcsh", "csh", "ksh"}
	isShell := false
	for _, shell := range shellCommands {
		if command == shell {
			isShell = true
			break
		}
	}

	if isShell && panePID != "" {
		// Check if claude is a child process of the pane
		// pgrep -P returns children of the given PID
		pgrepCmd := exec.Command("pgrep", "-P", panePID)
		childPIDs, err := pgrepCmd.Output()
		if err == nil && len(childPIDs) > 0 {
			// Check each child process
			for _, pidStr := range strings.Fields(string(childPIDs)) {
				psCmd := exec.Command("ps", "-p", pidStr, "-o", "comm=")
				commOut, err := psCmd.Output()
				if err == nil {
					comm := strings.TrimSpace(string(commOut))
					if strings.Contains(strings.ToLower(comm), "claude") {
						return true // Claude is a child process
					}
				}
			}
		}
		// No claude child process found, shell is idle
		return false
	}

	// Not a shell, something else is running - assume Claude
	return true
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
	cmd := tmuxCmd("list-sessions", "-F", "#{session_name}")
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
	cmd := tmuxCmd("list-sessions", "-F", format, "-f", fmt.Sprintf("#{==:#{session_name},%s}", name))
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
	cmd := tmuxCmd("attach", "-t", name)
	cmd.Stdin = nil // Will be set by the caller
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// AttachCmd returns an exec.Cmd for attaching to a session.
// Useful when the caller needs the command (e.g., for tea.ExecProcess).
func AttachCmd(name string) *exec.Cmd {
	return tmuxCmd("attach", "-t", name)
}

// SendKeysTo sends keys to a named session without requiring a Session struct.
func SendKeysTo(sessionName string, keys ...string) error {
	args := append([]string{"send-keys", "-t", sessionName}, keys...)
	cmd := tmuxCmd(args...)
	return cmd.Run()
}

// SuspendSession sends Ctrl+Z to suspend the foreground process in a session.
func SuspendSession(sessionName string) error {
	return SendKeysTo(sessionName, "C-z")
}

// ResumeSession sends 'fg' to resume the foreground process in a session.
func ResumeSession(sessionName string) error {
	return SendKeysTo(sessionName, "fg", "Enter")
}

// CapturePaneFrom captures pane content from a named session.
func CapturePaneFrom(sessionName string, lines int) ([]byte, error) {
	cmd := tmuxCmd("capture-pane", "-t", sessionName, "-p", "-S", fmt.Sprintf("-%d", lines))
	return cmd.Output()
}

// KillServer terminates the entire forge tmux server.
// This cleanly shuts down all forge sessions at once.
func KillServer() error {
	cmd := tmuxCmd("kill-server")
	return cmd.Run()
}

// ServerRunning checks if the forge tmux server is running.
func ServerRunning() bool {
	cmd := tmuxCmd("list-sessions")
	return cmd.Run() == nil
}

// GetSocketName returns the current tmux socket name.
func GetSocketName() string {
	return SocketName
}

// SetSocketName sets a custom tmux socket name.
// Call this before any tmux operations to use a different socket.
func SetSocketName(name string) {
	SocketName = name
}
