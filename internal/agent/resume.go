package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zach-source/forge/internal/detector"
	"github.com/zach-source/forge/internal/logs"
	"github.com/zach-source/forge/internal/mcp"
	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/tmux"
)

var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionActive    = errors.New("session is already active")
	ErrNoStateFile      = errors.New("no state file found")
	ErrSessionCompleted = errors.New("session completed successfully, nothing to resume")
)

// ResumeConfig holds configuration for resuming a session.
type ResumeConfig struct {
	// SessionID is the session to resume
	SessionID string
	// MCPServers is the list of MCP servers to enable
	MCPServers []string
	// MCPConfigPath is a custom path to an MCP config file
	MCPConfigPath string
	// SkipPermissions enables --dangerously-skip-permissions
	SkipPermissions bool
	// ContextLines is how many lines from the log to include as context
	ContextLines int
	// Timeout is the maximum time to wait for Claude per iteration
	Timeout time.Duration
}

// DefaultResumeConfig returns a ResumeConfig with sensible defaults.
func DefaultResumeConfig() ResumeConfig {
	return ResumeConfig{
		MCPServers:      []string{},
		SkipPermissions: true,
		ContextLines:    50,
		Timeout:         30 * time.Minute,
	}
}

// Resume resumes an interrupted session from its state file.
func Resume(ctx context.Context, cfg ResumeConfig) error {
	// Load the state file
	statePath := ralph.SessionStatePath(cfg.SessionID)
	ctrl := ralph.NewStateController(statePath)

	if !ctrl.Exists() {
		return ErrNoStateFile
	}

	state, err := ctrl.Read()
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}

	// Check if session is already active (tmux session exists)
	tmuxSession := tmux.NewSession(cfg.SessionID, state.WorkDir, state.LogFile)
	if tmuxSession.Exists() {
		return ErrSessionActive
	}

	// Create the agent
	a := &Agent{
		config: Config{
			Prompt:            state.Prompt,
			CompletionPromise: state.CompletionPromise,
			MaxIterations:     state.MaxIterations,
			WorkDir:           state.WorkDir,
			MCPServers:        cfg.MCPServers,
			MCPConfigPath:     cfg.MCPConfigPath,
			SkipPermissions:   cfg.SkipPermissions,
			SessionID:         cfg.SessionID,
			Timeout:           cfg.Timeout,
			LogFile:           state.LogFile,
		},
		state:    ctrl,
		detector: detector.New(state.CompletionPromise),
	}

	return a.runResume(ctx, state, cfg.ContextLines)
}

