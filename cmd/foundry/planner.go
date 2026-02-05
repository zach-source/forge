package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/leader"
)

func newPlannerCmd() *cobra.Command {
	var (
		noSkip    bool
		sessionID string
	)

	cmd := &cobra.Command{
		Use:   "planner",
		Short: "Strategic planning and roadmap management",
		Long: `Launch the Planner leader for high-level project planning.

The Planner is responsible for:
- Breaking down goals into epics, features, and tasks
- Creating and managing tasks (via foundry task add)
- Maintaining project roadmap and priorities

Examples:
  foundry planner                    # Start planning session
  foundry planner --id my-session    # Use custom session ID`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := leader.GetWorkDir()
			if err != nil {
				return err
			}

			cfg := leader.DefaultConfig(leader.RolePlanner)
			cfg.WorkDir = workDir
			cfg.SkipPerms = !noSkip
			cfg.SessionID = sessionID

			prompt := leader.PlannerPrompt("beads", workDir)
			return leader.Run(context.Background(), cfg, prompt, leader.PlannerPromise())
		},
	}

	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")

	return cmd
}
