package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/zach-source/forge/internal/agent"
	"github.com/zach-source/forge/internal/logs"
	"github.com/zach-source/forge/internal/mcp"
	"github.com/zach-source/forge/internal/tmux"
)

// StartOptions configures worker startup.
type StartOptions struct {
	TaskID        string
	Worktree      string
	Prompt        string
	Promise       string
	MCPConfig     string
	MaxIterations int // 0 = use default (100), -1 = unlimited
}

// Start starts a worker with the given options.
func Start(ctx context.Context, reg *Registry, workerID string, opts StartOptions) error {
	w := reg.Get(workerID)
	if w == nil {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	if w.Status == StatusActive {
		return fmt.Errorf("worker %s is already active", w.DisplayName())
	}

	// Check worktree isn't already assigned
	if opts.Worktree != "" {
		existing := reg.FindByWorktree(opts.Worktree)
		if existing != nil && existing.ID != w.ID {
			return fmt.Errorf("worktree %s is already assigned to %s", opts.Worktree, existing.DisplayName())
		}
	}

	// Check single-threaded role locks
	if w.Role.IsSingleThreaded() {
		locked, holder := IsLocked(string(w.Role))
		if locked && holder != w.ID {
			return fmt.Errorf("%s role is locked by another worker", w.Role)
		}
		if err := AcquireLock(string(w.Role), w.ID); err != nil {
			return fmt.Errorf("acquiring %s lock: %w", w.Role, err)
		}
	}

	// Build session ID
	sessionID := w.TmuxSessionName()

	// Build promise
	promise := opts.Promise
	if promise == "" {
		promise = WorkerPromise(w)
	}

	// Build worker-aware prompt
	fullPrompt := opts.Prompt
	if opts.TaskID != "" {
		identity := WorkerIdentityPrompt(w, opts.TaskID, promise)
		fullPrompt = identity + "\n\n" + opts.Prompt
	}

	// Build agent config
	cfg := agent.DefaultConfig()
	cfg.Prompt = fullPrompt
	cfg.CompletionPromise = promise
	cfg.SessionID = sessionID
	cfg.SkipPermissions = true

	// Set max iterations (0 = default 100, -1 = unlimited)
	if opts.MaxIterations < 0 {
		cfg.MaxIterations = 0 // 0 means unlimited in agent config
	} else if opts.MaxIterations > 0 {
		cfg.MaxIterations = opts.MaxIterations
	} else {
		cfg.MaxIterations = 100 // default
	}

	if opts.Worktree != "" {
		cfg.WorkDir = opts.Worktree
	}

	// Add MCP servers
	cfg.MCPServers = WorkerMCPServers(w.Role)
	if opts.MCPConfig != "" {
		cfg.MCPConfigPath = opts.MCPConfig
	}

	// Set log file path
	cfg.LogFile = logs.WorkerLogPath(w.Name)

	// Update worker state
	if err := reg.Update(workerID, func(w *Worker) {
		w.Status = StatusActive
		w.SessionID = sessionID
		w.CurrentTask = opts.TaskID
		w.Worktree = opts.Worktree
	}); err != nil {
		return err
	}

	// Create and run agent
	a, err := agent.New(cfg)
	if err != nil {
		// Rollback state
		reg.Update(workerID, func(w *Worker) {
			w.Status = StatusIdle
			w.SessionID = ""
			w.CurrentTask = ""
			w.Worktree = ""
		})
		return err
	}

	fmt.Printf("%s  Worker %s starting...\n", w.RoleIcon(), w.DisplayName())
	fmt.Printf("   Session: %s\n", sessionID)
	if opts.TaskID != "" {
		fmt.Printf("   Task: %s\n", opts.TaskID)
	}
	if opts.Worktree != "" {
		fmt.Printf("   Worktree: %s\n", opts.Worktree)
	}
	fmt.Println()

	return a.Run(ctx)
}

// Pause suspends a running worker's tmux session.
func Pause(reg *Registry, workerID string) error {
	w := reg.Get(workerID)
	if w == nil {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	if w.Status != StatusActive {
		return fmt.Errorf("worker %s is not active", w.DisplayName())
	}

	// Check if tmux session exists
	session := tmux.NewSession(w.SessionID, "", "")
	if !session.Exists() {
		return fmt.Errorf("tmux session %s not found", w.SessionID)
	}

	// Send Ctrl+Z to suspend the foreground process
	if err := suspendTmuxSession(w.SessionID); err != nil {
		return fmt.Errorf("suspending session: %w", err)
	}

	// Update state
	return reg.Update(workerID, func(w *Worker) {
		w.Status = StatusPaused
	})
}

// Resume resumes a paused worker's tmux session.
func Resume(reg *Registry, workerID string) error {
	w := reg.Get(workerID)
	if w == nil {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	if w.Status != StatusPaused {
		return fmt.Errorf("worker %s is not paused", w.DisplayName())
	}

	// Check if tmux session exists
	session := tmux.NewSession(w.SessionID, "", "")
	if !session.Exists() {
		return fmt.Errorf("tmux session %s not found", w.SessionID)
	}

	// Send 'fg' to resume the foreground process
	if err := resumeTmuxSession(w.SessionID); err != nil {
		return fmt.Errorf("resuming session: %w", err)
	}

	// Update state
	return reg.Update(workerID, func(w *Worker) {
		w.Status = StatusActive
	})
}

// Stop terminates a worker's session and marks it as stopped.
func Stop(reg *Registry, workerID string) error {
	w := reg.Get(workerID)
	if w == nil {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	if w.Status == StatusIdle || w.Status == StatusStopped {
		return nil // Already stopped
	}

	// Kill tmux session if it exists
	if w.SessionID != "" {
		session := tmux.NewSession(w.SessionID, "", "")
		if session.Exists() {
			if err := session.Kill(); err != nil {
				fmt.Printf("Warning: failed to kill tmux session: %v\n", err)
			}
		}
	}

	// Release lock if single-threaded role
	if w.Role.IsSingleThreaded() {
		ReleaseLock(string(w.Role), w.ID)
	}

	// Update state
	return reg.Update(workerID, func(w *Worker) {
		w.Status = StatusStopped
		w.SessionID = ""
	})
}

// Reset resets a stopped worker to idle state.
func Reset(reg *Registry, workerID string) error {
	w := reg.Get(workerID)
	if w == nil {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	if w.Status == StatusActive {
		return fmt.Errorf("cannot reset active worker %s", w.DisplayName())
	}

	return reg.Update(workerID, func(w *Worker) {
		w.Status = StatusIdle
		w.CurrentTask = ""
		w.Worktree = ""
		w.SessionID = ""
	})
}

// Reassign moves a task from one worker to another.
func Reassign(reg *Registry, fromID, toID string) error {
	from := reg.Get(fromID)
	if from == nil {
		return fmt.Errorf("source worker not found: %s", fromID)
	}

	to := reg.Get(toID)
	if to == nil {
		return fmt.Errorf("target worker not found: %s", toID)
	}

	if !to.Status.IsAvailable() {
		return fmt.Errorf("target worker %s is not available", to.DisplayName())
	}

	// Stop the source worker first
	if from.Status == StatusActive || from.Status == StatusPaused {
		if err := Stop(reg, fromID); err != nil {
			return fmt.Errorf("stopping source worker: %w", err)
		}
	}

	// Update target worker with source's task/worktree
	return reg.Update(toID, func(w *Worker) {
		w.CurrentTask = from.CurrentTask
		w.Worktree = from.Worktree
	})
}

// WorkerMCPServers returns the MCP servers needed for a worker role.
// NOTE: MCP servers that are not running will cause Claude to hang, so
// only include servers that are known to be available.
func WorkerMCPServers(role Role) []string {
	// Empty base - MCP servers should be explicitly configured per-role
	// when they are known to be available
	var servers []string

	switch role {
	case RolePlanner:
		return append(servers, "sequential-thinking")
	case RoleMerge, RoleDeploy:
		return append(servers, "sequential-thinking")
	default:
		return servers
	}
}

// WorkerPromise returns the default completion promise for a worker.
func WorkerPromise(w *Worker) string {
	return fmt.Sprintf("WORKER_%s_COMPLETE", UpperName(w.Name))
}

// UpperName returns the uppercase version of a name.
func UpperName(name string) string {
	result := ""
	for _, r := range name {
		if r >= 'a' && r <= 'z' {
			result += string(r - 32)
		} else {
			result += string(r)
		}
	}
	return result
}

// suspendTmuxSession sends Ctrl+Z to suspend the foreground process.
func suspendTmuxSession(sessionName string) error {
	return tmux.SuspendSession(sessionName)
}

// resumeTmuxSession sends 'fg' to resume the foreground process.
func resumeTmuxSession(sessionName string) error {
	return tmux.ResumeSession(sessionName)
}

// GetStatus returns the current status of a worker's tmux session.
func GetStatus(w *Worker) (string, error) {
	if w.SessionID == "" {
		return "no session", nil
	}

	info, err := tmux.GetSessionInfo(w.SessionID)
	if err != nil {
		return "session not found", nil
	}

	status := "running"
	if info.Attached {
		status = "attached"
	}

	since := time.Since(info.Activity)
	if since > 5*time.Minute {
		status += fmt.Sprintf(" (idle %s)", formatDuration(since))
	}

	return status, nil
}

// CaptureOutput captures recent output from a worker's tmux session.
func CaptureOutput(w *Worker, lines int) ([]string, error) {
	if w.SessionID == "" {
		return nil, fmt.Errorf("no active session")
	}

	session := tmux.NewSession(w.SessionID, "", "")
	return session.CapturePaneLines(lines)
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// LogPath returns the path to a worker's current log file.
func LogPath(w *Worker) string {
	return logs.WorkerLogPath(w.Name)
}

// GetMCPConfigPath returns the path to a worker's MCP config file.
func GetMCPConfigPath(w *Worker) (string, error) {
	cfg := mcp.NewConfigurator()
	cfg.WithServers(WorkerMCPServers(w.Role))
	return cfg.Generate()
}
