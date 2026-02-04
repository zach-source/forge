package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeMDFile is the name of the Claude instructions file.
const ClaudeMDFile = "CLAUDE.md"

// GenerateClaudeMD generates the CLAUDE.md content for a workspace.
func (w *Workspace) GenerateClaudeMD() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`# %s

> Forge Workspace - Auto-generated

## Workspace Overview

This is a forge-managed workspace for autonomous Claude execution.

`, w.Name))

	if w.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", w.Description))
	}

	// Repositories section with detailed info
	sb.WriteString("## Repositories\n\n")
	if len(w.Repos) == 0 {
		sb.WriteString("No repositories added yet. Add with:\n")
		sb.WriteString("```bash\n")
		sb.WriteString("forge repo add <git-url>     # Clone a repo\n")
		sb.WriteString("forge repo link <path>       # Link existing repo\n")
		sb.WriteString("```\n\n")
	} else {
		for _, repo := range w.Repos {
			// Repo header
			name := repo.Name
			if repo.IsPrimary {
				name = name + " ⭐"
			}
			sb.WriteString(fmt.Sprintf("### %s\n\n", name))

			// Summary
			if repo.Summary != "" {
				sb.WriteString(fmt.Sprintf("%s\n\n", repo.Summary))
			}

			// Metadata table
			sb.WriteString("| Property | Value |\n")
			sb.WriteString("|----------|-------|\n")
			sb.WriteString(fmt.Sprintf("| Path | `%s` |\n", repo.Path))
			sb.WriteString(fmt.Sprintf("| Branch | %s |\n", repo.Branch))
			if repo.DevBranch != "" && repo.DevBranch != repo.Branch {
				sb.WriteString(fmt.Sprintf("| Dev Branch | %s |\n", repo.DevBranch))
			}
			repoType := "cloned"
			if repo.IsLinked {
				repoType = "linked"
			}
			sb.WriteString(fmt.Sprintf("| Type | %s |\n", repoType))
			sb.WriteString("\n")

			// Features
			if len(repo.Features) > 0 {
				sb.WriteString("**Features:**\n")
				for _, f := range repo.Features {
					sb.WriteString(fmt.Sprintf("- %s\n", f))
				}
				sb.WriteString("\n")
			}

			// Technologies
			if len(repo.Technologies) > 0 {
				sb.WriteString(fmt.Sprintf("**Technologies:** %s\n\n", strings.Join(repo.Technologies, ", ")))
			}
		}
	}

	// Active worktrees section
	worktrees, _ := w.ActiveWorktrees()
	if len(worktrees) > 0 {
		sb.WriteString("## Active Worktrees\n\n")
		sb.WriteString("| Feature | Repo | Branch | Path |\n")
		sb.WriteString("|---------|------|--------|------|\n")
		for _, wt := range worktrees {
			path := wt.Path
			if len(path) > 30 {
				path = "..." + path[len(path)-27:]
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | `%s` | `%s` |\n",
				wt.Name, wt.RepoName, wt.Branch, path))
		}
		sb.WriteString("\n")
	}

	// Tools section
	sb.WriteString(`## Forge Tools

### Workspace Management
` + "```bash" + `
forge init [dir]           # Initialize workspace
forge repo add <url>       # Clone and add repo
forge repo link <path>     # Link existing repo
forge repo list            # List repos
forge repo primary <name>  # Set primary repo
forge repo sync            # Update CLAUDE.md
` + "```" + `

### Worktree Development
` + "```bash" + `
forge work start <name>    # Create worktree for feature
forge work list            # List active worktrees
forge work attach <name>   # Launch Claude in worktree
forge work complete <name> # Mark ready for merge
forge work abandon <name>  # Remove worktree
` + "```" + `

### Board & Notion Sync
` + "```bash" + `
forge board --config       # Configure Notion database
forge board --sync         # One-shot sync
forge board                # Interactive session
` + "```" + `

### Leader Commands
` + "```bash" + `
forge planner              # Strategic planning & roadmap
forge reviewer             # Code review & issue creation
forge merge                # Single-threaded merge coordination
forge deploy               # Deployment & smoke testing
` + "```" + `

### Autonomous Execution
` + "```bash" + `
forge start "<task>" -p DONE    # Start autonomous task
forge attach                    # Attach to running session
forge status                    # View session status
forge monitor                   # TUI dashboard
forge cancel                    # Cancel session
` + "```" + `

## Development Workflow

### Worktree-Based Development

1. **Plan**: ` + "`forge planner`" + ` → Create epics/features in Notion
2. **Create worktree**: ` + "`forge work start \"feature-name\" --repo api`" + `
3. **Develop**: Claude works in isolated worktree
4. **Complete**: ` + "`forge work complete \"feature-name\"`" + `
5. **Review**: ` + "`forge reviewer`" + ` → Review from main/develop
6. **Merge**: ` + "`forge merge`" + ` → Single-threaded merge to main
7. **Deploy**: ` + "`forge deploy`" + ` → Deploy & smoke test

### Directory Structure

` + "```" + `
workspace/
├── CLAUDE.md                 # This file (auto-updated)
├── .forge/
│   ├── workspace.yaml        # Workspace config
│   ├── repos/                # Cloned repositories
│   ├── worktrees/            # Feature worktrees
│   │   ├── <repo>/
│   │   │   └── <feature>/    # Isolated working directory
│   │   └── worktrees.yaml    # Worktree tracking
│   ├── sessions/             # Session state
│   └── logs/                 # Log files
└── (linked repos)
` + "```" + `

### Leader Responsibilities

| Leader | Works From | Creates |
|--------|------------|---------|
| Planner | Notion | Epics, Features in Notion + Beads |
| Reviewer | main/develop | Issues for findings |
| Merge Leader | Feature branches | Merges to main |
| Deploy Leader | main | Issues for failures |

---

*Last updated: ` + time.Now().Format("2006-01-02 15:04:05") + `*
`)

	return sb.String()
}

// WriteClaudeMD writes the CLAUDE.md file to the workspace.
func (w *Workspace) WriteClaudeMD() error {
	content := w.GenerateClaudeMD()
	path := filepath.Join(w.Path, ClaudeMDFile)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing CLAUDE.md: %w", err)
	}

	return nil
}

// UpdateClaudeMD regenerates and writes the CLAUDE.md file.
func (w *Workspace) UpdateClaudeMD() error {
	return w.WriteClaudeMD()
}

// HasClaudeMD returns true if CLAUDE.md exists.
func (w *Workspace) HasClaudeMD() bool {
	path := filepath.Join(w.Path, ClaudeMDFile)
	_, err := os.Stat(path)
	return err == nil
}
