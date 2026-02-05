package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/leader"
)

func newCICDCmd() *cobra.Command {
	var (
		noSkip    bool
		sessionID string
	)

	cmd := &cobra.Command{
		Use:   "cicd",
		Short: "CI/CD health monitoring and fix creation",
		Long: `Launch the CI/CD leader for pipeline monitoring and issue creation.

The CI/CD leader is responsible for:
- Monitoring GitHub Actions workflow health
- Diagnosing CI failures and their root causes
- Creating tasks for CI fixes via foundry task
- Reporting overall CI/CD health status
- Recommending CI improvements

Examples:
  foundry cicd                     # Start CI/CD monitoring
  foundry cicd --id my-session     # Use custom session ID`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := leader.GetWorkDir()
			if err != nil {
				return err
			}

			cfg := leader.DefaultConfig(leader.RoleCICD)
			cfg.WorkDir = workDir
			cfg.SkipPerms = !noSkip
			cfg.SessionID = sessionID

			prompt := leader.CICDPrompt(workDir)
			return leader.Run(context.Background(), cfg, prompt, leader.CICDPromise())
		},
	}

	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")

	return cmd
}
