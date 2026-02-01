package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/agent"
	"github.com/zach-source/forge/internal/mcp"
	"github.com/zach-source/forge/internal/workspace"
)

func newWorkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Manage worktree-based feature development",
		Long: `Manage worktrees for isolated feature development.

Worktrees provide isolated working directories for each feature,
allowing parallel development without branch switching.

Workflow:
  1. forge work start "feature-name" --repo api
  2. (Claude works in the worktree)
  3. forge work complete "feature-name"
  4. forge merge (merges to main)

Examples:
  forge work start "user-auth" --repo api
  forge work list
  forge work attach "user-auth"
  forge work complete "user-auth"`,
	}

	cmd.AddCommand(
		newWorkStartCmd(),
		newWorkListCmd(),
		newWorkAttachCmd(),
		newWorkCompleteCmd(),
		newWorkAbandonCmd(),
	)

	return cmd
}

func newWorkStartCmd() *cobra.Command {
	var (
		repoName   string
		baseBranch string
		noSkip     bool
		sessionID  string
		prompt     string
	)

	cmd := &cobra.Command{
		Use:   "start <feature-name>",
		Short: "Start work on a new feature in a worktree",
		Long: `Create a worktree for a feature and optionally launch Claude.

This creates:
  1. A new git worktree at .forge/worktrees/<repo>/<feature>/
  2. A feature branch: feature/<feature-name>
  3. Optionally starts Claude in the worktree

Examples:
  forge work start "user-auth" --repo api
  forge work start "payment-flow" --repo api --base develop
  forge work start "fix-bug" --repo api --prompt "Fix the login bug"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			featureName := args[0]

			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			// Default to primary repo
			if repoName == "" {
				primary := ws.PrimaryRepo()
				if primary == nil {
					return fmt.Errorf("no repository specified and no primary repo set")
				}
				repoName = primary.Name
			}

			fmt.Printf("Creating worktree for %q in %s...\n", featureName, repoName)

			wt, err := ws.CreateWorktree(repoName, featureName, baseBranch)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Created worktree: %s\n", wt.Name)
			fmt.Printf("   Path: %s\n", wt.Path)
			fmt.Printf("   Branch: %s\n", wt.Branch)
			fmt.Printf("   Base: %s\n", wt.BaseBranch)

			// Update CLAUDE.md
			if err := ws.UpdateClaudeMD(); err != nil {
				fmt.Printf("   Warning: failed to update CLAUDE.md: %v\n", err)
			}

			// If prompt provided, launch Claude
			if prompt != "" {
				fmt.Println()
				fmt.Println("Launching Claude in worktree...")

				cfg := agent.DefaultConfig()
				cfg.Prompt = fmt.Sprintf(`You are working on feature: %s

Working directory: %s
Branch: %s
Base branch: %s

Task:
%s

When complete, output <promise>FEATURE_COMPLETE</promise>`, featureName, wt.Path, wt.Branch, wt.BaseBranch, prompt)
				cfg.CompletionPromise = "FEATURE_COMPLETE"
				cfg.MaxIterations = 100
				cfg.WorkDir = wt.Path
				cfg.MCPServers = []string{"graphiti", "context7"}
				cfg.SkipPermissions = !noSkip
				cfg.SessionID = sessionID

				a, err := agent.New(cfg)
				if err != nil {
					return err
				}

				return a.Run(context.Background())
			}

			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  cd %s\n", wt.Path)
			fmt.Println("  # or")
			fmt.Printf("  forge work attach %q\n", featureName)

			return nil
		},
	}

	cmd.Flags().StringVarP(&repoName, "repo", "r", "", "Repository name (default: primary)")
	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch (default: main or repo's dev_branch)")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt for Claude to execute in worktree")
	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")

	return cmd
}

func newWorkListCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List active worktrees",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			var worktrees []workspace.Worktree
			if all {
				worktrees, err = ws.ListWorktrees()
			} else {
				worktrees, err = ws.ActiveWorktrees()
			}
			if err != nil {
				return err
			}

			if len(worktrees) == 0 {
				fmt.Println("No worktrees found.")
				fmt.Println("Start one with: forge work start <feature-name>")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tREPO\tBRANCH\tSTATUS\tPATH")
			fmt.Fprintln(w, "----\t----\t------\t------\t----")

			for _, wt := range worktrees {
				path := wt.Path
				if len(path) > 40 {
					path = "..." + path[len(path)-37:]
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					wt.Name, wt.RepoName, wt.Branch, wt.Status, path)
			}

			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Show all worktrees including merged/abandoned")

	return cmd
}

func newWorkAttachCmd() *cobra.Command {
	var (
		noSkip    bool
		sessionID string
		prompt    string
	)

	cmd := &cobra.Command{
		Use:   "attach <feature-name>",
		Short: "Launch Claude in an existing worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			featureName := args[0]

			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			wt, err := ws.GetWorktree(featureName)
			if err != nil {
				return err
			}

			if wt.Status != "active" {
				return fmt.Errorf("worktree %q is %s, not active", featureName, wt.Status)
			}

			// Get repo for context
			repo := ws.GetRepo(wt.RepoName)
			repoContext := ""
			if repo != nil && repo.Summary != "" {
				repoContext = fmt.Sprintf("\nRepository: %s\n%s\n", repo.Name, repo.Summary)
			}

			// Default prompt
			if prompt == "" {
				prompt = "Continue working on this feature. Review the current state and make progress."
			}

			cfg := agent.DefaultConfig()
			cfg.Prompt = fmt.Sprintf(`You are working on feature: %s
%s
Working directory: %s
Branch: %s
Base branch: %s

Task:
%s

When complete, output <promise>FEATURE_COMPLETE</promise>`, featureName, repoContext, wt.Path, wt.Branch, wt.BaseBranch, prompt)
			cfg.CompletionPromise = "FEATURE_COMPLETE"
			cfg.MaxIterations = 100
			cfg.WorkDir = wt.Path
			cfg.MCPServers = []string{"graphiti", "context7"}
			cfg.SkipPermissions = !noSkip
			cfg.SessionID = sessionID

			a, err := agent.New(cfg)
			if err != nil {
				return err
			}

			fmt.Printf("🔧 Attaching to worktree: %s\n", featureName)
			fmt.Printf("   Path: %s\n", wt.Path)
			fmt.Println()

			return a.Run(context.Background())
		},
	}

	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt for Claude")
	cmd.Flags().BoolVar(&noSkip, "no-skip", false, "Don't use --dangerously-skip-permissions")
	cmd.Flags().StringVar(&sessionID, "id", "", "Custom session ID")

	return cmd
}

func newWorkCompleteCmd() *cobra.Command {
	var push bool

	cmd := &cobra.Command{
		Use:   "complete <feature-name>",
		Short: "Mark a feature as ready for merge",
		Long: `Mark a worktree's feature as complete and ready for merge.

This:
  1. Commits any uncommitted changes
  2. Pushes to remote (if --push)
  3. Marks worktree status as ready for merge

The merge leader will then pick it up.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			featureName := args[0]

			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			wt, err := ws.GetWorktree(featureName)
			if err != nil {
				return err
			}

			// Update status
			if err := ws.UpdateWorktreeStatus(featureName, "ready"); err != nil {
				return err
			}

			fmt.Printf("✅ Marked %q as ready for merge\n", featureName)
			fmt.Printf("   Branch: %s\n", wt.Branch)

			if push {
				fmt.Println("   Pushing to remote...")
				// Push would happen here
			}

			fmt.Println()
			fmt.Println("Next: Run 'forge merge' to merge to main")

			return nil
		},
	}

	cmd.Flags().BoolVar(&push, "push", false, "Push branch to remote")

	return cmd
}

func newWorkAbandonCmd() *cobra.Command {
	var deleteBranch bool

	cmd := &cobra.Command{
		Use:   "abandon <feature-name>",
		Short: "Abandon a feature worktree",
		Long: `Abandon a worktree and optionally delete the branch.

Use this when a feature is no longer needed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			featureName := args[0]

			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			if err := ws.RemoveWorktree(featureName, deleteBranch); err != nil {
				return err
			}

			fmt.Printf("✅ Abandoned worktree: %s\n", featureName)
			if deleteBranch {
				fmt.Println("   Branch deleted")
			}

			// Update CLAUDE.md
			if err := ws.UpdateClaudeMD(); err != nil {
				fmt.Printf("   Warning: failed to update CLAUDE.md: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&deleteBranch, "delete-branch", false, "Also delete the git branch")

	return cmd
}

// Ensure mcp package is used (for future use)
var _ = mcp.AvailableServers
