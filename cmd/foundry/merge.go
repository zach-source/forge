package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/leader"
)

func newMergeCmd() *cobra.Command {
	var (
		branch    string
		dryRun    bool
		noSkip    bool
		sessionID string
	)

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Single-threaded merge coordination",
		Long: `Launch the Merge Leader for coordinated merging to main.

The Merge Leader is responsible for:
- Coordinating all merges to prevent conflicts
- Ensuring work is reviewed before merging
- Handling merge conflicts systematically
- Updating tasks after merge (via foundry task move)

This is a single-threaded operation - only one merge leader
should be active per repository at a time.

Examples:
  foundry merge                      # Start merge leader for main
  foundry merge --branch develop     # Target different branch
  foundry merge --dry-run            # Show what would happen`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := leader.GetWorkDir()
			if err != nil {
				return err
			}

			cfg := leader.DefaultConfig(leader.RoleMerge)
			cfg.WorkDir = workDir
			cfg.SkipPerms = !noSkip
			cfg.SessionID = sessionID
			cfg.Branch = branch
			cfg.DryRun = dryRun

			prompt := leader.MergePrompt("beads", workDir, branch, dryRun)
			return leader.Run(context.Background(), cfg, prompt, leader.MergePromise())
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "main", "Target branch for merges")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would happen without executing")
	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")

	return cmd
}
