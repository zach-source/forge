package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/leader"
)

func newReviewerCmd() *cobra.Command {
	var (
		branch    string
		noSkip    bool
		sessionID string
	)

	cmd := &cobra.Command{
		Use:   "reviewer [branch]",
		Short: "Code review and issue creation",
		Long: `Launch the Reviewer leader for code review and quality gates.

The Reviewer is responsible for:
- Reviewing code changes for quality and correctness
- Creating tasks for problems found (via foundry task add)
- Running automated checks (tests, linters)
- Approving or blocking merges

Examples:
  foundry reviewer                   # Review current branch vs main
  foundry reviewer feature/auth      # Review specific branch
  foundry reviewer --branch develop  # Use different base branch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := leader.GetWorkDir()
			if err != nil {
				return err
			}

			cfg := leader.DefaultConfig(leader.RoleReviewer)
			cfg.WorkDir = workDir
			cfg.SkipPerms = !noSkip
			cfg.SessionID = sessionID
			cfg.Branch = branch

			prompt := leader.ReviewerPrompt("beads", workDir, branch)
			return leader.Run(context.Background(), cfg, prompt, leader.ReviewerPromise())
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "main", "Base branch to compare against")
	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")

	return cmd
}
