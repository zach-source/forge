package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/agent"
	"github.com/zach-source/forge/internal/board"
)

func newBoardCmd() *cobra.Command {
	var (
		sync      bool
		watch     bool
		config    bool
		show      bool
		noSkip    bool
		sessionID string
		useGitHub bool
	)

	cmd := &cobra.Command{
		Use:   "board",
		Short: "Manage beads from Notion or GitHub Projects",
		Long: `Launches Claude with board MCP to sync tasks from Notion or GitHub Projects.

The board command connects to a configured board and syncs items
to the beads task system. It can create beads from board items, update
statuses bidirectionally, and maintain hierarchy.

## Notion Setup:
  1. Create a Notion integration at https://www.notion.so/my-integrations
  2. Share your database with the integration
  3. Set NOTION_API_TOKEN=secret_xxx in your environment
  4. Run 'forge board --config' to set your database ID

## GitHub Projects Setup:
  1. Create a GitHub Personal Access Token with 'project' scope
  2. Set GITHUB_TOKEN=ghp_xxx in your environment
  3. Run 'forge board --github --config' to set your project

Examples:
  forge board                    # Interactive session with Notion
  forge board --github           # Interactive session with GitHub Projects
  forge board --sync             # One-shot sync, then exit
  forge board --github --sync    # One-shot GitHub sync
  forge board --watch            # Continuous sync loop
  forge board --config           # Configure Notion database
  forge board --github --config  # Configure GitHub project
  forge board --show             # Show current configuration`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Select provider based on flags
			var provider board.Provider
			if useGitHub {
				provider = board.GetProvider(board.ProviderGitHub)
			} else {
				provider = board.GetProvider(board.ProviderNotion)
			}

			// Handle --config: interactive setup
			if config {
				return provider.Configure()
			}

			// Handle --show: display current config
			if show {
				return provider.ShowConfig()
			}

			// Validate environment
			if !provider.HasToken() {
				return fmt.Errorf("%s not set; export it or add to your shell profile", provider.TokenEnvVar())
			}

			// Load configuration
			if err := provider.Load(); err != nil {
				return err
			}

			// Get working directory
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			// Build prompt based on mode
			var prompt string
			var promise string
			var maxIterations int

			if watch {
				prompt = provider.WatchPrompt(30)
				promise = "STOPPED" // Watch mode doesn't auto-complete
				maxIterations = 0   // Unlimited
			} else if sync {
				prompt = provider.SyncPrompt(true)
				promise = provider.CompletionPromise()
				maxIterations = 10 // One-shot should complete quickly
			} else {
				// Interactive mode
				prompt = provider.SyncPrompt(false)
				promise = "EXIT" // User explicitly exits
				maxIterations = 0
			}

			// Build agent config
			agentCfg := agent.DefaultConfig()
			agentCfg.Prompt = prompt
			agentCfg.CompletionPromise = promise
			agentCfg.MaxIterations = maxIterations
			agentCfg.WorkDir = workDir
			agentCfg.MCPServers = provider.MCPServers()
			agentCfg.SkipPermissions = !noSkip
			agentCfg.SessionID = sessionID

			// Create and run agent
			a, err := agent.New(agentCfg)
			if err != nil {
				return err
			}

			fmt.Printf("%s Starting %s sync session...\n", provider.Icon(), provider.DisplayName())
			fmt.Printf("   Board: %s\n", provider.BoardIdentifier())
			if sync {
				fmt.Println("   Mode: one-shot sync")
			} else if watch {
				fmt.Println("   Mode: continuous watch")
			} else {
				fmt.Println("   Mode: interactive")
			}
			fmt.Println()

			return a.Run(context.Background())
		},
	}

	cmd.Flags().BoolVar(&sync, "sync", false, "One-shot sync, then exit")
	cmd.Flags().BoolVar(&watch, "watch", false, "Continuous sync loop")
	cmd.Flags().BoolVar(&config, "config", false, "Configure board (Notion or GitHub)")
	cmd.Flags().BoolVar(&show, "show", false, "Show current configuration")
	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")
	cmd.Flags().BoolVar(&useGitHub, "github", false, "Use GitHub Projects instead of Notion")

	return cmd
}
