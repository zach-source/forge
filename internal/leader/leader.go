// Package leader provides shared infrastructure for forge leader commands.
// Leaders are specialized autonomous agents that coordinate specific aspects
// of the development workflow: planning, reviewing, merging, and deploying.
package leader

import (
	"context"
	"fmt"
	"os"

	"github.com/zach-source/forge/internal/agent"
)

// Role defines the type of leader.
type Role string

const (
	RolePlanner    Role = "planner"
	RoleReviewer   Role = "reviewer"
	RoleMerge      Role = "merge"
	RoleDeployment Role = "deployment"
	RoleGroomer    Role = "groomer"
	RoleMonitor    Role = "monitor"
	RoleTester     Role = "tester"
	RolePM         Role = "pm"
	RoleCICD       Role = "cicd"
)

// Config holds configuration for a leader session.
type Config struct {
	Role          Role
	WorkDir       string
	SessionID     string
	SkipPerms     bool
	MaxIterations int
	ExtraServers  []string

	// Role-specific options
	Branch      string // For merge/deploy: target branch
	Environment string // For deploy: target environment (staging, prod)
	DryRun      bool   // For merge/deploy: don't actually execute
}

// DefaultConfig returns sensible defaults for a leader.
func DefaultConfig(role Role) Config {
	return Config{
		Role:          role,
		MaxIterations: 100, // Leaders run longer
		SkipPerms:     true,
		Branch:        "main",
		Environment:   "staging",
	}
}

// MCPServers returns the MCP servers needed for a leader role.
// NOTE: Only include MCP servers that are known to be available.
// Unavailable servers will cause Claude to hang.
func MCPServers(role Role, extra []string) []string {
	var base []string

	// Role-specific MCP servers
	switch role {
	case RoleMerge, RoleDeployment:
		// These roles benefit from sequential thinking for complex decisions
		base = append(base, "sequential-thinking")
	}

	return append(base, extra...)
}

// Validate checks that the leader config is valid.
func (c *Config) Validate() error {
	if c.WorkDir == "" {
		return fmt.Errorf("working directory is required")
	}
	return nil
}

// Run executes a leader session.
func Run(ctx context.Context, cfg Config, prompt, promise string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Build agent config
	agentCfg := agent.DefaultConfig()
	agentCfg.Prompt = prompt
	agentCfg.CompletionPromise = promise
	agentCfg.MaxIterations = cfg.MaxIterations
	agentCfg.WorkDir = cfg.WorkDir
	agentCfg.MCPServers = MCPServers(cfg.Role, cfg.ExtraServers)
	agentCfg.SkipPermissions = cfg.SkipPerms
	agentCfg.SessionID = cfg.SessionID

	// Create and run agent
	a, err := agent.New(agentCfg)
	if err != nil {
		return err
	}

	// Print startup banner
	printBanner(cfg.Role)

	return a.Run(ctx)
}

// printBanner displays the startup message for a leader.
func printBanner(role Role) {
	icons := map[Role]string{
		RolePlanner:    "📋",
		RoleReviewer:   "🔍",
		RoleMerge:      "🔀",
		RoleDeployment: "🚀",
		RoleGroomer:    "🧹",
		RoleMonitor:    "📡",
		RoleTester:     "🧪",
		RolePM:         "📊",
	}

	names := map[Role]string{
		RolePlanner:    "Planner",
		RoleReviewer:   "Reviewer",
		RoleMerge:      "Merge Leader",
		RoleDeployment: "Deployment Leader",
		RoleGroomer:    "Backlog Groomer",
		RoleMonitor:    "Monitor",
		RoleTester:     "Tester",
		RolePM:         "Project Manager",
	}

	icon := icons[role]
	name := names[role]

	fmt.Printf("%s  Starting %s session...\n", icon, name)
	fmt.Printf("   Issue tracker: beads (.beads/)\n")
	fmt.Println()
}

// GetWorkDir returns the current working directory or exits on error.
func GetWorkDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	return wd, nil
}
