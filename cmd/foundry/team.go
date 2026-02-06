package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/kanban"
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

			// 7. Build starter prompt with project context
			starterPrompt := buildTeamStarterPrompt(workDir)

			// 8. Start Claude interactively with agent teams
			mode := teammateMode
			if mode == "" {
				mode = "tmux"
			}
			if err := session.RunClaudeInteractive(tmux.ClaudeTeamOptions{
				MCPConfig:    mcpPath,
				TeammateMode: mode,
				SystemPrompt: starterPrompt,
			}); err != nil {
				_ = session.Kill()
				return fmt.Errorf("starting Claude: %w", err)
			}

			fmt.Printf("🎯 Team session started: %s\n", name)
			fmt.Printf("   Reattach with: foundry team --attach\n")
			fmt.Printf("   Detach with: Ctrl+B d\n\n")

			// 9. Brief pause for Claude to initialize
			time.Sleep(500 * time.Millisecond)

			// 10. Attach user to session (replaces current process)
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

// buildTeamStarterPrompt builds a system prompt with project context for the team lead.
func buildTeamStarterPrompt(workDir string) string {
	var sb strings.Builder

	sb.WriteString("# Team Lead Context\n\n")
	sb.WriteString("You are a team lead coordinating development work using Claude Code Agent Teams.\n")
	sb.WriteString("You can spawn teammates to work in parallel on different tasks.\n\n")

	// Project info
	sb.WriteString("## Project\n\n")
	sb.WriteString(fmt.Sprintf("Working directory: %s\n", workDir))

	// Read CLAUDE.md if present
	claudeMD := findProjectFile(workDir, "CLAUDE.md")
	if claudeMD != "" {
		content, err := os.ReadFile(claudeMD)
		if err == nil && len(content) > 0 {
			sb.WriteString("\n### Project Instructions (from CLAUDE.md)\n\n")
			// Truncate if very long to avoid overwhelming the system prompt
			text := string(content)
			if len(text) > 4000 {
				text = text[:4000] + "\n\n... (truncated)"
			}
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	}

	// Git status
	gitInfo := captureCommand(workDir, "git", "branch", "--show-current")
	if gitInfo != "" {
		sb.WriteString("## Git\n\n")
		sb.WriteString(fmt.Sprintf("Branch: %s\n", strings.TrimSpace(gitInfo)))

		recentCommits := captureCommand(workDir, "git", "log", "--oneline", "-5")
		if recentCommits != "" {
			sb.WriteString("\nRecent commits:\n```\n")
			sb.WriteString(strings.TrimSpace(recentCommits))
			sb.WriteString("\n```\n\n")
		}
	}

	// Kanban board state
	sb.WriteString("## Kanban Board\n\n")
	store, err := kanban.NewStore(workDir)
	if err == nil {
		board, err := store.GetBoard()
		if err == nil && board != nil {
			hasIssues := false
			for _, col := range board.Columns {
				if len(col.Issues) > 0 {
					hasIssues = true
					sb.WriteString(fmt.Sprintf("### %s (%d)\n", col.Status, len(col.Issues)))
					for _, issue := range col.Issues {
						sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID, issue.Title))
					}
					sb.WriteString("\n")
				}
			}
			if !hasIssues {
				sb.WriteString("No issues on the board. Use `foundry kanban add` to create tasks.\n\n")
			}
		} else {
			sb.WriteString("Board not available. Use `foundry kanban` to manage tasks.\n\n")
		}
		_ = store.Close()
	}

	// Available roles
	sb.WriteString("## Available Teammate Roles\n\n")
	sb.WriteString("Spawn teammates for these roles as needed:\n\n")
	sb.WriteString("- **Groomer**: Research and detail backlog items, add acceptance criteria, break down large tasks\n")
	sb.WriteString("- **Reviewer**: Review completed work in the review queue, check code quality, run tests\n")
	sb.WriteString("- **Planner**: Plan next sprint, prioritize backlog, estimate complexity\n")
	sb.WriteString("- **PM**: Analyze completed tasks, identify patterns, suggest improvements\n")
	sb.WriteString("- **Worker**: Implement a specific task (coding, bug fixes, features)\n\n")

	// Instructions
	sb.WriteString("## How to Work\n\n")
	sb.WriteString("1. Assess the board state and decide which roles are needed\n")
	sb.WriteString("2. Spawn teammates with specific instructions and their work queue\n")
	sb.WriteString("3. You can also do work directly when it's faster than spawning a teammate\n")
	sb.WriteString("4. Use `foundry kanban` commands to manage task status\n")
	sb.WriteString("5. Coordinate between teammates when their work overlaps\n")

	return sb.String()
}

// findProjectFile looks for a file in the given directory or parent directories.
func findProjectFile(dir, name string) string {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// captureCommand runs a command and returns its stdout, or empty string on error.
func captureCommand(dir string, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
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
