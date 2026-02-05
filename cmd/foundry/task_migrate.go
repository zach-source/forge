package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

// legacyIssue represents an issue from the old SQLite kanban.db
type legacyIssue struct {
	ID          string
	Title       string
	Description string
	Status      string
	Priority    string
	Labels      []string
	Assignee    string
	ParentID    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func newTaskMigrateCmd() *cobra.Command {
	var (
		dryRun     bool
		dbPath     string
		skipClosed bool
	)

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate issues from legacy SQLite kanban.db to beads",
		Long: `Migrate issues from the old .foundry/kanban.db SQLite database to beads.

This command reads issues from the legacy SQLite database and creates them
in the beads issue tracker (.beads/) using the bd CLI.

Status mapping:
  backlog, todo     → open
  in_progress, review → in_progress
  done              → closed (skipped with --skip-closed)

Priority mapping:
  critical → P0
  high     → P1
  medium   → P2
  low      → P3

Examples:
  foundry kanban migrate              # Migrate from .foundry/kanban.db
  foundry kanban migrate --dry-run    # Preview without creating
  foundry kanban migrate --db /path/to/kanban.db
  foundry kanban migrate --skip-closed  # Skip done issues`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			// Default to .foundry/kanban.db in current directory
			if dbPath == "" {
				dbPath = filepath.Join(cwd, ".foundry", "kanban.db")
			}

			// Check if database exists
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				return fmt.Errorf("legacy database not found: %s", dbPath)
			}

			// Check if bd is available
			if _, err := exec.Command("which", "bd").Output(); err != nil {
				return fmt.Errorf("bd CLI not found - please install beads first")
			}

			// Check if beads is initialized
			beadsDir := filepath.Join(cwd, ".beads")
			if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
				if dryRun {
					fmt.Println("Note: .beads/ directory not found (would run 'bd init')")
				} else {
					fmt.Println("Initializing beads...")
					initCmd := exec.Command("bd", "init")
					initCmd.Dir = cwd
					if out, err := initCmd.CombinedOutput(); err != nil {
						return fmt.Errorf("bd init failed: %s", string(out))
					}
				}
			}

			// Open legacy database
			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				return fmt.Errorf("opening legacy database: %w", err)
			}
			defer db.Close()

			// Read all issues
			issues, err := readLegacyIssues(db)
			if err != nil {
				return fmt.Errorf("reading legacy issues: %w", err)
			}

			if len(issues) == 0 {
				fmt.Println("No issues found in legacy database")
				return nil
			}

			fmt.Printf("Found %d issues in %s\n\n", len(issues), dbPath)

			// Get existing beads to avoid duplicates
			existingTitles := make(map[string]bool)
			if !dryRun {
				listCmd := exec.Command("bd", "list", "--json", "--all", "--limit", "0")
				listCmd.Dir = cwd
				if out, err := listCmd.Output(); err == nil {
					var existing []struct {
						Title string `json:"title"`
					}
					if json.Unmarshal(out, &existing) == nil {
						for _, e := range existing {
							existingTitles[e.Title] = true
						}
					}
				}
			}

			// Migrate each issue
			var (
				created  int
				skipped  int
				failures int
			)

			for _, issue := range issues {
				// Skip closed issues if requested
				if skipClosed && issue.Status == "done" {
					if dryRun {
						fmt.Printf("  SKIP (closed): %s\n", issue.Title)
					}
					skipped++
					continue
				}

				// Skip if already exists
				if existingTitles[issue.Title] {
					fmt.Printf("  SKIP (exists): %s\n", issue.Title)
					skipped++
					continue
				}

				if dryRun {
					bdStatus := legacyToBdStatus(issue.Status)
					bdPriority := legacyToBdPriority(issue.Priority)
					fmt.Printf("  CREATE: [%s] [P%d] %s\n", bdStatus, bdPriority, issue.Title)
					created++
					continue
				}

				// Create in beads
				if err := createBeadFromLegacy(cwd, issue); err != nil {
					fmt.Printf("  FAIL: %s - %v\n", issue.Title, err)
					failures++
					continue
				}

				fmt.Printf("  OK: %s\n", issue.Title)
				created++
			}

			fmt.Println()
			if dryRun {
				fmt.Printf("Dry run complete: %d would be created, %d would be skipped\n", created, skipped)
			} else {
				fmt.Printf("Migration complete: %d created, %d skipped, %d failed\n", created, skipped, failures)
				if failures == 0 && created > 0 {
					fmt.Printf("\nYou can now safely remove the legacy database:\n")
					fmt.Printf("  rm %s\n", dbPath)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview migration without creating issues")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to legacy kanban.db (default: .foundry/kanban.db)")
	cmd.Flags().BoolVar(&skipClosed, "skip-closed", false, "Skip issues with status 'done'")

	return cmd
}

func readLegacyIssues(db *sql.DB) ([]*legacyIssue, error) {
	rows, err := db.Query(`
		SELECT id, title, description, status, priority, labels, assignee, parent_id, created_at, updated_at
		FROM issues ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []*legacyIssue
	for rows.Next() {
		var (
			issue    legacyIssue
			labels   string
			parentID sql.NullString
		)

		err := rows.Scan(
			&issue.ID, &issue.Title, &issue.Description,
			&issue.Status, &issue.Priority, &labels,
			&issue.Assignee, &parentID,
			&issue.CreatedAt, &issue.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning issue: %w", err)
		}

		if labels != "" {
			if err := json.Unmarshal([]byte(labels), &issue.Labels); err != nil {
				// Try comma-separated fallback
				issue.Labels = strings.Split(labels, ",")
			}
		}
		if parentID.Valid {
			issue.ParentID = parentID.String
		}

		issues = append(issues, &issue)
	}

	return issues, rows.Err()
}

func createBeadFromLegacy(workDir string, issue *legacyIssue) error {
	args := []string{"create", issue.Title}

	if issue.Description != "" {
		args = append(args, "-d", issue.Description)
	}

	// Map priority
	bdPriority := legacyToBdPriority(issue.Priority)
	args = append(args, "-p", fmt.Sprintf("%d", bdPriority))

	if issue.Assignee != "" {
		args = append(args, "-a", issue.Assignee)
	}

	if len(issue.Labels) > 0 {
		args = append(args, "-l", strings.Join(issue.Labels, ","))
	}

	// Note: parent_id mapping would require a second pass since IDs change
	// For now, we skip parent relationships

	args = append(args, "--silent")

	cmd := exec.Command("bd", args...)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("%s", string(exitErr.Stderr))
		}
		return err
	}

	newID := strings.TrimSpace(string(out))

	// Set status if not open
	switch bdStatus := legacyToBdStatus(issue.Status); bdStatus {
	case "in_progress":
		updateCmd := exec.Command("bd", "update", newID, "-s", "in_progress")
		updateCmd.Dir = workDir
		if _, err := updateCmd.CombinedOutput(); err != nil {
			// Non-fatal, issue was created
			fmt.Printf("    Warning: could not set status to in_progress\n")
		}
	case "closed":
		closeCmd := exec.Command("bd", "close", newID)
		closeCmd.Dir = workDir
		if _, err := closeCmd.CombinedOutput(); err != nil {
			// Non-fatal, issue was created
			fmt.Printf("    Warning: could not close issue\n")
		}
	}

	return nil
}

func legacyToBdStatus(status string) string {
	switch status {
	case "backlog", "todo":
		return "open"
	case "in_progress", "review":
		return "in_progress"
	case "done":
		return "closed"
	default:
		return "open"
	}
}

func legacyToBdPriority(priority string) int {
	switch priority {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 2
	}
}
