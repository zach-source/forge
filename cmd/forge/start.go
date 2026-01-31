package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/agent"
	"github.com/zach-source/forge/internal/mcp"
)

func newStartCmd() *cobra.Command {
	var (
		promise       string
		maxIterations int
		mcpServers    string
		mcpConfigPath string
		workDir       string
		noSkip        bool
		sessionID     string
	)

	cmd := &cobra.Command{
		Use:   "start <prompt>",
		Short: "Start an autonomous Claude agent",
		Long: `Start an autonomous Claude agent that runs in a tmux session.

The agent will iterate until the completion promise is found in Claude's output
or the maximum number of iterations is reached.

Example:
  forge start "Build a REST API for todos" --promise "COMPLETE" --max 50
  forge start "Research topic Y" --promise "DONE" --mcp graphiti,context7`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]

			// Determine working directory
			if workDir == "" {
				var err error
				workDir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
			}

			// Parse MCP servers
			servers := mcp.ParseServerList(mcpServers)
			if len(servers) == 0 {
				servers = []string{"graphiti", "context7"}
			}

			// Build config
			cfg := agent.DefaultConfig()
			cfg.Prompt = prompt
			cfg.CompletionPromise = promise
			cfg.MaxIterations = maxIterations
			cfg.WorkDir = workDir
			cfg.MCPServers = servers
			cfg.MCPConfigPath = mcpConfigPath
			cfg.SkipPermissions = !noSkip
			cfg.SessionID = sessionID

			// Create and run agent
			a, err := agent.New(cfg)
			if err != nil {
				return err
			}

			err = a.Run(context.Background())
			if err != nil {
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

	cmd.Flags().StringVarP(&promise, "promise", "p", "", "Completion promise text (required)")
	cmd.Flags().IntVarP(&maxIterations, "max", "m", 50, "Max iterations (0 = unlimited)")
	cmd.Flags().StringVar(&mcpServers, "mcp", "graphiti,context7", "Comma-separated MCP servers")
	cmd.Flags().StringVar(&mcpConfigPath, "mcp-config", "", "Path to custom MCP config file")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "Working directory (default: current)")
	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")

	cmd.MarkFlagRequired("promise")

	return cmd
}
