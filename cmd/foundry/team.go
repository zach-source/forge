package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/logs"
	"github.com/zach-source/forge/internal/mcp"
	"github.com/zach-source/forge/internal/tmux"
)

const teamSessionPrefix = "forge-team-"

func newTeamCmd() *cobra.Command {
	var (
		attach       bool
		teammateMode string
		mcpServers   string
		sessionName  string
	)

	cmd := &cobra.Command{
		Use:   "team",
		Short: "Launch an interactive agent team session",
		Long: `Start Claude with Agent Teams enabled for coordinating
development work. You interact directly with the team lead,
who can spawn teammates for grooming, reviewing, planning, etc.

Examples:
  foundry team                          # Start new team session
  foundry team --attach                 # Reattach to existing session
  foundry team --teammate-mode tmux     # Teammates in tmux panes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Determine workspace
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			// 2. If --attach, find and attach to existing session
			if attach {
				return attachToTeamSession(sessionName)
			}

			// 3. Build MCP config
			mcpPath, err := buildTeamMCPConfig(mcpServers)
			if err != nil {
				return fmt.Errorf("building MCP config: %w", err)
			}

			// 4. Generate session name
			name := sessionName
			if name == "" {
				hash := sha256.Sum256([]byte(workDir + time.Now().String()))
				name = teamSessionPrefix + hex.EncodeToString(hash[:])[:8]
			}

			// 5. Create log file
			logFile, err := logs.SessionLogPath(name)
			if err != nil {
				return fmt.Errorf("creating log path: %w", err)
			}

			// 6. Create tmux session
			session := tmux.NewSession(name, workDir, logFile)
			if err := session.Create(); err != nil {
				return fmt.Errorf("creating tmux session: %w", err)
			}

			// 7. Start Claude interactively with agent teams
			mode := teammateMode
			if mode == "" {
				mode = "tmux"
			}
			if err := session.RunClaudeInteractive(tmux.ClaudeTeamOptions{
				MCPConfig:    mcpPath,
				TeammateMode: mode,
			}); err != nil {
				_ = session.Kill()
				return fmt.Errorf("starting Claude: %w", err)
			}

			fmt.Printf("🎯 Team session started: %s\n", name)
			fmt.Printf("   Reattach with: foundry team --attach\n")
			fmt.Printf("   Detach with: Ctrl+B d\n\n")

			// 8. Brief pause for Claude to initialize
			time.Sleep(500 * time.Millisecond)

			// 9. Attach user to session (replaces current process)
			return attachToSession(name)
		},
	}

	cmd.Flags().BoolVarP(&attach, "attach", "a", false, "Reattach to existing team session")
	cmd.Flags().StringVar(&teammateMode, "teammate-mode", "tmux", "Teammate mode: tmux, in-process, auto")
	cmd.Flags().StringVar(&mcpServers, "mcp", "", "Additional MCP servers (comma-separated)")
	cmd.Flags().StringVar(&sessionName, "name", "", "Custom session name")

	return cmd
}

// attachToTeamSession finds and attaches to an existing team session.
func attachToTeamSession(name string) error {
	if name != "" {
		// Attach to specific session
		session := tmux.NewSession(name, "", "")
		if !session.Exists() {
			return fmt.Errorf("session %s not found", name)
		}
		return attachToSession(name)
	}

	// Find team sessions
	sessions, err := tmux.ListSessions(teamSessionPrefix)
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No active team sessions found.")
		fmt.Println("Start one with: foundry team")
		return nil
	}

	if len(sessions) == 1 {
		fmt.Printf("Attaching to %s...\n", sessions[0])
		return attachToSession(sessions[0])
	}

	// Multiple sessions - show list
	fmt.Println("Multiple team sessions running. Specify which to attach:")
	for _, s := range sessions {
		info, err := tmux.GetSessionInfo(s)
		if err != nil {
			fmt.Printf("  %s\n", s)
			continue
		}
		attached := ""
		if info.Attached {
			attached = " (attached)"
		}
		fmt.Printf("  %s - created %s%s\n", s, info.Created.Format("15:04:05"), attached)
	}
	fmt.Println("\nUse: foundry team --attach --name <session-name>")
	return nil
}

// attachToSession replaces the current process with tmux attach.
func attachToSession(name string) error {
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}

	// Use exec to replace current process
	cmd := exec.Command(tmuxPath, "-L", tmux.GetSocketName(), "attach", "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// buildTeamMCPConfig creates an MCP config for the team session.
func buildTeamMCPConfig(extraServers string) (string, error) {
	cfg := mcp.NewConfigurator()

	// Look for user's MCP config
	userConfig := mcp.FindUserMCPConfig()
	if userConfig != "" {
		cfg.WithBaseConfig(userConfig)
	}

	// Look for project-level .mcp.json
	projectConfig := filepath.Join(".mcp.json")
	if _, err := os.Stat(projectConfig); err == nil {
		cfg.WithBaseConfig(projectConfig)
	}

	// Add extra servers
	if extraServers != "" {
		cfg.WithServers(mcp.ParseServerList(extraServers))
	}

	return cfg.Generate()
}
