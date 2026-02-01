package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/workspace"
)

func newInitCmd() *cobra.Command {
	var (
		name        string
		description string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize a new forge workspace",
		Long: `Initialize a new forge workspace directory.

A forge workspace is a directory that contains:
- CLAUDE.md                - Claude instructions (auto-generated)
- .forge/workspace.yaml    - Workspace configuration
- .forge/repos/            - Cloned repositories
- .forge/sessions/         - Session state files
- .forge/logs/             - Log files

The workspace ensures there are no conflicting parent or child workspaces.
You can then add repositories with 'forge repo add'.

Examples:
  forge init                           # Initialize in current directory
  forge init my-project                # Create and initialize new directory
  forge init . --name "My Project"     # Initialize with custom name
  forge init ~/workspaces/api --description "API services"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine target directory
			var dir string
			if len(args) > 0 {
				dir = args[0]
			} else {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("getting current directory: %w", err)
				}
			}

			// Resolve to absolute path
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			// Check for workspace conflicts
			conflicts, err := workspace.CheckConflicts(absDir)
			if err != nil {
				return fmt.Errorf("checking conflicts: %w", err)
			}

			if conflicts.HasParentWorkspace && !force {
				fmt.Printf("⚠️  Found parent workspace at: %s\n", conflicts.ParentWorkspace)
				fmt.Println("   Creating a nested workspace may cause confusion.")
				fmt.Println()
				if !confirmPrompt("Continue anyway?") {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			if conflicts.HasChildWorkspace && !force {
				fmt.Printf("⚠️  Found %d child workspace(s):\n", len(conflicts.ChildWorkspaces))
				for _, child := range conflicts.ChildWorkspaces {
					fmt.Printf("   - %s\n", child)
				}
				fmt.Println()
				fmt.Println("   Creating a parent workspace may cause confusion.")
				if !confirmPrompt("Continue anyway?") {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			// Default name to directory name
			if name == "" {
				name = filepath.Base(absDir)
			}

			// Initialize workspace
			ws, err := workspace.Init(absDir, name, description)
			if err != nil {
				return err
			}

			// Generate CLAUDE.md
			if err := ws.WriteClaudeMD(); err != nil {
				fmt.Printf("Warning: failed to create CLAUDE.md: %v\n", err)
			}

			fmt.Printf("✅ Initialized forge workspace: %s\n", ws.Name)
			fmt.Printf("   Location: %s\n", absDir)
			fmt.Println()
			fmt.Println("Created files:")
			fmt.Println("   CLAUDE.md              - Claude instructions")
			fmt.Println("   .forge/workspace.yaml  - Workspace config")
			fmt.Println("   .forge/repos/          - Repository directory")
			fmt.Println("   .forge/sessions/       - Session state")
			fmt.Println("   .forge/logs/           - Log files")
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  1. Add repositories:")
			fmt.Println("     forge repo add <git-url>           # Clone a repo")
			fmt.Println("     forge repo link <path>             # Link existing repo")
			fmt.Println()
			fmt.Println("  2. Configure Notion (optional):")
			fmt.Println("     forge board --config")
			fmt.Println()
			fmt.Println("  3. Start working:")
			fmt.Println("     forge planner                      # Plan work")
			fmt.Println("     forge start \"<task>\" -p DONE       # Run autonomous task")

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Workspace name (default: directory name)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Workspace description")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Initialize even with conflicts")

	return cmd
}

// confirmPrompt asks the user for yes/no confirmation.
func confirmPrompt(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", question)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