// runResume continues the iteration loop from a resumed state.
func (a *Agent) runResume(ctx context.Context, state *ralph.State, contextLines int) error {
	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Generate MCP config
	mcpConfig := mcp.NewConfigurator()
	if a.config.MCPConfigPath != "" {
		mcpConfig.WithBaseConfig(a.config.MCPConfigPath)
	}
	if len(a.config.MCPServers) > 0 {
		mcpConfig.WithServers(a.config.MCPServers)
	}

	mcpPath, err := mcpConfig.Generate()
	if err != nil {
		return fmt.Errorf("generating MCP config: %w", err)
	}
	a.mcpPath = mcpPath
	defer os.Remove(mcpPath)

	// Get log file path
	logFile := state.LogFile
	if logFile == "" {
		logFile, err = logs.SessionLogPath(state.ID)
		if err != nil {
			return fmt.Errorf("getting session log path: %w", err)
		}
	}

	// Read context from previous log
	logContext := readLogContext(logFile, contextLines)

	// Create new tmux session
	a.session = tmux.NewSession(state.ID, state.WorkDir, logFile)
	if err := a.session.Create(); err != nil {
		return fmt.Errorf("creating tmux session: %w", err)
	}

	// Mark session as active again
	state.Active = true
	state.TmuxSession = a.session.Name

	// Write updated state
	if err := a.state.Write(state); err != nil {
		return fmt.Errorf("writing state: %w", err)
	}

	fmt.Printf("🔄 Resuming session: %s\n", a.session.Name)
	fmt.Printf("   Previous iteration: %d\n", state.Iteration)
	fmt.Printf("   Attach with: forge attach\n")
	fmt.Printf("   Promise: %q\n\n", state.CompletionPromise)

	// Run the iteration loop
	for {
		select {
		case <-ctx.Done():
			return a.handleCancel(state)
		default:
		}

		// Check iteration limit
		if a.config.MaxIterations > 0 && state.Iteration >= a.config.MaxIterations {
			return a.handleMaxIterations(state)
		}

		state.Iteration++
		fmt.Printf("📍 Iteration %d", state.Iteration)
		if a.config.MaxIterations > 0 {
			fmt.Printf("/%d", a.config.MaxIterations)
		}
		fmt.Printf(" (resumed)\n")

		// Update state file
		if err := a.state.Write(state); err != nil {
			fmt.Printf("Warning: failed to update state: %v\n", err)
		}

		// Build resume prompt with context
		prompt := a.buildResumePrompt(state, logContext)
		// Clear log context after first iteration (it's now been injected)
		logContext = ""

		// Run Claude in tmux
		if err := a.session.RunClaude(prompt, a.mcpPath, a.config.SkipPermissions); err != nil {
			return fmt.Errorf("running Claude: %w", err)
		}

		// Wait for Claude to exit
		output, err := a.session.WaitForClaudeExit(a.config.Timeout)
		if err != nil {
			fmt.Printf("Warning: error waiting for Claude: %v\n", err)
		}

		// Check for completion
		result := a.detector.Check(output)
		if result.Complete {
			return a.handleComplete(state, result.Promises)
		}

		// If session is gone and we have output, it might have exited cleanly
		if !a.session.Exists() {
			if output != "" {
				fmt.Printf("Warning: session ended without completion promise\n")
			}
			return fmt.Errorf("tmux session ended unexpectedly")
		}

		// Check if state file was deleted (external cancel)
		if !a.state.Exists() {
			return a.handleCancel(state)
		}

		// Small delay before next iteration
		time.Sleep(1 * time.Second)
	}
}

// buildResumePrompt builds the prompt for a resumed session.
func (a *Agent) buildResumePrompt(state *ralph.State, logContext string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("This is iteration %d, resuming from a previous session.\n\n", state.Iteration))

	if logContext != "" {
		sb.WriteString("## Previous Session Context\n")
		sb.WriteString("Here is the recent output from the previous session to help you continue:\n\n")
		sb.WriteString("```\n")
		sb.WriteString(logContext)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("## Original Task\n")
	sb.WriteString(state.Prompt)
	sb.WriteString("\n\n")

	sb.WriteString("Please continue working on this task from where you left off. ")
	sb.WriteString(fmt.Sprintf("When complete, output: <promise>%s</promise>", state.CompletionPromise))

	return sb.String()
}

// readLogContext reads the last n lines from a log file.
func readLogContext(logFile string, n int) string {
	if n <= 0 {
		return ""
	}

	f, err := os.Open(logFile)
	if err != nil {
		return ""
	}
	defer f.Close()

	// Read all lines and keep last n
	var lines []string
	scanner := bufio.NewScanner(f)
	// Increase buffer size for potentially long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) == 0 {
		return ""
	}

	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}

	return strings.Join(lines[start:], "\n")
}

// ListResumableSessions returns sessions that can be resumed.
// A session is resumable if:
// - It has a state file
// - It is not currently active (no tmux session)
func ListResumableSessions() ([]*ralph.State, error) {
	stateFiles, err := ralph.ListSessionStateFiles()
	if err != nil {
		return nil, err
	}

	var resumable []*ralph.State

	for _, path := range stateFiles {
		ctrl := ralph.NewStateController(path)
		state, err := ctrl.Read()
		if err != nil {
			continue
		}

		// Check if tmux session exists
		tmuxSession := tmux.NewSession(state.ID, "", "")
		if tmuxSession.Exists() {
			// Session is active, not resumable
			continue
		}

		// Session can be resumed
		resumable = append(resumable, state)
	}

	return resumable, nil
}
