package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/logs"
)

func newLogsCmd() *cobra.Command {
	var (
		logType string
		clean   bool
		maxAge  time.Duration
		dryRun  bool
		tail    int
		all     bool
	)

	cmd := &cobra.Command{
		Use:   "logs [session-id]",
		Short: "View and manage forge logs",
		Long: `View and manage log files for forge sessions, workers, and leaders.

Without arguments, lists all log files.
With a session ID, shows the log content.

Examples:
  foundry logs                     # List all logs
  foundry logs abc123              # View log for session abc123
  foundry logs --type worker       # List worker logs only
  foundry logs --clean --age 7d    # Remove logs older than 7 days
  foundry logs --clean --dry-run   # Preview cleanup without deleting
  foundry logs abc123 --tail 100   # Show last 100 lines of log`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Clean mode
			if clean {
				return runLogsClean(maxAge, dryRun)
			}

			// View specific log
			if len(args) == 1 {
				return runLogsView(args[0], tail)
			}

			// List logs
			return runLogsList(logType, all)
		},
	}

	cmd.Flags().StringVarP(&logType, "type", "t", "", "Filter by log type (session, worker, leader)")
	cmd.Flags().BoolVarP(&clean, "clean", "c", false, "Remove old log files")
	cmd.Flags().DurationVarP(&maxAge, "age", "a", 7*24*time.Hour, "Max age for cleanup (default 7d)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview cleanup without deleting")
	cmd.Flags().IntVarP(&tail, "tail", "n", 0, "Show last N lines of log")
	cmd.Flags().BoolVar(&all, "all", false, "Show all logs including empty")

	return cmd
}

func runLogsList(filterType string, showAll bool) error {
	opts := logs.ListOptions{}

	if filterType != "" {
		switch filterType {
		case "session", "sessions":
			opts.Type = logs.LogTypeSession
		case "worker", "workers":
			opts.Type = logs.LogTypeWorker
		case "leader", "leaders":
			opts.Type = logs.LogTypeLeader
		default:
			return fmt.Errorf("invalid log type: %s (use session, worker, or leader)", filterType)
		}
	}

	entries, err := logs.ListLogs(opts)
	if err != nil {
		return fmt.Errorf("listing logs: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No log files found.")
		return nil
	}

	// Print header
	fmt.Printf("%-12s %-20s %-10s %-20s %s\n", "TYPE", "NAME", "SIZE", "MODIFIED", "PATH")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────")

	for _, e := range entries {
		if !showAll && e.Size == 0 {
			continue
		}
		fmt.Printf("%-12s %-20s %-10s %-20s %s\n",
			e.Type,
			truncate(e.Name, 20),
			humanize.Bytes(uint64(e.Size)),
			humanize.Time(e.ModTime),
			e.Path,
		)
	}

	// Summary
	total, _ := logs.TotalSize()
	counts, _ := logs.CountByType()
	fmt.Println()
	fmt.Printf("Total: %d files, %s\n", len(entries), humanize.Bytes(uint64(total)))
	fmt.Printf("By type: sessions=%d, workers=%d, leaders=%d\n",
		counts[logs.LogTypeSession],
		counts[logs.LogTypeWorker],
		counts[logs.LogTypeLeader])

	return nil
}

func runLogsView(sessionID string, tailLines int) error {
	// Try to find the log file
	paths := []string{
		logs.SessionLogPath(sessionID),
		logs.WorkerLogPath(sessionID),
		logs.LeaderLogPath(sessionID),
	}

	var logPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			logPath = p
			break
		}
	}

	if logPath == "" {
		// Try direct path
		if _, err := os.Stat(sessionID); err == nil {
			logPath = sessionID
		}
	}

	if logPath == "" {
		return fmt.Errorf("log not found for: %s", sessionID)
	}

	// Read and display log
	content, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}

	if tailLines > 0 {
		lines := splitLines(string(content))
		if len(lines) > tailLines {
			lines = lines[len(lines)-tailLines:]
		}
		for _, line := range lines {
			fmt.Println(line)
		}
	} else {
		fmt.Print(string(content))
	}

	return nil
}

func runLogsClean(maxAge time.Duration, dryRun bool) error {
	if dryRun {
		fmt.Printf("🔍 Previewing cleanup (logs older than %s)...\n", maxAge)
	} else {
		fmt.Printf("🧹 Cleaning logs older than %s...\n", maxAge)
	}

	removed, err := logs.CleanLogs(maxAge, dryRun)
	if err != nil {
		return fmt.Errorf("cleaning logs: %w", err)
	}

	if len(removed) == 0 {
		fmt.Println("No logs to clean.")
		return nil
	}

	var totalSize int64
	for _, e := range removed {
		action := "would remove"
		if !dryRun {
			action = "removed"
		}
		fmt.Printf("  %s: %s (%s)\n", action, filepath.Base(e.Path), humanize.Bytes(uint64(e.Size)))
		totalSize += e.Size
	}

	fmt.Println()
	if dryRun {
		fmt.Printf("Would remove %d files, %s\n", len(removed), humanize.Bytes(uint64(totalSize)))
	} else {
		fmt.Printf("Removed %d files, %s freed\n", len(removed), humanize.Bytes(uint64(totalSize)))
	}

	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
