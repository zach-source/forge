package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	complexityPkg "github.com/zach-source/forge/internal/complexity"
	"github.com/zach-source/forge/internal/kanban"
	"golang.org/x/term"
)

func getKanbanStore() (*kanban.Store, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	return kanban.NewStore(cwd)
}

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "task",
		Short:   "Task management with git integration",
		Aliases: []string{"t", "tasks", "kanban", "kb", "issues", "i"},
		Long: `Task management with git integration.

Manage tasks stored in .beads/ with full git branch awareness.

Examples:
  foundry task                         # List all tasks by status
  foundry task list --status review    # Filter by status
  foundry task add "Fix bug" -p high   # Create new task
  foundry task move <id> done          # Move task to status
  foundry task show <id>               # Detailed task view
  foundry task branches                # List task branches with status
  foundry task diff <id>               # Show git diff for task branch
  foundry task log <id>                # Show git log for task branch`,
	}

	cmd.AddCommand(
		newTaskListCmd(),
		newTaskShowCmd(),
		newTaskAddCmd(),
		newTaskMoveCmd(),
		newTaskEditCmd(),
		newTaskDeleteCmd(),
		newTaskBoardCmd(),
		newTaskBranchesCmd(),
		newTaskDiffCmd(),
		newTaskLogCmd(),
		newTaskMigrateCmd(),
	)

	// Default to list
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTaskList(cmd, args, "")
	}

	return cmd
}

func newTaskListCmd() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tasks by status",
		Long: `List tasks from the kanban board with filtering options.

Examples:
  foundry task list                    # All tasks
  foundry task list --status review    # Tasks in review
  foundry task list -s todo            # Tasks in todo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskList(cmd, args, status)
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status (backlog, todo, in_progress, review, merge, done)")
	return cmd
}

func runTaskList(cmd *cobra.Command, args []string, status string) error {
	store, err := getKanbanStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	var issues []*kanban.Issue
	if status != "" {
		issues, err = store.List(kanban.Status(status))
	} else {
		issues, err = store.List()
	}
	if err != nil {
		return err
	}

	if len(issues) == 0 {
		if status != "" {
			fmt.Printf("No tasks with status: %s\n", status)
		} else {
			fmt.Println("No tasks found")
		}
		return nil
	}

	// Group by status for display
	byStatus := make(map[kanban.Status][]*kanban.Issue)
	for _, issue := range issues {
		byStatus[issue.Status] = append(byStatus[issue.Status], issue)
	}

	// Print in status order
	for _, s := range kanban.ValidStatuses() {
		statusIssues := byStatus[s]
		if len(statusIssues) == 0 {
			continue
		}

		// Skip other statuses if filtering
		if status != "" && kanban.Status(status) != s {
			continue
		}

		fmt.Printf("\n%s (%d tasks):\n", toTitle(string(s)), len(statusIssues))

		for _, issue := range statusIssues {
			// Truncate title if too long
			title := issue.Title
			if len(title) > 60 {
				title = title[:57] + "..."
			}

			// Format: ID [priority] [complexity] Title
			comp := ""
			if issue.Complexity.Valid() {
				comp = fmt.Sprintf("[%s] ", issue.Complexity)
			}
			fmt.Printf("  %-18s [%-4s] %s%s\n", issue.ID, issue.Priority, comp, title)
		}
	}

	return nil
}

// toTitle capitalizes the first letter of a string.
func toTitle(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func newTaskShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show detailed task information",
		Long: `Show detailed information about a task including:
- Title, status, priority
- Description and labels
- Associated git branch (if any)
- Worktree path (if task is in progress)
- Timestamps

Examples:
  foundry task show blocks-forge-03k
  foundry task show 03k                # Partial ID match`,
		Args: cobra.ExactArgs(1),
		RunE: runTaskShow,
	}
}

