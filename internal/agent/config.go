// Package agent provides the core agent loop for forge.
package agent

import (
	"time"
)

// Config holds configuration for an agent run.
type Config struct {
	// Prompt is the initial prompt for Claude
	Prompt string
	// CompletionPromise is the text that indicates completion
	CompletionPromise string
	// MaxIterations is the maximum number of iterations (0 = unlimited)
	MaxIterations int
	// WorkDir is the working directory for the session
	WorkDir string
	// MCPServers is the list of MCP servers to enable
	MCPServers []string
	// MCPConfigPath is a custom path to an MCP config file
	MCPConfigPath string
	// SkipPermissions enables --dangerously-skip-permissions
	SkipPermissions bool
	// SessionID is an optional custom session ID
	SessionID string
	// PollInterval is how often to check for Claude exit
	PollInterval time.Duration
	// Timeout is the maximum time to wait for Claude per iteration
	Timeout time.Duration

	// Worker identity fields (optional)
	// WorkerID is the worker's unique identifier
	WorkerID string
	// WorkerName is the worker's human-readable name
	WorkerName string
	// WorkerRole is the worker's role (worker, planner, etc.)
	WorkerRole string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxIterations:   50,
		MCPServers:      []string{"graphiti", "context7"},
		SkipPermissions: true,
		PollInterval:    2 * time.Second,
		Timeout:         30 * time.Minute,
	}
}

// Validate checks that the config is valid.
func (c *Config) Validate() error {
	if c.Prompt == "" {
		return ErrNoPrompt
	}
	if c.CompletionPromise == "" {
		return ErrNoPromise
	}
	if c.WorkDir == "" {
		return ErrNoWorkDir
	}
	return nil
}
