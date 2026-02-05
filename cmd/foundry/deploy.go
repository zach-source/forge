package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/leader"
)

func newDeployCmd() *cobra.Command {
	var (
		environment string
		dryRun      bool
		noSkip      bool
		sessionID   string
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deployment coordination and smoke testing",
		Long: `Launch the Deployment Leader for coordinated deployments.

The Deployment Leader is responsible for:
- Deploying latest features to target environment
- Running smoke tests to verify deployment
- Creating tasks for failures and fixes (via foundry task add)
- Managing rollbacks if needed

This is a single-threaded operation - only one deployment
should be in progress per environment at a time.

Examples:
  foundry deploy                     # Deploy to staging
  foundry deploy --env production    # Deploy to production
  foundry deploy --dry-run           # Show what would be deployed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := leader.GetWorkDir()
			if err != nil {
				return err
			}

			cfg := leader.DefaultConfig(leader.RoleDeployment)
			cfg.WorkDir = workDir
			cfg.SkipPerms = !noSkip
			cfg.SessionID = sessionID
			cfg.Environment = environment
			cfg.DryRun = dryRun

			prompt := leader.DeployPrompt("beads", workDir, environment, dryRun)
			return leader.Run(context.Background(), cfg, prompt, leader.DeployPromise())
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "staging", "Target environment (staging, production)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deployed without executing")
	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")

	return cmd
}