func runTaskShow(cmd *cobra.Command, args []string) error {
	store, err := getKanbanStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	// Try to find the task (supports partial ID match)
	taskID := args[0]
	issue, err := kanban.FindTask(store, taskID)
	if err != nil {
		return err
	}
	if issue == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Print task details
	fmt.Printf("Task:        %s\n", issue.ID)
	fmt.Printf("Title:       %s\n", issue.Title)
	fmt.Printf("Status:      %s\n", issue.Status)
	fmt.Printf("Priority:    %s\n", issue.Priority)

	if issue.Complexity.Valid() {
		fmt.Printf("Complexity:  %s (score: %d)\n", issue.Complexity, complexityPkg.Score(issue.Complexity))
	}
	if issue.ActualComplexity.Valid() {
		fmt.Printf("Actual:      %s\n", issue.ActualComplexity)
	}

	if len(issue.Labels) > 0 {
		fmt.Printf("Labels:      %s\n", strings.Join(issue.Labels, ", "))
	}

	if issue.Assignee != "" {
		fmt.Printf("Assignee:    %s\n", issue.Assignee)
	}

	if issue.Description != "" {
		fmt.Printf("Description: %s\n", issue.Description)
	}

	// Find git branch for this task
	branchSuffix := kanban.ExtractBranchSuffix(issue.ID)
	branch, err := kanban.FindTaskBranch(branchSuffix)
	if err == nil && branch != "" {
		fmt.Printf("Branch:      %s\n", branch)

		// Check for diff
		diffCmd := exec.Command("git", "rev-list", "--count", "main.."+branch)
		if out, err := diffCmd.Output(); err == nil {
			commits := strings.TrimSpace(string(out))
			if commits != "0" {
				fmt.Printf("Commits:     %s ahead of main\n", commits)
			}
		}
	}

	// Check for worktree
	cwd, _ := os.Getwd()
	worktreePath := fmt.Sprintf("%s/.forge/worktrees/task-%s", cwd, branchSuffix)
	if _, err := os.Stat(worktreePath); err == nil {
		fmt.Printf("Worktree:    %s\n", worktreePath)
	}

	fmt.Printf("Created:     %s\n", issue.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:     %s\n", issue.UpdatedAt.Format("2006-01-02 15:04:05"))

	// Show children/subtasks
	children, err := store.GetChildren(issue.ID)
	if err == nil && len(children) > 0 {
		fmt.Printf("\nSubtasks (%d):\n", len(children))
		for _, child := range children {
			fmt.Printf("  • [%s] %s - %s\n", child.Status, child.ID, child.Title)
		}
	}

	return nil
}

func newTaskBranchesCmd() *cobra.Command {
	var showAll bool

	cmd := &cobra.Command{
		Use:   "branches",
		Short: "List task branches with their kanban status",
		Long: `List all git branches matching the task/* pattern and show their
corresponding kanban status.

Examples:
  foundry task branches                # List active task branches
  foundry task branches --all          # Include merged/stale branches`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskBranches(showAll)
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all branches including merged")
	return cmd
}

func runTaskBranches(showAll bool) error {
	store, err := getKanbanStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	// Get task branches with status
	branches, err := kanban.ListTaskBranches(store)
	if err != nil {
		return fmt.Errorf("listing branches: %w", err)
	}

	if len(branches) == 0 {
		fmt.Println("No task branches found")
		return nil
	}

	// Print header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "BRANCH\tSTATUS\tTASK ID\tTITLE")

	for _, tb := range branches {
		// Skip merged branches unless --all
		if !showAll && !tb.HasDiff && tb.TaskID == "" {
			continue
		}

		// Format output
		status := "-"
		taskID := "-"
		title := "(no matching task)"

		if tb.TaskID != "" {
			status = string(tb.Status)
			taskID = tb.TaskID
			title = tb.Title
			if len(title) > 50 {
				title = title[:47] + "..."
			}
		}

		diffMarker := ""
		if tb.HasDiff {
			diffMarker = "*"
		}

		_, _ = fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\n", tb.Branch, diffMarker, status, taskID, title)
	}

	_ = w.Flush()
	fmt.Println("\n* = has unmerged commits")
	return nil
}

func newTaskDiffCmd() *cobra.Command {
	var stat bool

	cmd := &cobra.Command{
		Use:   "diff <id>",
		Short: "Show git diff for a task's branch",
		Long: `Show the git diff between main and a task's branch.

Examples:
  foundry task diff 03k                # Full diff
  foundry task diff 03k --stat         # Summary only`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskDiff(args[0], stat)
		},
	}

	cmd.Flags().BoolVar(&stat, "stat", false, "Show only diff stats")
	return cmd
}

func runTaskDiff(taskID string, stat bool) error {
	// Find the branch for this task
	branchSuffix := kanban.ExtractBranchSuffix(taskID)
	branch, err := kanban.FindTaskBranch(branchSuffix)
	if err != nil {
		return err
	}
	if branch == "" {
		return fmt.Errorf("no branch found for task: %s (tried task/%s)", taskID, branchSuffix)
	}

	// Build diff command
	args := []string{"diff", "main.." + branch}
	if stat {
		args = append(args, "--stat")
	}

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func newTaskLogCmd() *cobra.Command {
	var numCommits int

	cmd := &cobra.Command{
		Use:   "log <id>",
		Short: "Show git log for a task's branch",
		Long: `Show the git commit history for a task's branch since diverging from main.

Examples:
  foundry task log 03k                 # All commits since main
  foundry task log 03k -n 5            # Last 5 commits`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskLog(args[0], numCommits)
		},
	}

	cmd.Flags().IntVarP(&numCommits, "number", "n", 0, "Limit number of commits")
	return cmd
}

func runTaskLog(taskID string, numCommits int) error {
	// Find the branch for this task
	branchSuffix := kanban.ExtractBranchSuffix(taskID)
	branch, err := kanban.FindTaskBranch(branchSuffix)
	if err != nil {
		return err
	}
	if branch == "" {
		return fmt.Errorf("no branch found for task: %s (tried task/%s)", taskID, branchSuffix)
	}

	// Build log command
	args := []string{"log", "main.." + branch, "--oneline"}
	if numCommits > 0 {
		args = append(args, fmt.Sprintf("-n%d", numCommits))
	}

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func newTaskAddCmd() *cobra.Command {
	var (
		priority       string
		status         string
		labels         string
		assignee       string
		description    string
		parentID       string
		complexityFlag string
	)

	cmd := &cobra.Command{
		Use:     "add <title>",
		Aliases: []string{"new", "create"},
		Short:   "Add a new task",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			issue := &kanban.Issue{
				Title:       strings.Join(args, " "),
				Description: description,
				Priority:    kanban.Priority(priority),
				Status:      kanban.Status(status),
				Assignee:    assignee,
				ParentID:    parentID,
			}

			if complexityFlag != "" {
				c, err := complexityPkg.Parse(complexityFlag)
				if err != nil {
					return err
				}
				issue.Complexity = c
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

			fmt.Printf("Created: %s\n", issue.ID)
			fmt.Printf("  %s [%s] %s\n", issue.ID, issue.Priority, issue.Title)
			return nil
		},
	}

	cmd.Flags().StringVarP(&priority, "priority", "p", "medium", "Priority (low, medium, high, critical)")
	cmd.Flags().StringVarP(&status, "status", "s", "backlog", "Initial status")
	cmd.Flags().StringVarP(&labels, "labels", "l", "", "Comma-separated labels")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assignee")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description")
	cmd.Flags().StringVar(&parentID, "parent", "", "Parent task ID")
	cmd.Flags().StringVarP(&complexityFlag, "complexity", "c", "", "Complexity estimate (S, M, L, XL)")

	return cmd
}

func newTaskMoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "move <id> <status>",
		Short: "Move task to a different status",
		Long: `Move a task to a different status column.

Valid statuses: backlog, todo, in_progress, review, merge, done

Shortcuts:
  b = backlog
  t = todo
  p = in_progress (progress)
  r = review
  m = merge
  d = done`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			id := args[0]
			statusArg := args[1]

			// Handle shortcuts
			status := expandStatus(statusArg)

			// Validate status
			valid := false
			for _, s := range kanban.ValidStatuses() {
				if s == status {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid status: %s (valid: backlog, todo, in_progress, review, merge, done)", statusArg)
			}

			if err := store.Move(id, status); err != nil {
				return err
			}

			fmt.Printf("Moved %s → %s\n", id, status)
			return nil
		},
	}
}

