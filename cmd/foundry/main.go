package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time
var Version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "foundry",
		Short: "Development orchestration platform",
		Long: `Foundry orchestrates development workflows on top of forge.

It provides tools for:
- Local kanban issue tracking (SQLite)
- Parallel workers with persistent identity
- External board sync (Notion, GitHub Projects)
- Workspace and repository management
- Worktree-based feature development
- Leader agents (planner, reviewer, merge, deploy)

Foundry uses forge for running Claude sessions.

Examples:
  foundry kanban                 # View local kanban board
  foundry worker create          # Create a new worker
  foundry board --sync           # Sync with Notion
  foundry init                   # Initialize workspace
  foundry work start "feature"   # Start feature worktree`,
		Version: Version,
	}

	// Add subcommands
	rootCmd.AddCommand(
		// Local tools
		newKanbanCmd(),
		// Orchestration
		newWorkerCmd(),
		newBoardCmd(),
		newMonitorCmd(),
		// Leaders (launch forge sessions)
		newPlannerCmd(),
		newReviewerCmd(),
		newMergeCmd(),
		newDeployCmd(),
		// Workspace management
		newInitCmd(),
		newRepoCmd(),
		newWorkCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
