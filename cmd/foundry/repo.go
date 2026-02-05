package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/workspace"
)

func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage repositories in the workspace",
		Long: `Manage repositories in the forge workspace.

Repositories can be added by cloning from a remote URL or by linking
an existing local repository.

Examples:
  forge repo add https://github.com/user/repo.git
  forge repo link ../my-existing-repo
  forge repo list
  forge repo remove my-repo`,
	}

	cmd.AddCommand(
		newRepoAddCmd(),
		newRepoLinkCmd(),
		newRepoListCmd(),
		newRepoRemoveCmd(),
		newRepoPrimaryCmd(),
		newRepoFetchCmd(),
		newRepoSyncCmd(),
		newRepoDescribeCmd(),
	)

	return cmd
}

func newRepoAddCmd() *cobra.Command {
	var (
		name   string
		branch string
	)

	cmd := &cobra.Command{
		Use:   "add <git-url>",
		Short: "Clone and add a repository to the workspace",
		Long: `Clone a repository from a remote URL and add it to the workspace.

The repository will be cloned to .forge/repos/<name>/

Examples:
  forge repo add https://github.com/user/repo.git
  forge repo add git@github.com:user/repo.git --name my-repo
  forge repo add https://github.com/user/repo.git --branch develop`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			remote := args[0]

			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			fmt.Printf("Cloning %s...\n", remote)
			repo, err := ws.CloneRepo(remote, name, branch)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Added repository: %s\n", repo.Name)
			fmt.Printf("   Path: %s\n", repo.Path)
			if repo.Branch != "" {
				fmt.Printf("   Branch: %s\n", repo.Branch)
			}

			// Update CLAUDE.md
			if err := ws.UpdateClaudeMD(); err != nil {
				fmt.Printf("   Warning: failed to update CLAUDE.md: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Repository name (default: extracted from URL)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch to clone (default: default branch)")

	return cmd
}

func newRepoLinkCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "link <path>",
		Short: "Link an existing repository to the workspace",
		Long: `Link an existing local repository to the workspace.

Unlike 'add', this does not clone the repository - it just adds
a reference to an existing git repository on disk.

Examples:
  forge repo link ../my-repo
  forge repo link /path/to/repo --name custom-name
  forge repo link .                    # Link current directory`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			repo, err := ws.LinkRepo(path, name)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Linked repository: %s\n", repo.Name)
			fmt.Printf("   Path: %s\n", repo.Path)
			if repo.Remote != "" {
				fmt.Printf("   Remote: %s\n", repo.Remote)
			}
			if repo.Branch != "" {
				fmt.Printf("   Branch: %s\n", repo.Branch)
			}

			// Update CLAUDE.md
			if err := ws.UpdateClaudeMD(); err != nil {
				fmt.Printf("   Warning: failed to update CLAUDE.md: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Repository name (default: directory name)")

	return cmd
}

func newRepoListCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List repositories in the workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			if len(ws.Repos) == 0 {
				fmt.Println("No repositories in workspace.")
				fmt.Println("Add one with: forge repo add <git-url>")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

			if verbose {
				_, _ = fmt.Fprintln(w, "NAME\tTYPE\tBRANCH\tPATH\tREMOTE")
				_, _ = fmt.Fprintln(w, "----\t----\t------\t----\t------")
			} else {
				_, _ = fmt.Fprintln(w, "NAME\tTYPE\tBRANCH\tPATH")
				_, _ = fmt.Fprintln(w, "----\t----\t------\t----")
			}

			for _, repo := range ws.Repos {
				repoType := "cloned"
				if repo.IsLinked {
					repoType = "linked"
				}

				name := repo.Name
				if repo.IsPrimary {
					name = name + " *"
				}

				if verbose {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						name, repoType, repo.Branch, repo.Path, repo.Remote)
				} else {
					// Truncate path for display
					path := repo.Path
					if len(path) > 40 {
						path = "..." + path[len(path)-37:]
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						name, repoType, repo.Branch, path)
				}
			}

			_ = w.Flush()

			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show full details")

	return cmd
}

func newRepoRemoveCmd() *cobra.Command {
	var deleteFiles bool

	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a repository from the workspace",
		Long: `Remove a repository from the workspace.

By default, this only removes the reference from the workspace config.
The actual files are not deleted unless --delete is specified.

For linked repositories, --delete has no effect (won't delete external repos).

Examples:
  forge repo remove my-repo
  forge repo remove my-repo --delete   # Also delete cloned files`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			repo := ws.GetRepo(name)
			if repo == nil {
				return fmt.Errorf("repository %q not found", name)
			}

			// Check if we should delete files
			if deleteFiles && !repo.IsLinked {
				// Only delete cloned repos, not linked ones
				if strings.HasPrefix(repo.Path, ws.ReposDir()) {
					fmt.Printf("Deleting files at %s...\n", repo.Path)
					if err := os.RemoveAll(repo.Path); err != nil {
						fmt.Printf("Warning: failed to delete files: %v\n", err)
					}
				}
			}

			if err := ws.RemoveRepo(name); err != nil {
				return err
			}

			fmt.Printf("✅ Removed repository: %s\n", name)

			// Update CLAUDE.md
			if err := ws.UpdateClaudeMD(); err != nil {
				fmt.Printf("   Warning: failed to update CLAUDE.md: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&deleteFiles, "delete", false, "Also delete cloned files")

	return cmd
}

func newRepoPrimaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "primary <name>",
		Short: "Set the primary repository",
		Long: `Set a repository as the primary workspace repository.

The primary repository is used as the default for commands that
operate on a single repository.

Examples:
  forge repo primary my-main-repo`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			if err := ws.SetPrimary(name); err != nil {
				return err
			}

			fmt.Printf("✅ Set primary repository: %s\n", name)

			return nil
		},
	}

	return cmd
}

func newRepoFetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch [name]",
		Short: "Fetch updates for repositories",
		Long: `Fetch the latest changes from remote for one or all repositories.

Examples:
  forge repo fetch          # Fetch all repositories
  forge repo fetch my-repo  # Fetch specific repository`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			if len(args) == 0 {
				// Fetch all
				return ws.FetchAll()
			}

			// Fetch specific repo
			name := args[0]
			repo := ws.GetRepo(name)
			if repo == nil {
				return fmt.Errorf("repository %q not found", name)
			}

			fmt.Printf("Fetching %s...\n", name)
			return ws.Pull(name)
		},
	}

	return cmd
}

func newRepoDescribeCmd() *cobra.Command {
	var (
		summary      string
		features     []string
		technologies []string
		devBranch    string
	)

	cmd := &cobra.Command{
		Use:   "describe <name>",
		Short: "Set repository metadata (summary, features, technologies)",
		Long: `Set descriptive metadata for a repository.

This metadata is displayed in CLAUDE.md and helps Claude understand
the purpose and capabilities of each repository.

Examples:
  forge repo describe api --summary "REST API for user management"
  forge repo describe api --features "Authentication,User CRUD,Webhooks"
  forge repo describe api --tech "Go,PostgreSQL,Redis"
  forge repo describe api --dev-branch develop`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			repo := ws.GetRepo(name)
			if repo == nil {
				return fmt.Errorf("repository %q not found", name)
			}

			// Update fields that were provided
			err = ws.UpdateRepo(name, func(r *workspace.Repo) {
				if summary != "" {
					r.Summary = summary
				}
				if len(features) > 0 {
					r.Features = features
				}
				if len(technologies) > 0 {
					r.Technologies = technologies
				}
				if devBranch != "" {
					r.DevBranch = devBranch
				}
			})
			if err != nil {
				return err
			}

			// Reload to show updated values
			ws, _ = findWorkspace()
			repo = ws.GetRepo(name)

			fmt.Printf("✅ Updated repository: %s\n", name)
			if repo.Summary != "" {
				fmt.Printf("   Summary: %s\n", repo.Summary)
			}
			if len(repo.Features) > 0 {
				fmt.Printf("   Features: %s\n", strings.Join(repo.Features, ", "))
			}
			if len(repo.Technologies) > 0 {
				fmt.Printf("   Technologies: %s\n", strings.Join(repo.Technologies, ", "))
			}
			if repo.DevBranch != "" {
				fmt.Printf("   Dev Branch: %s\n", repo.DevBranch)
			}

			// Update CLAUDE.md
			if err := ws.UpdateClaudeMD(); err != nil {
				fmt.Printf("   Warning: failed to update CLAUDE.md: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&summary, "summary", "s", "", "Brief description of the repository")
	cmd.Flags().StringSliceVarP(&features, "features", "f", nil, "Key features (comma-separated)")
	cmd.Flags().StringSliceVarP(&technologies, "tech", "t", nil, "Technologies used (comma-separated)")
	cmd.Flags().StringVar(&devBranch, "dev-branch", "", "Development branch (for worktrees)")

	return cmd
}

func newRepoSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync workspace state and update CLAUDE.md",
		Long: `Regenerate CLAUDE.md with current workspace state.

This updates the CLAUDE.md file with:
- Current repository list
- Updated tool documentation
- Fresh timestamp

Examples:
  forge repo sync`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := findWorkspace()
			if err != nil {
				return err
			}

			if err := ws.UpdateClaudeMD(); err != nil {
				return fmt.Errorf("updating CLAUDE.md: %w", err)
			}

			fmt.Println("✅ Updated CLAUDE.md")

			return nil
		},
	}

	return cmd
}

// findWorkspace finds the workspace for the current directory.
func findWorkspace() (*workspace.Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current directory: %w", err)
	}

	return workspace.Find(cwd)
}
