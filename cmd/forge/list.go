package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/session"
)

func newListCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all forge tmux sessions",
		Long: `List all forge agent sessions.

By default, only shows active sessions. Use --all to include completed sessions.

Example:
  forge list
  forge list --all`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := session.NewManager()
			if err := mgr.Discover(); err != nil {
				return fmt.Errorf("discovering sessions: %w", err)
			}

			// Also check local directory
			wd, _ := os.Getwd()
			mgr.DiscoverLocal(wd)

			sessions := mgr.List()

			if !all {
				// Filter to active only
				active := make([]*session.Session, 0)
				for _, s := range sessions {
					if s.Status == session.StatusActive {
						active = append(active, s)
					}
				}
				sessions = active
			}

			if len(sessions) == 0 {
				if all {
					fmt.Println("No forge sessions found.")
				} else {
					fmt.Println("No active forge sessions. Use --all to see completed sessions.")
				}
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SESSION\tSTATUS\tITER\tPROMISE\tWORKDIR")

			for _, s := range sessions {
				promise := "-"
				workDir := "-"
				if s.State != nil {
					promise = s.State.CompletionPromise
					if len(promise) > 15 {
						promise = promise[:12] + "..."
					}
					workDir = s.State.WorkDir
					if len(workDir) > 30 {
						workDir = "..." + workDir[len(workDir)-27:]
					}
				}

				fmt.Fprintf(w, "%s\t%s %s\t%s\t%s\t%s\n",
					s.ID,
					s.StatusIcon(),
					s.Status,
					s.IterationString(),
					promise,
					workDir,
				)
			}

			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Show all sessions including completed")

	return cmd
}