func expandStatus(s string) kanban.Status {
	switch strings.ToLower(s) {
	case "b", "backlog":
		return kanban.StatusBacklog
	case "t", "todo":
		return kanban.StatusTodo
	case "p", "progress", "in_progress", "wip":
		return kanban.StatusInProgress
	case "r", "review":
		return kanban.StatusReview
	case "m", "merge":
		return kanban.StatusMerge
	case "d", "done", "complete":
		return kanban.StatusDone
	default:
		return kanban.Status(s)
	}
}

func newTaskEditCmd() *cobra.Command {
	var (
		title          string
		priority       string
		labels         string
		assignee       string
		description    string
		complexityFlag string
	)

	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			issue, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if issue == nil {
				return fmt.Errorf("task not found: %s", args[0])
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
			if complexityFlag != "" {
				c, err := complexityPkg.Parse(complexityFlag)
				if err != nil {
					return err
				}
				issue.Complexity = c
			}

			if err := store.Update(issue); err != nil {
				return err
			}

			fmt.Printf("Updated: %s\n", issue.ID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "New title")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "New priority")
	cmd.Flags().StringVarP(&labels, "labels", "l", "", "New labels (comma-separated)")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "New assignee")
	cmd.Flags().StringVarP(&description, "description", "d", "", "New description")
	cmd.Flags().StringVarP(&complexityFlag, "complexity", "c", "", "Complexity estimate (S, M, L, XL)")

	return cmd
}

func newTaskDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete a task",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			if !force {
				issue, err := store.Get(args[0])
				if err != nil {
					return err
				}
				if issue == nil {
					return fmt.Errorf("task not found: %s", args[0])
				}
				fmt.Printf("Delete \"%s\"? Use --force to confirm\n", issue.Title)
				return nil
			}

			if err := store.Delete(args[0]); err != nil {
				return err
			}

			fmt.Printf("Deleted: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion")
	return cmd
}

func newTaskBoardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "board",
		Short: "Show kanban board view",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getKanbanStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

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
		},
	}
}
