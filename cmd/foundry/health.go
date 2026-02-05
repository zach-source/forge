package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/tmux"
	"github.com/zach-source/forge/internal/worker"
)

// HealthCheck represents a single health check result
type HealthCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass", "fail", "warn"
	Message string `json:"message"`
	TaskID  string `json:"task_id,omitempty"`
}

// RecentMerge represents a recent merge to main
type RecentMerge struct {
	Commit string `json:"commit"`
	TaskID string `json:"task_id,omitempty"`
	Title  string `json:"title"`
	Date   string `json:"date"`
	Author string `json:"author,omitempty"`
}

// HealthReport is the full system health report
type HealthReport struct {
	Timestamp    time.Time     `json:"timestamp"`
	Workspace    string        `json:"workspace"`
	GitRepo      string        `json:"git_repo,omitempty"` // Set when different from workspace
	Checks       []HealthCheck `json:"checks"`
	RecentMerges []RecentMerge `json:"recent_merges,omitempty"`
	Summary      struct {
		Passing  int `json:"passing"`
		Failing  int `json:"failing"`
		Warnings int `json:"warnings"`
	} `json:"summary"`
}

// InfraStatus represents the infrastructure health status written by the monitor leader
type InfraStatus struct {
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"` // healthy, degraded, critical
	Cluster   struct {
		NodesReady  int `json:"nodes_ready"`
		NodesTotal  int `json:"nodes_total"`
		PodsRunning int `json:"pods_running"`
		PodsFailed  int `json:"pods_failed"`
	} `json:"cluster"`
	Alerts struct {
		Critical int `json:"critical"`
		Warning  int `json:"warning"`
		Info     int `json:"info"`
	} `json:"alerts"`
	Issues []struct {
		Severity       string `json:"severity"`
		Component      string `json:"component"`
		Message        string `json:"message"`
		Recommendation string `json:"recommendation"`
	} `json:"issues"`
	Services struct {
		Healthy  []string `json:"healthy"`
		Degraded []string `json:"degraded"`
		Down     []string `json:"down"`
	} `json:"services"`
	Metrics struct {
		ErrorRatePercent float64 `json:"error_rate_percent"`
		P95LatencyMs     float64 `json:"p95_latency_ms"`
		PodRestarts24h   int     `json:"pod_restarts_24h"`
	} `json:"metrics"`
}

