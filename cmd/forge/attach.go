package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/session"
)

func newAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach [session-id]",
		Short: "Attach to a running forge agent's tmux session",
		Long: `Attach to a running forge agent's tmux session.

If no session ID is provided and only one session is running, it attaches to that.
Otherwise, it shows available sessions.

Example:
  forge attach
  forge attach forge-api-12345678`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := session.NewManager()
			if err := mgr.Discover(); err != nil {
				return fmt.Errorf("discovering sessions: %w", err)
			}

			var sessionName string

			if len(args) > 0 {
				sessionName = args[0]
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
					sessionName = active[0].Tmux
					fmt.Printf("Attaching to %s...\n", sessionName)
				} else {
					fmt.Println("Multiple active sessions. Specify which to attach:")
					for _, s := range active {
						fmt.Printf("  %s - %s (%s)\n", s.ID, s.State.CompletionPromise, s.IterationString())
					}
					return nil
				}
			}

			// Attach using exec to replace this process
			tmuxPath, err := exec.LookPath("tmux")
			if err != nil {
				return fmt.Errorf("tmux not found: %w", err)
			}

			return syscallExec(tmuxPath, []string{"tmux", "attach", "-t", sessionName}, os.Environ())
		},
	}

	return cmd
}

// syscallExec replaces the current process with tmux
func syscallExec(path string, args []string, env []string) error {
	// Use os/exec for portability (syscall.Exec not available on all platforms)
	cmd := exec.Command(path, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Run()
}
