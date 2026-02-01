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
		Short: "Claude session runner",
		Long: `Forge runs Claude sessions in tmux for autonomous execution.

It provides a minimal interface for starting, monitoring, and controlling
Claude sessions. For orchestration features (workers, leaders, boards),
use the foundry CLI.

Commands:
  forge start <prompt>   Start a Claude session
  forge attach [id]      Attach to a session's tmux
  forge status [id]      Show session status
  forge cancel [id]      Cancel a session
  forge list             List all sessions
  forge log [id]         View session output`,
		Version: Version,
	}

	// Session commands only
	rootCmd.AddCommand(
		newStartCmd(),
		newAttachCmd(),
		newStatusCmd(),
		newCancelCmd(),
		newListCmd(),
		newLogCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
