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
- Task management with git integration (beads backend)
- Parallel workers with persistent identity
- External board sync (Notion, GitHub Projects)
- Workspace and repository management
- Worktree-based feature development
- Leader agents (planner, reviewer, merge, deploy)

Foundry uses forge for running Claude sessions.

Examples:
  foundry task                   # View all tasks by status
  foundry task add "Fix bug"     # Create new task
  foundry worker create          # Create a new worker
  foundry board --sync           # Sync with Notion
  foundry init                   # Initialize workspace
  foundry work start "feature"   # Start feature worktree`,
		Version: Version,
	}

	// Add subcommands
	rootCmd.AddCommand(
		// Local tools
		newTaskCmd(),
		newLogsCmd(),
		newHealthCmd(),
		newSummaryCmd(),
		// Orchestration
		newWorkerCmd(),
		newBoardCmd(),
		newMonitorCmd(),
		newSupervisorCmd(),
		newShutdownCmd(),
		// Agent teams
		newTeamCmd(),
		// Leaders (launch forge sessions)
		newPlannerCmd(),
		newReviewerCmd(),
		newMergeCmd(),
		newDeployCmd(),
		newCICDCmd(),
		newObserveCmd(),
		// Workspace management
		newInitCmd(),
		newRepoCmd(),
		newWorkCmd(),
		newPromptsCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
