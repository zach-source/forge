package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/leader"
)

func newPromptsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "prompts",
		Aliases: []string{"prompt"},
		Short:   "Manage leader prompts",
		Long: `Manage leader prompts for the workspace.

Prompts are markdown files in .forge/prompts/ that define how each leader
agent behaves. You can customize them per-workspace to match your project's
conventions and requirements.

Examples:
  foundry prompts init              # Initialize default prompts
  foundry prompts list              # List available prompts
  foundry prompts show reviewer     # Show a prompt
  foundry prompts edit reviewer     # Edit a prompt in $EDITOR
  foundry prompts reset reviewer    # Reset to default`,
	}

	cmd.AddCommand(newPromptsInitCmd())
	cmd.AddCommand(newPromptsListCmd())
	cmd.AddCommand(newPromptsShowCmd())
	cmd.AddCommand(newPromptsEditCmd())
	cmd.AddCommand(newPromptsResetCmd())

	return cmd
}

func newPromptsInitCmd() *cobra.Command {
	var (
		force   bool
		context string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize default leader prompts",
		Long: `Initialize default leader prompts in .forge/prompts/.

By default, only creates prompts that don't already exist.
Use --force to overwrite existing prompts.

The --context flag lets you provide project-specific context that
will be inserted into the prompts (replaces {{.ProjectContext}}).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			loader := leader.NewPromptLoader(workDir)

			var initialized []string
			if context != "" {
				initialized, err = loader.InitDefaultsWithContext(context, force)
			} else {
				initialized, err = loader.InitDefaults(force)
			}
			if err != nil {
				return err
			}

			if len(initialized) == 0 {
				fmt.Println("All prompts already exist. Use --force to overwrite.")
				return nil
			}

			fmt.Printf("✅ Initialized %d prompt(s):\n", len(initialized))
			for _, role := range initialized {
				fmt.Printf("   .forge/prompts/%s.md\n", role)
			}
			fmt.Println()
			fmt.Println("Edit these files to customize leader behavior for your project.")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing prompts")
	cmd.Flags().StringVarP(&context, "context", "c", "", "Project context to insert into prompts")

	return cmd
}

func newPromptsListCmd() *cobra.Command {
	var showDefaults bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available leader prompts",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			loader := leader.NewPromptLoader(workDir)
			customPrompts, err := loader.ListCustomPrompts()
			if err != nil {
				return err
			}

			// Create map for quick lookup
			customMap := make(map[string]bool)
			for _, role := range customPrompts {
				customMap[role] = true
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ROLE\tSTATUS\tPATH")

			allRoles := leader.AllRoles()
			for _, role := range allRoles {
				var status, path string
				if customMap[role] {
					status = "custom"
					path = loader.PromptPath(role)
				} else if showDefaults {
					status = "default"
					path = "(embedded)"
				} else {
					status = "not initialized"
					path = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", role, status, path)
			}

			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showDefaults, "defaults", "d", false, "Show default prompt info")

	return cmd
}

func newPromptsShowCmd() *cobra.Command {
	var showDefault bool

	cmd := &cobra.Command{
		Use:   "show <role>",
		Short: "Show a leader prompt",
		Long: `Show the content of a leader prompt.

By default shows the custom prompt if it exists.
Use --default to show the embedded default prompt instead.`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return leader.AllRoles(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			role := args[0]

			// Validate role
			validRole := false
			for _, r := range leader.AllRoles() {
				if r == role {
					validRole = true
					break
				}
			}
			if !validRole {
				return fmt.Errorf("unknown role: %s (valid: %s)", role, strings.Join(leader.AllRoles(), ", "))
			}

			var content string
			var err error

			if showDefault {
				content, err = leader.GetDefaultPrompt(role)
				if err != nil {
					return err
				}
				fmt.Printf("# Default prompt for: %s\n\n", role)
			} else {
				workDir, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}

				loader := leader.NewPromptLoader(workDir)
				content, err = loader.Load(role)
				if err != nil {
					return err
				}

				if content == "" {
					// Fall back to default
					content, err = leader.GetDefaultPrompt(role)
					if err != nil {
						return err
					}
					fmt.Printf("# Default prompt for: %s (no custom prompt)\n\n", role)
				} else {
					fmt.Printf("# Custom prompt for: %s\n# Path: %s\n\n", role, loader.PromptPath(role))
				}
			}

			fmt.Print(content)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showDefault, "default", "d", false, "Show default prompt instead of custom")

	return cmd
}

func newPromptsEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <role>",
		Short: "Edit a leader prompt in $EDITOR",
		Long: `Edit a leader prompt in your configured editor.

If the prompt doesn't exist, it will be created from the default first.
Uses $EDITOR environment variable (defaults to vim).`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return leader.AllRoles(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			role := args[0]

			// Validate role
			validRole := false
			for _, r := range leader.AllRoles() {
				if r == role {
					validRole = true
					break
				}
			}
			if !validRole {
				return fmt.Errorf("unknown role: %s (valid: %s)", role, strings.Join(leader.AllRoles(), ", "))
			}

			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			loader := leader.NewPromptLoader(workDir)

			// Create from default if doesn't exist
			if !loader.Exists(role) {
				content, err := leader.GetDefaultPrompt(role)
				if err != nil {
					return err
				}
				if err := loader.Save(role, content); err != nil {
					return err
				}
				fmt.Printf("Created %s from default\n", loader.PromptPath(role))
			}

			// Get editor
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}

			// Open in editor
			promptPath := loader.PromptPath(role)
			editorCmd := exec.Command(editor, promptPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			return editorCmd.Run()
		},
	}

	return cmd
}

func newPromptsResetCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "reset [role]",
		Short: "Reset a prompt to default",
		Long: `Reset a leader prompt to its default content.

Use --all to reset all prompts.`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return leader.AllRoles(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			loader := leader.NewPromptLoader(workDir)

			var roles []string
			if all {
				roles = leader.AllRoles()
			} else if len(args) > 0 {
				role := args[0]
				// Validate role
				validRole := false
				for _, r := range leader.AllRoles() {
					if r == role {
						validRole = true
						break
					}
				}
				if !validRole {
					return fmt.Errorf("unknown role: %s (valid: %s)", role, strings.Join(leader.AllRoles(), ", "))
				}
				roles = []string{role}
			} else {
				return fmt.Errorf("specify a role or use --all")
			}

			for _, role := range roles {
				content, err := leader.GetDefaultPrompt(role)
				if err != nil {
					return err
				}
				if err := loader.Save(role, content); err != nil {
					return err
				}
				fmt.Printf("Reset: .forge/prompts/%s.md\n", role)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Reset all prompts")

	return cmd
}

// getPromptsDir returns the prompts directory path for the current workspace.
func getPromptsDir() string {
	workDir, _ := os.Getwd()
	return filepath.Join(workDir, ".forge", "prompts")
}
