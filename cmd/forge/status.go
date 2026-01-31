package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/session"
)

func newStatusCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "status [session-id]",
		Short: "Show status of forge agent sessions",
		Long: `Show status of forge agent sessions.

If no session ID is provided, shows a summary of all sessions.
With a session ID, shows detailed status for that session.

Example:
  forge status
  forge status forge-api-12345678`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := session.NewManager()
			if err := mgr.Discover(); err != nil {
				return fmt.Errorf("discovering sessions: %w", err)
			}

			// Also check local directory
			wd, _ := os.Getwd()
			mgr.DiscoverLocal(wd)

			if len(args) > 0 {
				sessionID = args[0]
			}

			if sessionID != "" {
				return showDetailedStatus(mgr, sessionID)
			}

			return showStatusSummary(mgr)
		},
	}

	return cmd
}

func showStatusSummary(mgr *session.Manager) error {
	sessions := mgr.List()

	if len(sessions) == 0 {
		fmt.Println("No forge sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tSESSION\tITERATION\tPROMISE\tELAPSED")

	for _, s := range sessions {
		promise := "-"
		if s.State != nil {
			promise = s.State.CompletionPromise
			if len(promise) > 20 {
				promise = promise[:17] + "..."
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			s.StatusIcon(),
			s.ID,
			s.IterationString(),
			promise,
			s.ElapsedString(),
		)
	}

	w.Flush()

	// Summary
	active := mgr.ActiveCount()
	total := len(sessions)
	fmt.Printf("\n%d active / %d total sessions\n", active, total)

	return nil
}

func showDetailedStatus(mgr *session.Manager, id string) error {
	s := mgr.Get(id)
	if s == nil {
		return fmt.Errorf("session %q not found", id)
	}

	fmt.Printf("Session: %s\n", s.ID)
	fmt.Printf("Status:  %s %s\n", s.StatusIcon(), s.Status)
	fmt.Printf("Elapsed: %s\n", s.ElapsedString())

	if s.State != nil {
		fmt.Printf("\nIteration: %s\n", s.IterationString())
		fmt.Printf("Promise:   %s\n", s.State.CompletionPromise)
		fmt.Printf("WorkDir:   %s\n", s.State.WorkDir)
		fmt.Printf("LogFile:   %s\n", s.State.LogFile)
	}

	if s.Tmux != "" {
		fmt.Printf("\nTmux Session: %s\n", s.Tmux)
		fmt.Println("  Attach with: forge attach", s.ID)
	}

	// Show recent output
	if s.Status == session.StatusActive {
		output, err := mgr.CaptureOutput(s.ID, 10)
		if err == nil && len(output) > 0 {
			fmt.Println("\nRecent output:")
			for _, line := range output {
				if line != "" {
					fmt.Printf("  %s\n", line)
				}
			}
		}
	}

	return nil
}
