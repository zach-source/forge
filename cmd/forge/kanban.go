package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/kanban"
	"golang.org/x/term"
)

func newKanbanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kanban",
		Aliases: []string{"kb", "issues"},
		Short:   "Manage local kanban issue tracker",
		Long: `Local kanban-style issue tracker with SQLite storage.

Issues are stored in .forge/kanban.db in the current directory.
Use this for quick task tracking without external dependencies.

Examples:
  forge kanban                     # Show board view
  forge kanban list                # List all issues
  forge kanban add "Fix bug"       # Add new issue
  forge kanban move <id> todo      # Move issue to column
  forge kanban show <id>           # Show issue details`,
	}

	cmd.AddCommand(
		newKanbanBoardCmd(),
		newKanbanListCmd(),
		newKanbanAddCmd(),
		newKanbanShowCmd(),
		newKanbanMoveCmd(),
		newKanbanEditCmd(),
		newKanbanDeleteCmd(),
	)

	// Default to board view
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runKanbanBoard(cmd, args)
	}

	return cmd
}

func getKanbanStore() (*kanban.Store, error) {
	// Look for .forge directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	forgeDir := filepath.Join(cwd, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating .forge directory: %w", err)
	}

	dbPath := filepath.Join(forgeDir, "kanban.db")
	return kanban.NewStore(dbPath)
}

func newKanbanBoardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "board",
		Short: "Show kanban board view",
		RunE:  runKanbanBoard,
	}
}

func runKanbanBoard(cmd *cobra.Command, args []string) error {
	store, err := getKanbanStore()
	if err != nil {
		return err
	}
	defer store.Close()

	board, err := store.GetBoard()
	if err != nil {
		return err
	}

	// Get terminal width
	width := 120
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}

	fmt.Println(kanban.RenderBoard(board, width))
	return nil
}

func newKanbanListCmd() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer store.Close()

			var issues []*kanban.Issue
			if status != "" {
				issues, err = store.List(kanban.Status(status))
			} else {
				issues, err = store.List()
			}
			if err != nil {
				return err
			}

			fmt.Print(kanban.RenderList(issues))
			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status")
	return cmd
}

func newKanbanAddCmd() *cobra.Command {
	var (
		priority    string
		status      string
		labels      string
		assignee    string
		description string
		parentID    string
	)

	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a new issue",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer store.Close()

			issue := &kanban.Issue{
				Title:       strings.Join(args, " "),
				Description: description,
				Priority:    kanban.Priority(priority),
				Status:      kanban.Status(status),
				Assignee:    assignee,
				ParentID:    parentID,
			}

			if labels != "" {
				issue.Labels = strings.Split(labels, ",")
				for i := range issue.Labels {
					issue.Labels[i] = strings.TrimSpace(issue.Labels[i])
				}
			}

			if err := store.Create(issue); err != nil {
				return err
			}

			fmt.Printf("Created issue: %s\n", issue.ID)
			fmt.Printf("  Title: %s\n", issue.Title)
			fmt.Printf("  Status: %s\n", issue.Status)
			return nil
		},
	}

	cmd.Flags().StringVarP(&priority, "priority", "p", "medium", "Priority (low, medium, high, critical)")
	cmd.Flags().StringVarP(&status, "status", "s", "backlog", "Initial status")
	cmd.Flags().StringVarP(&labels, "labels", "l", "", "Comma-separated labels")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assignee")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description")
	cmd.Flags().StringVar(&parentID, "parent", "", "Parent issue ID")

	return cmd
}

func newKanbanShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show issue details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer store.Close()

			issue, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if issue == nil {
				return fmt.Errorf("issue not found: %s", args[0])
			}

			fmt.Printf("ID:          %s\n", issue.ID)
			fmt.Printf("Title:       %s\n", issue.Title)
			fmt.Printf("Status:      %s\n", issue.Status)
			fmt.Printf("Priority:    %s\n", issue.Priority)
			if issue.Description != "" {
				fmt.Printf("Description: %s\n", issue.Description)
			}
			if len(issue.Labels) > 0 {
				fmt.Printf("Labels:      %s\n", strings.Join(issue.Labels, ", "))
			}
			if issue.Assignee != "" {
				fmt.Printf("Assignee:    %s\n", issue.Assignee)
			}
			if issue.ParentID != "" {
				fmt.Printf("Parent:      %s\n", issue.ParentID)
			}
			fmt.Printf("Created:     %s\n", issue.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Updated:     %s\n", issue.UpdatedAt.Format("2006-01-02 15:04:05"))

			// Show children
			children, err := store.GetChildren(issue.ID)
			if err == nil && len(children) > 0 {
				fmt.Printf("\nSubtasks (%d):\n", len(children))
				for _, child := range children {
					fmt.Printf("  • [%s] %s\n", child.Status, child.Title)
				}
			}

			return nil
		},
	}
}

func newKanbanMoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "move <id> <status>",
		Short: "Move issue to a different status",
		Long: `Move an issue to a different kanban column.

Valid statuses: backlog, todo, in_progress, review, done`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer store.Close()

			id := args[0]
			status := kanban.Status(args[1])

			// Validate status
			valid := false
			for _, s := range kanban.ValidStatuses() {
				if s == status {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid status: %s (valid: backlog, todo, in_progress, review, done)", args[1])
			}

			if err := store.Move(id, status); err != nil {
				return err
			}

			fmt.Printf("Moved %s to %s\n", id, status)
			return nil
		},
	}
}

func newKanbanEditCmd() *cobra.Command {
	var (
		title       string
		priority    string
		labels      string
		assignee    string
		description string
	)

	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer store.Close()

			issue, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if issue == nil {
				return fmt.Errorf("issue not found: %s", args[0])
			}

			// Update fields that were provided
			if title != "" {
				issue.Title = title
			}
			if priority != "" {
				issue.Priority = kanban.Priority(priority)
			}
			if labels != "" {
				issue.Labels = strings.Split(labels, ",")
				for i := range issue.Labels {
					issue.Labels[i] = strings.TrimSpace(issue.Labels[i])
				}
			}
			if assignee != "" {
				issue.Assignee = assignee
			}
			if description != "" {
				issue.Description = description
			}

			if err := store.Update(issue); err != nil {
				return err
			}

			fmt.Printf("Updated issue: %s\n", issue.ID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "New title")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "New priority")
	cmd.Flags().StringVarP(&labels, "labels", "l", "", "New labels (comma-separated)")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "New assignee")
	cmd.Flags().StringVarP(&description, "description", "d", "", "New description")

	return cmd
}

func newKanbanDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer store.Close()

			if !force {
				issue, err := store.Get(args[0])
				if err != nil {
					return err
				}
				if issue == nil {
					return fmt.Errorf("issue not found: %s", args[0])
				}
				fmt.Printf("Delete issue \"%s\"? Use --force to confirm\n", issue.Title)
				return nil
			}

			if err := store.Delete(args[0]); err != nil {
				return err
			}

			fmt.Printf("Deleted issue: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")
	return cmd
}
