package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/zach-source/forge/internal/detector"
	"github.com/zach-source/forge/internal/mcp"
	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/tmux"
)

var (
	ErrNoPrompt             = errors.New("prompt is required")
	ErrNoPromise            = errors.New("completion promise is required")
	ErrNoWorkDir            = errors.New("working directory is required")
	ErrMaxIterationsReached = errors.New("maximum iterations reached")
	ErrCancelled            = errors.New("agent cancelled")
	ErrTmuxNotAvailable     = errors.New("tmux is not available")
)

// Agent orchestrates autonomous Claude sessions.
type Agent struct {
	config   Config
	state    *ralph.StateController
	detector *detector.Detector
	session  *tmux.Session
	mcpPath  string
}

// New creates a new agent with the given configuration.
func New(config Config) (*Agent, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if !tmux.IsTmuxInstalled() {
		return nil, ErrTmuxNotAvailable
	}

	return &Agent{
		config:   config,
		detector: detector.New(config.CompletionPromise),
	}, nil
}

// Run executes the agent loop until completion or max iterations.
func (a *Agent) Run(ctx context.Context) error {
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

	// 1. Generate session ID
	sessionID := a.config.SessionID
	if sessionID == "" {
		sessionID = session.GenerateSessionID(a.config.Prompt)
	}

	// 2. Initialize state
	state := &ralph.State{
		ID:                sessionID,
		Active:            true,
		Iteration:         0,
		MaxIterations:     a.config.MaxIterations,
		CompletionPromise: a.config.CompletionPromise,
		StartedAt:         time.Now(),
		WorkDir:           a.config.WorkDir,
		Prompt:            a.config.Prompt,
	}

	// Create state controller for persistent state
	statePath := ralph.SessionStatePath(sessionID)
	a.state = ralph.NewStateController(statePath)

	// 3. Generate MCP config
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

	// 4. Create tmux session
	logFile := filepath.Join(os.TempDir(), sessionID+".log")
	a.session = tmux.NewSession(sessionID, a.config.WorkDir, logFile)

	if err := a.session.Create(); err != nil {
		return fmt.Errorf("creating tmux session: %w", err)
	}

	state.TmuxSession = a.session.Name
	state.LogFile = logFile

	// Write initial state
	if err := a.state.Write(state); err != nil {
		return fmt.Errorf("writing initial state: %w", err)
	}

	fmt.Printf("🚀 Agent started in tmux session: %s\n", a.session.Name)
	fmt.Printf("   Attach with: forge attach\n")
	fmt.Printf("   Promise: %q\n", a.config.CompletionPromise)
	fmt.Printf("   Max iterations: %d\n\n", a.config.MaxIterations)

	// 5. Run the iteration loop
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
		fmt.Println()

		// Update state file
		if err := a.state.Write(state); err != nil {
			fmt.Printf("Warning: failed to update state: %v\n", err)
		}

		// Build iteration prompt
		prompt := a.buildPrompt(state)

		// Run Claude in tmux
		if err := a.session.RunClaude(prompt, a.mcpPath, a.config.SkipPermissions); err != nil {
			return fmt.Errorf("running Claude: %w", err)
		}

		// Wait for Claude to exit
		output, err := a.session.WaitForClaudeExit(a.config.Timeout)
		if err != nil {
			fmt.Printf("Warning: error waiting for Claude: %v\n", err)
		}

		// Check for completion - even if we got an error, check the output we have
		result := a.detector.Check(output)
		if result.Complete {
			return a.handleComplete(state, result.Promises)
		}

		// If session is gone and we have output, it might have exited cleanly
		// but we missed detecting it - check one more time
		if !a.session.Exists() {
			if output != "" {
				// We have output but no completion - unusual, log it
				fmt.Printf("Warning: session ended without completion promise\n")
			}
			// Session is gone, can't continue
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

// buildPrompt builds the prompt for an iteration.
func (a *Agent) buildPrompt(state *ralph.State) string {
	if state.Iteration == 1 {
		// First iteration - use original prompt
		return state.Prompt
	}

	// Subsequent iterations - add context
	return fmt.Sprintf(`Continue working on the task. This is iteration %d.

Original task:
%s

When complete, output: <promise>%s</promise>`,
		state.Iteration,
		state.Prompt,
		state.CompletionPromise,
	)
}

// handleComplete handles successful completion.
func (a *Agent) handleComplete(state *ralph.State, promises []string) error {
	fmt.Printf("\n✅ Task completed! Promise matched: %v\n", promises)

	state.Active = false
	if err := a.state.Write(state); err != nil {
		fmt.Printf("Warning: failed to update final state: %v\n", err)
	}

	// Kill tmux session
	if err := a.session.Kill(); err != nil {
		fmt.Printf("Warning: failed to kill tmux session: %v\n", err)
	}

	return nil
}

// handleMaxIterations handles reaching the iteration limit.
func (a *Agent) handleMaxIterations(state *ralph.State) error {
	fmt.Printf("\n⚠️ Maximum iterations (%d) reached without completion\n", a.config.MaxIterations)

	state.Active = false
	if err := a.state.Write(state); err != nil {
		fmt.Printf("Warning: failed to update state: %v\n", err)
	}

	// Kill tmux session
	if err := a.session.Kill(); err != nil {
		fmt.Printf("Warning: failed to kill tmux session: %v\n", err)
	}

	return ErrMaxIterationsReached
}

// handleCancel handles cancellation.
func (a *Agent) handleCancel(state *ralph.State) error {
	fmt.Printf("\n❌ Agent cancelled\n")

	state.Active = false
	if err := a.state.Write(state); err != nil {
		// Might fail if file was deleted
		_ = err
	}

	// Kill tmux session
	if a.session != nil {
		if err := a.session.Kill(); err != nil {
			fmt.Printf("Warning: failed to kill tmux session: %v\n", err)
		}
	}

	return ErrCancelled
}

// SessionName returns the tmux session name.
func (a *Agent) SessionName() string {
	if a.session != nil {
		return a.session.Name
	}
	return ""
}

// Cancel stops the agent gracefully.
func (a *Agent) Cancel() error {
	if a.state != nil {
		return a.state.Delete()
	}
	return nil
}
