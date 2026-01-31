package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/tmux"
)

func newCancelCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "cancel [session-id]",
		Short: "Cancel a running forge agent",
		Long: `Cancel a running forge agent session.

This removes the state file and kills the tmux session.
If no session ID is provided and only one session is running, it cancels that.

Example:
  forge cancel
  forge cancel forge-api-12345678
  forge cancel --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := session.NewManager()
			if err := mgr.Discover(); err != nil {
				return fmt.Errorf("discovering sessions: %w", err)
			}

			// Also check local directory
			wd, _ := os.Getwd()
			mgr.DiscoverLocal(wd)

			var sessionID string

			if len(args) > 0 {
				sessionID = args[0]
			} else {
				// Find active sessions
				sessions := mgr.List()
				active := make([]*session.Session, 0)
				for _, s := range sessions {
					if s.Status == session.StatusActive {
						active = append(active, s)
					}
				}

				if len(active) == 0 {
					fmt.Println("No active forge sessions found.")
					return nil
				}

				if len(active) == 1 {
					sessionID = active[0].ID
				} else {
					fmt.Println("Multiple active sessions. Specify which to cancel:")
					for _, s := range active {
						fmt.Printf("  %s - %s (%s)\n", s.ID, s.State.CompletionPromise, s.IterationString())
					}
					return nil
				}
			}

			return cancelSession(sessionID, force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force kill without confirmation")

	return cmd
}

func cancelSession(id string, force bool) error {
	if !force {
		fmt.Printf("Cancel session %s? [y/N] ", id)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Delete state file
	statePath := ralph.SessionStatePath(id)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to remove state file: %v\n", err)
	}

	// Kill tmux session
	tmuxSession := tmux.NewSession(id, "", "")
	if tmuxSession.Exists() {
		if err := tmuxSession.Kill(); err != nil {
			return fmt.Errorf("killing tmux session: %w", err)
		}
	}

	fmt.Printf("❌ Session %s cancelled\n", id)
	return nil
}
