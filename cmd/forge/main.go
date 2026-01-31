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
		Use:   "forge",
		Short: "Autonomous Claude execution agent",
		Long: `Forge orchestrates autonomous Claude sessions for the Ralph Loop.

It spawns Claude in a tmux session for attachable, persistent execution,
runs with --dangerously-skip-permissions for autonomous operation,
and iterates until the completion promise is met or max iterations reached.`,
		Version: Version,
	}

	// Add subcommands
	rootCmd.AddCommand(
		// Core commands
		newStartCmd(),
		newAttachCmd(),
		newStatusCmd(),
		newCancelCmd(),
		newListCmd(),
		newLogCmd(),
		newMonitorCmd(),
		// Workspace commands
		newInitCmd(),
		newRepoCmd(),
		newWorkCmd(),
		newBoardCmd(),
		// Worker commands
		newWorkerCmd(),
		// Leader commands
		newPlannerCmd(),
		newReviewerCmd(),
		newMergeCmd(),
		newDeployCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
