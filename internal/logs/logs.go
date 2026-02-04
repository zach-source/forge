// Package logs provides log file management for forge sessions, workers, and leaders.
package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LogType represents the type of log file.
type LogType string

const (
	LogTypeSession LogType = "session"
	LogTypeWorker  LogType = "worker"
	LogTypeLeader  LogType = "leader"
)

// LogEntry represents a log file with metadata.
type LogEntry struct {
	Path      string
	Name      string
	Type      LogType
	Size      int64
	ModTime   time.Time
	SessionID string
}

// LogDir returns the default directory for forge logs.
// This is a variable so it can be overridden in tests.
var LogDir = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "logs")
}

// SessionLogPath returns the path to a session's log file.
func SessionLogPath(sessionID string) string {
	return filepath.Join(LogDir(), "sessions", sessionID+".log")
}

// WorkerLogPath returns the path to a worker's log file.
func WorkerLogPath(workerID string) string {
	return filepath.Join(LogDir(), "workers", workerID+".log")
}

// LeaderLogPath returns the path to a leader's log file.
func LeaderLogPath(leaderType string) string {
	return filepath.Join(LogDir(), "leaders", leaderType+".log")
}

// EnsureDir ensures the log directory and subdirectories exist.
func EnsureDir() error {
	dirs := []string{
		LogDir(),
		filepath.Join(LogDir(), "sessions"),
		filepath.Join(LogDir(), "workers"),
		filepath.Join(LogDir(), "leaders"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating log directory %s: %w", dir, err)
		}
	}

	return nil
}

// ListOptions configures the ListLogs function.
type ListOptions struct {
	Type      LogType   // Filter by type (empty for all)
	Before    time.Time // Filter by modification time (zero value for no filter)
	After     time.Time // Filter by modification time (zero value for no filter)
	Pattern   string    // Filter by name pattern (glob)
	SortByAge bool      // Sort by modification time (oldest first)
}

// ListLogs returns all log files matching the given options.
func ListLogs(opts ListOptions) ([]LogEntry, error) {
	var entries []LogEntry

	subdirs := map[string]LogType{
		"sessions": LogTypeSession,
		"workers":  LogTypeWorker,
		"leaders":  LogTypeLeader,
	}

	for subdir, logType := range subdirs {
		// Skip if filtering by type and doesn't match
		if opts.Type != "" && opts.Type != logType {
			continue
		}

		dir := filepath.Join(LogDir(), subdir)
		files, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", dir, err)
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".log") {
				continue
			}

			// Apply pattern filter
			if opts.Pattern != "" {
				matched, err := filepath.Match(opts.Pattern, f.Name())
				if err != nil {
					return nil, fmt.Errorf("invalid pattern %s: %w", opts.Pattern, err)
				}
				if !matched {
					continue
				}
			}

			info, err := f.Info()
			if err != nil {
				continue
			}

			// Apply time filters
			if !opts.Before.IsZero() && info.ModTime().After(opts.Before) {
				continue
			}
			if !opts.After.IsZero() && info.ModTime().Before(opts.After) {
				continue
			}

			name := strings.TrimSuffix(f.Name(), ".log")
			entry := LogEntry{
				Path:      filepath.Join(dir, f.Name()),
				Name:      name,
				Type:      logType,
				Size:      info.Size(),
				ModTime:   info.ModTime(),
				SessionID: name, // Session ID is the file name without extension
			}
			entries = append(entries, entry)
		}
	}

	// Sort entries
	if opts.SortByAge {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ModTime.Before(entries[j].ModTime)
		})
	} else {
		// Default: sort by modification time descending (newest first)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ModTime.After(entries[j].ModTime)
		})
	}

	return entries, nil
}

// CleanLogs removes log files older than the given age.
func CleanLogs(maxAge time.Duration, dryRun bool) ([]LogEntry, error) {
	cutoff := time.Now().Add(-maxAge)

	entries, err := ListLogs(ListOptions{
		Before:    cutoff,
		SortByAge: true,
	})
	if err != nil {
		return nil, err
	}

	if dryRun {
		return entries, nil
	}

	var removed []LogEntry
	for _, entry := range entries {
		if err := os.Remove(entry.Path); err != nil {
			if !os.IsNotExist(err) {
				return removed, fmt.Errorf("removing %s: %w", entry.Path, err)
			}
		}
		removed = append(removed, entry)
	}

	return removed, nil
}

// TotalSize returns the total size of all log files in bytes.
func TotalSize() (int64, error) {
	entries, err := ListLogs(ListOptions{})
	if err != nil {
		return 0, err
	}

	var total int64
	for _, e := range entries {
		total += e.Size
	}
	return total, nil
}

// CountByType returns the count of log files by type.
func CountByType() (map[LogType]int, error) {
	entries, err := ListLogs(ListOptions{})
	if err != nil {
		return nil, err
	}

	counts := make(map[LogType]int)
	for _, e := range entries {
		counts[e.Type]++
	}
	return counts, nil
}
