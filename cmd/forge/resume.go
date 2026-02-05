package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/agent"
	"github.com/zach-source/forge/internal/mcp"
)

func newResumeCmd() *cobra.Command {
	var (
		listResumable bool
		mcpServers    string
		mcpConfigPath string
		noSkip        bool
		contextLines  int
	)

	cmd := &cobra.Command{
		Use:   "resume [session-id]",
		Short: "Resume an interrupted session",
		Long: `Resume an interrupted forge session from where it left off.

Sessions are resumable when they have been interrupted (terminal closed,
system restart) but have not completed. The session state and log files
are used to restore context.

Example:
  forge resume                    # Resume most recent session
  forge resume --list             # List resumable sessions
  forge resume forge-api-12345678 # Resume specific session
  forge resume --context 100      # Include more log context`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if listResumable {
				return showResumableSessions()
			}

			var sessionID string

			if len(args) > 0 {
				sessionID = args[0]
			} else {
				// Find most recent resumable session
				sessions, err := agent.ListResumableSessions()
				if err != nil {
					return fmt.Errorf("listing sessions: %w", err)
				}

				if len(sessions) == 0 {
					fmt.Println("No resumable sessions found.")
					fmt.Println("Use 'forge list' to see all sessions.")
					return nil
				}

				// Sort by start time, most recent first
				sort.Slice(sessions, func(i, j int) bool {
					return sessions[i].StartedAt.After(sessions[j].StartedAt)
				})

				if len(sessions) == 1 {
					sessionID = sessions[0].ID
					fmt.Printf("Resuming session: %s\n", sessionID)
				} else {
					fmt.Println("Multiple resumable sessions found. Specify which to resume:")
					for _, s := range sessions {
						fmt.Printf("  %s - %s (iteration %d)\n",
							s.ID,
							truncateString(s.CompletionPromise, 30),
							s.Iteration,
						)
					}
					fmt.Println("\nOr use 'forge resume --list' for detailed information.")
					return nil
				}
			}

			// Parse MCP servers
			servers := mcp.ParseServerList(mcpServers)

			// Build resume config
			cfg := agent.DefaultResumeConfig()
			cfg.SessionID = sessionID
			cfg.MCPServers = servers
			cfg.MCPConfigPath = mcpConfigPath
			cfg.SkipPermissions = !noSkip
			cfg.ContextLines = contextLines

			err := agent.Resume(context.Background(), cfg)
			if err != nil {
				if errors.Is(err, agent.ErrSessionActive) {
					fmt.Printf("Session %s is already running.\n", sessionID)
					fmt.Println("Use 'forge attach' to attach to the running session.")
					return nil
				}
				if errors.Is(err, agent.ErrNoStateFile) {
					fmt.Printf("Session %s not found.\n", sessionID)
					fmt.Println("Use 'forge resume --list' to see resumable sessions.")
					return nil
				}
				if errors.Is(err, agent.ErrMaxIterationsReached) {
					fmt.Println("\nMax iterations reached. Use 'forge attach' to view the session.")
					return nil
				}
				if errors.Is(err, agent.ErrCancelled) {
					return nil
				}
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&listResumable, "list", "l", false, "List resumable sessions")
	cmd.Flags().StringVar(&mcpServers, "mcp", "", "Comma-separated MCP servers (e.g., graphiti,sequential-thinking)")
	cmd.Flags().StringVar(&mcpConfigPath, "mcp-config", "", "Path to custom MCP config file")
	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().IntVarP(&contextLines, "context", "c", 50, "Number of log lines to include as context")

	return cmd
}

func showResumableSessions() error {
	sessions, err := agent.ListResumableSessions()
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No resumable sessions found.")
		fmt.Println("\nA session is resumable when:")
		fmt.Println("  - It was interrupted (terminal closed, system restart)")
		fmt.Println("  - Its tmux session is no longer running")
		fmt.Println("  - A state file still exists")
		return nil
	}

	// Sort by start time, most recent first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION\tITERATION\tPROMISE\tSTARTED")

	for _, s := range sessions {
		promise := truncateString(s.CompletionPromise, 25)
		started := s.StartedAt.Format("Jan 02 15:04")

		iterStr := fmt.Sprintf("%d", s.Iteration)
		if s.MaxIterations > 0 {
			iterStr = fmt.Sprintf("%d/%d", s.Iteration, s.MaxIterations)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			s.ID,
			iterStr,
			promise,
			started,
		)
	}

	w.Flush()

	fmt.Printf("\n%d resumable session(s)\n", len(sessions))
	fmt.Println("\nResume with: forge resume <session-id>")

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
