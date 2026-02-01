package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/leader"
	"github.com/zach-source/forge/internal/notion"
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
- Creating Notion issues for problems found
- Running automated checks (tests, linters)
- Approving or blocking merges

Examples:
  forge reviewer                   # Review current branch vs main
  forge reviewer feature/auth      # Review specific branch
  forge reviewer --branch develop  # Use different base branch`,
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

			notionCfg, err := notion.Load()
			if err != nil {
				return err
			}

			prompt := leader.ReviewerPrompt(notionCfg.DatabaseID, workDir, branch)
			return leader.Run(context.Background(), cfg, prompt, leader.ReviewerPromise())
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "main", "Base branch to compare against")
	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")

	return cmd
}