func newHealthCmd() *cobra.Command {
	var (
		createTasks bool
		jsonOutput  bool
		workspace   string
	)

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check system health and optionally create tasks for issues",
		Long: `Inspect the foundry/forge system state and report issues.

Checks performed:
  - Supervisor process status
  - CI/CD pipeline status (via gh CLI)
  - Forge tmux sessions
  - Task board state (stuck tasks, empty queues)
  - Worker registry health

Subcommands:
  foundry health status   # Show infrastructure health from monitor leader

Examples:
  foundry health                    # Check health in current directory
  foundry health -d /path/to/repo   # Check specific workspace
  foundry health --create-tasks     # Create tasks for broken items
  foundry health --json             # Output as JSON
  foundry health status             # Show infrastructure status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspace == "" {
				var err error
				workspace, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			// Find git root from workspace (may be in .forge/repos/)
			gitRepo := workspace
			if gitRoot, err := findGitRoot(workspace); err == nil {
				gitRepo = gitRoot
			}

			report := runHealthChecks(workspace, gitRepo, createTasks)

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			printHealthReport(report)
			return nil
		},
	}

	cmd.Flags().BoolVar(&createTasks, "create-tasks", false, "Create tasks for failing checks")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVarP(&workspace, "dir", "d", "", "Workspace directory (default: current)")

	// Add status subcommand
	cmd.AddCommand(newHealthStatusCmd())

	return cmd
}

func newHealthStatusCmd() *cobra.Command {
	var (
		jsonOutput bool
		workspace  string
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show infrastructure health status from monitor leader",
		Long: `Display the infrastructure health status written by the monitor leader.

The monitor leader periodically writes health status to .forge/health/status.json.
This command reads and displays that status in a human-readable format.

The status includes:
  - Overall cluster health (healthy/degraded/critical)
  - Node and pod status
  - Active alerts by severity
  - Service health
  - Key metrics (error rate, latency, restarts)
  - Known issues with recommendations`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspace == "" {
				var err error
				workspace, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			return runHealthStatus(workspace, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVarP(&workspace, "dir", "d", "", "Workspace directory (default: current)")

	return cmd
}

func runHealthStatus(workspace string, jsonOutput bool) error {
	statusFile := filepath.Join(workspace, ".forge", "health", "status.json")

	data, err := os.ReadFile(statusFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("📡 No infrastructure status available")
			fmt.Println()
			fmt.Println("The monitor leader has not written a status file yet.")
			fmt.Println("Status file location: .forge/health/status.json")
			fmt.Println()
			fmt.Println("To generate status:")
			fmt.Println("  1. Start supervisor with leaders: foundry supervisor --leaders")
			fmt.Println("  2. Or start monitor manually: foundry worker start <monitor-worker>")
			return nil
		}
		return fmt.Errorf("reading status file: %w", err)
	}

	var status InfraStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return fmt.Errorf("parsing status file: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	printInfraStatus(&status)
	return nil
}

func printInfraStatus(status *InfraStatus) {
	// Header with overall status
	statusEmoji := "✅"
	switch status.Status {
	case "degraded":
		statusEmoji = "⚠️"
	case "critical":
		statusEmoji = "🔴"
	}

	fmt.Printf("%s Infrastructure Status: %s\n", statusEmoji, strings.ToUpper(status.Status))
	fmt.Printf("   Last updated: %s\n\n", status.Timestamp)

	// Cluster health
	fmt.Println("📦 Cluster:")
	fmt.Printf("   Nodes: %d/%d ready\n", status.Cluster.NodesReady, status.Cluster.NodesTotal)
	fmt.Printf("   Pods:  %d running, %d failed\n", status.Cluster.PodsRunning, status.Cluster.PodsFailed)
	fmt.Println()

	// Alerts
	totalAlerts := status.Alerts.Critical + status.Alerts.Warning + status.Alerts.Info
	if totalAlerts > 0 {
		fmt.Println("🚨 Alerts:")
		if status.Alerts.Critical > 0 {
			fmt.Printf("   🔴 Critical: %d\n", status.Alerts.Critical)
		}
		if status.Alerts.Warning > 0 {
			fmt.Printf("   ⚠️  Warning:  %d\n", status.Alerts.Warning)
		}
		if status.Alerts.Info > 0 {
			fmt.Printf("   ℹ️  Info:     %d\n", status.Alerts.Info)
		}
		fmt.Println()
	}

	// Services
	fmt.Println("🔌 Services:")
	if len(status.Services.Down) > 0 {
		fmt.Printf("   🔴 Down:     %s\n", strings.Join(status.Services.Down, ", "))
	}
	if len(status.Services.Degraded) > 0 {
		fmt.Printf("   ⚠️  Degraded: %s\n", strings.Join(status.Services.Degraded, ", "))
	}
	if len(status.Services.Healthy) > 0 {
		fmt.Printf("   ✅ Healthy:  %s\n", strings.Join(status.Services.Healthy, ", "))
	}
	fmt.Println()

	// Metrics
	fmt.Println("📊 Metrics:")
	fmt.Printf("   Error rate:    %.2f%%\n", status.Metrics.ErrorRatePercent)
	fmt.Printf("   P95 latency:   %.0fms\n", status.Metrics.P95LatencyMs)
	fmt.Printf("   Pod restarts:  %d (24h)\n", status.Metrics.PodRestarts24h)
	fmt.Println()

	// Issues
	if len(status.Issues) > 0 {
		fmt.Println("⚡ Known Issues:")
		for _, issue := range status.Issues {
			severityIcon := "ℹ️"
			switch issue.Severity {
			case "critical":
				severityIcon = "🔴"
			case "warning":
				severityIcon = "⚠️"
			}
			fmt.Printf("   %s [%s] %s\n", severityIcon, issue.Component, issue.Message)
			if issue.Recommendation != "" {
				fmt.Printf("      → %s\n", issue.Recommendation)
			}
		}
	}
}

func runHealthChecks(workspace, gitRepo string, createTasks bool) *HealthReport {
	report := &HealthReport{
		Timestamp: time.Now(),
		Workspace: workspace,
	}

	// Set GitRepo if different from workspace
	if gitRepo != workspace {
		report.GitRepo = gitRepo
	}

	// 1. Check supervisor
	report.Checks = append(report.Checks, checkSupervisor())

	// 2. Check forge tmux server
	report.Checks = append(report.Checks, checkForgeServer())

	// 3. Check CI status (uses git repo for gh commands)
	report.Checks = append(report.Checks, checkCI(gitRepo, createTasks))

	// 4. Check worker registry
	report.Checks = append(report.Checks, checkWorkerRegistry())

	// 5. Check task board (uses workspace for .beads/)
	report.Checks = append(report.Checks, checkTaskBoard(workspace, createTasks)...)

	// 6. Get recent merges to main
	report.RecentMerges = getRecentMerges(gitRepo, 5)

	// Calculate summary
	for _, check := range report.Checks {
		switch check.Status {
		case "pass":
			report.Summary.Passing++
		case "fail":
			report.Summary.Failing++
		case "warn":
			report.Summary.Warnings++
		}
	}

	return report
}

func checkSupervisor() HealthCheck {
	cmd := exec.Command("pgrep", "-f", "foundry supervisor")
	if err := cmd.Run(); err != nil {
		return HealthCheck{
			Name:    "Supervisor",
			Status:  "fail",
			Message: "No supervisor process running",
		}
	}
	return HealthCheck{
		Name:    "Supervisor",
		Status:  "pass",
		Message: "Supervisor is running",
	}
}

func checkForgeServer() HealthCheck {
	if tmux.ServerRunning() {
		sessions, _ := tmux.ListSessions("")
		return HealthCheck{
			Name:    "Forge Server",
			Status:  "pass",
			Message: fmt.Sprintf("Forge tmux server running with %d sessions", len(sessions)),
		}
	}
	return HealthCheck{
		Name:    "Forge Server",
		Status:  "warn",
		Message: "Forge tmux server not running (will start on first session)",
	}
}

func checkCI(workspace string, createTask bool) HealthCheck {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "run", "list", "--limit", "5", "--json", "status,conclusion,name,createdAt")
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return HealthCheck{
				Name:    "CI/CD",
				Status:  "warn",
				Message: "CI check timed out",
			}
		}
		// Extract stderr for better error message
		msg := "Could not check CI status"
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(stderr, "not a git repository") {
				msg = "Not a git repository"
			} else if strings.Contains(stderr, "Could not resolve") {
				msg = "No GitHub remote configured"
			} else if len(stderr) < 100 {
				msg = stderr
			}
		}
		return HealthCheck{
			Name:    "CI/CD",
			Status:  "warn",
			Message: msg,
		}
	}

	var runs []struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		Name       string `json:"name"`
		CreatedAt  string `json:"createdAt"`
	}
	if err := json.Unmarshal(out, &runs); err != nil {
		return HealthCheck{
			Name:    "CI/CD",
			Status:  "warn",
			Message: "Could not parse CI status",
		}
	}

	if len(runs) == 0 {
		return HealthCheck{
			Name:    "CI/CD",
			Status:  "pass",
			Message: "No CI runs found",
		}
	}

	// Check for consecutive failures
	failures := 0
	for _, run := range runs {
		if run.Conclusion == "failure" {
			failures++
		} else {
			break
		}
	}

	if failures >= 3 {
		msg := fmt.Sprintf("CI failing: %d consecutive failures (last: %s)", failures, runs[0].Name)
		check := HealthCheck{
			Name:    "CI/CD",
			Status:  "fail",
			Message: msg,
		}

		if createTask {
			taskID := createCIFixTask(workspace, runs[0].Name, failures)
			if taskID != "" {
				check.TaskID = taskID
			}
		}
		return check
	}

	if failures > 0 {
		return HealthCheck{
			Name:    "CI/CD",
			Status:  "warn",
			Message: fmt.Sprintf("CI has %d recent failure(s)", failures),
		}
	}

	return HealthCheck{
		Name:    "CI/CD",
		Status:  "pass",
		Message: "CI passing",
	}
}

func checkWorkerRegistry() HealthCheck {
	reg, err := worker.LoadRegistry()
	if err != nil {
		return HealthCheck{
			Name:    "Worker Registry",
			Status:  "warn",
			Message: "Could not load worker registry",
		}
	}

	workers := reg.List()
	active := 0
	stale := 0

	for _, w := range workers {
		if w.Status == worker.StatusActive {
			active++
			// Check if session actually exists
			session := tmux.NewSession(w.SessionID, "", "")
			if !session.Exists() {
				stale++
			}
		}
	}

	if stale > 0 {
		return HealthCheck{
			Name:    "Worker Registry",
			Status:  "warn",
			Message: fmt.Sprintf("%d workers marked active but sessions missing", stale),
		}
	}

	return HealthCheck{
		Name:    "Worker Registry",
		Status:  "pass",
		Message: fmt.Sprintf("%d workers registered, %d active", len(workers), active),
	}
}

func checkTaskBoard(workspace string, createTask bool) []HealthCheck {
	var checks []HealthCheck

	store, err := kanban.NewStore(workspace)
	if err != nil {
		return []HealthCheck{{
			Name:    "Task Board",
			Status:  "warn",
			Message: "Could not access task board",
		}}
	}
	defer func() { _ = store.Close() }()

	board, err := store.GetBoard()
	if err != nil {
		return []HealthCheck{{
			Name:    "Task Board",
			Status:  "warn",
			Message: "Could not load task board",
		}}
	}

	// Count by status
	counts := make(map[string]int)
	for _, col := range board.Columns {
		counts[string(col.Status)] = len(col.Issues)
	}

	// Check for stuck workflow
	if counts["todo"] == 0 && counts["in_progress"] == 0 && counts["backlog"] > 0 {
		checks = append(checks, HealthCheck{
			Name:    "Task Flow",
			Status:  "warn",
			Message: fmt.Sprintf("Backlog has %d items but todo is empty - groomer may be needed", counts["backlog"]),
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:   "Task Flow",
			Status: "pass",
			Message: fmt.Sprintf("Backlog:%d Todo:%d WIP:%d Review:%d Done:%d",
				counts["backlog"], counts["todo"], counts["in_progress"], counts["review"], counts["done"]),
		})
	}

	return checks
}

func createCIFixTask(workspace, failedRun string, failures int) string {
	store, err := kanban.NewStore(workspace)
	if err != nil {
		return ""
	}
	defer func() { _ = store.Close() }()

	// Check if a CI fix task already exists
	issues, _ := store.List(kanban.StatusTodo, kanban.StatusInProgress)
	for _, issue := range issues {
		if strings.Contains(strings.ToLower(issue.Title), "ci") && strings.Contains(strings.ToLower(issue.Title), "fix") {
			return issue.ID // Already exists
		}
	}

	issue := &kanban.Issue{
		Title:       fmt.Sprintf("Fix CI: %d consecutive failures", failures),
		Description: fmt.Sprintf("CI has been failing for %d consecutive runs.\n\nLast failed run: %s\n\nInvestigate and fix the build.", failures, failedRun),
		Priority:    kanban.PriorityHigh,
		Status:      kanban.StatusTodo,
		Labels:      []string{"bug", "ci", "system"},
	}

	if err := store.Create(issue); err != nil {
		return ""
	}

	return issue.ID
}

func printHealthReport(report *HealthReport) {
	fmt.Printf("🏥 System Health Report\n")
	fmt.Printf("   Workspace: %s\n", report.Workspace)
	if report.GitRepo != "" {
		fmt.Printf("   Git Repo:  %s\n", report.GitRepo)
	}
	fmt.Printf("   Time: %s\n\n", report.Timestamp.Format(time.RFC3339))

	// Group by status
	var passing, failing, warnings []HealthCheck
	for _, check := range report.Checks {
		switch check.Status {
		case "pass":
			passing = append(passing, check)
		case "fail":
			failing = append(failing, check)
		case "warn":
			warnings = append(warnings, check)
		}
	}

	if len(failing) > 0 {
		fmt.Println("❌ Failing:")
		for _, check := range failing {
			fmt.Printf("   • %s: %s\n", check.Name, check.Message)
			if check.TaskID != "" {
				fmt.Printf("     → Task created: %s\n", check.TaskID)
			}
		}
		fmt.Println()
	}

	if len(warnings) > 0 {
		fmt.Println("⚠️  Warnings:")
		for _, check := range warnings {
			fmt.Printf("   • %s: %s\n", check.Name, check.Message)
		}
		fmt.Println()
	}

	if len(passing) > 0 {
		fmt.Println("✅ Passing:")
		for _, check := range passing {
			fmt.Printf("   • %s: %s\n", check.Name, check.Message)
		}
		fmt.Println()
	}

	// Show recent merges
	if len(report.RecentMerges) > 0 {
		fmt.Println("📦 Recent Merges:")
		for _, m := range report.RecentMerges {
			if m.TaskID != "" {
				fmt.Printf("   • %s [%s] task/%s: %s\n", m.Commit, m.Date, m.TaskID, m.Title)
			} else {
				fmt.Printf("   • %s [%s] %s\n", m.Commit, m.Date, m.Title)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Summary: %d passing, %d failing, %d warnings\n",
		report.Summary.Passing, report.Summary.Failing, report.Summary.Warnings)
}

// findGitRoot finds the primary git repository for health checks.
// If dir is a git repo, returns its root. Otherwise looks in .forge/repos/.
func findGitRoot(dir string) (string, error) {
	// Check if dir itself is a git repo
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	// Look for repos in .forge/repos/ (workspace pattern)
	reposDir := filepath.Join(dir, ".forge", "repos")
	entries, err := os.ReadDir(reposDir)
	if err != nil {
		return "", fmt.Errorf("not a git repo and no repos in .forge/repos/")
	}

	// Find first directory that's a git repo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoPath := filepath.Join(reposDir, entry.Name())
		cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--show-toplevel")
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}

	return "", fmt.Errorf("no git repos found in .forge/repos/")
}

// getRecentMerges returns the last n merges to main with associated task IDs.
func getRecentMerges(gitRepo string, n int) []RecentMerge {
	var merges []RecentMerge

	// Get recent merge commits on main
	// Format: commit hash, date, author, subject
	cmd := exec.Command("git", "-C", gitRepo, "log", "main", "--first-parent",
		"--format=%h|%ad|%an|%s", "--date=short", "-n", fmt.Sprintf("%d", n*2))
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		commit := parts[0]
		date := parts[1]
		author := parts[2]
		subject := parts[3]

		// Extract task ID from merge commit message
		// Pattern: "Merge task/<id>: <title>" or just look for task/ in subject
		taskID := ""
		title := subject

		if strings.Contains(subject, "task/") {
			// Try to extract task ID
			if idx := strings.Index(subject, "task/"); idx >= 0 {
				rest := subject[idx+5:]
				// Task ID is until next space, colon, or end
				endIdx := strings.IndexAny(rest, " :")
				if endIdx > 0 {
					taskID = rest[:endIdx]
				} else if len(rest) > 0 {
					taskID = rest
				}
			}
			// Extract title after colon if present
			if idx := strings.Index(subject, ": "); idx >= 0 {
				title = subject[idx+2:]
			}
		}

		merges = append(merges, RecentMerge{
			Commit: commit,
			TaskID: taskID,
			Title:  title,
			Date:   date,
			Author: author,
		})

		if len(merges) >= n {
			break
		}
	}

	return merges
}
