// Package logs provides unified logging with rotation support and log file management for forge/foundry.
package logs

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Log rotation constants
const (
	// DefaultMaxSize is the default maximum size of a log file in bytes (10MB).
	DefaultMaxSize = 10 * 1024 * 1024
	// DefaultMaxBackups is the default number of rotated log files to keep.
	DefaultMaxBackups = 5
	// DefaultDirPerms is the default permissions for log directories.
	DefaultDirPerms = 0o755
	// DefaultFilePerms is the default permissions for log files.
	DefaultFilePerms = 0o644
)

// ErrRotationFailed indicates that log rotation failed.
var ErrRotationFailed = errors.New("log rotation failed")

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

// Options configures log rotation behavior.
type Options struct {
	// MaxSize is the maximum size of a log file in bytes before rotation.
	// Default is 10MB.
	MaxSize int64
	// MaxBackups is the maximum number of old log files to retain.
	// Default is 5.
	MaxBackups int
	// Compress determines if rotated files should be gzipped.
	// Default is true.
	Compress bool
}

// DefaultOptions returns the default rotation options.
func DefaultOptions() Options {
	return Options{
		MaxSize:    DefaultMaxSize,
		MaxBackups: DefaultMaxBackups,
		Compress:   true,
	}
}

// RotatingWriter wraps a file and automatically rotates it when size is exceeded.
type RotatingWriter struct {
	path    string
	opts    Options
	mu      sync.Mutex
	file    *os.File
	size    int64
	created time.Time
}

// NewRotatingWriter creates a new rotating writer for the given path.
func NewRotatingWriter(path string, opts Options) (*RotatingWriter, error) {
	// Apply defaults for zero values
	if opts.MaxSize <= 0 {
		opts.MaxSize = DefaultMaxSize
	}
	if opts.MaxBackups <= 0 {
		opts.MaxBackups = DefaultMaxBackups
	}

	w := &RotatingWriter{
		path: path,
		opts: opts,
	}

	if err := w.openFile(); err != nil {
		return nil, err
	}

	return w, nil
}

// Write implements io.Writer. It automatically rotates the log file if needed.
func (w *RotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if rotation is needed before writing
	if w.size+int64(len(p)) > w.opts.MaxSize {
		if err := w.rotate(); err != nil {
			// Log rotation failed, but we still try to write
			fmt.Fprintf(os.Stderr, "log rotation failed: %v\n", err)
		}
	}

	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// Close closes the underlying file.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// Rotate forces a log rotation.
func (w *RotatingWriter) Rotate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotate()
}

// Path returns the current log file path.
func (w *RotatingWriter) Path() string {
	return w.path
}

// Size returns the current size of the log file.
func (w *RotatingWriter) Size() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size
}

// openFile opens or creates the log file.
func (w *RotatingWriter) openFile() error {
	// Ensure directory exists
	dir := filepath.Dir(w.path)
	if err := os.MkdirAll(dir, DefaultDirPerms); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, DefaultFilePerms)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}

	// Get current size
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("getting file info: %w", err)
	}

	w.file = file
	w.size = info.Size()
	w.created = info.ModTime()

	return nil
}

// rotate performs the log rotation.
func (w *RotatingWriter) rotate() error {
	// Close current file
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("closing current log: %w", err)
		}
	}

	// Generate rotated filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := fmt.Sprintf("%s.%s", w.path, timestamp)

	// Rename current log to rotated name
	if err := os.Rename(w.path, rotatedPath); err != nil {
		// If file doesn't exist, just open a new one
		if !os.IsNotExist(err) {
			return fmt.Errorf("renaming log file: %w", err)
		}
	} else {
		// Compress if enabled
		if w.opts.Compress {
			go func(path string) {
				if err := compressFile(path); err != nil {
					fmt.Fprintf(os.Stderr, "failed to compress %s: %v\n", path, err)
				}
			}(rotatedPath)
		}
	}

	// Clean up old backups
	if err := w.cleanupOldBackups(); err != nil {
		// Non-fatal, log and continue
		fmt.Fprintf(os.Stderr, "failed to cleanup old backups: %v\n", err)
	}

	// Open new file
	return w.openFile()
}

// cleanupOldBackups removes old backup files beyond MaxBackups.
func (w *RotatingWriter) cleanupOldBackups() error {
	dir := filepath.Dir(w.path)
	base := filepath.Base(w.path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading log directory: %w", err)
	}

	// Find all backup files for this log
	var backups []string
	for _, entry := range entries {
		name := entry.Name()
		// Match rotated files: base.timestamp or base.timestamp.gz
		if strings.HasPrefix(name, base+".") && name != base {
			backups = append(backups, filepath.Join(dir, name))
		}
	}

	// Sort by name (which includes timestamp) - oldest first
	sort.Strings(backups)

	// Remove oldest backups if we have too many
	for len(backups) > w.opts.MaxBackups {
		oldest := backups[0]
		if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing old backup %s: %w", oldest, err)
		}
		backups = backups[1:]
	}

	return nil
}

// compressFile compresses a file with gzip and removes the original.
func compressFile(path string) error {
	// Open source file
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}

	// Create destination file
	dstPath := path + ".gz"
	dst, err := os.Create(dstPath)
	if err != nil {
		_ = src.Close()
		return fmt.Errorf("creating gzip file: %w", err)
	}

	// Create gzip writer
	gz := gzip.NewWriter(dst)
	gz.Name = filepath.Base(path)
	gz.ModTime = time.Now()

	// Copy contents
	if _, err := io.Copy(gz, src); err != nil {
		_ = gz.Close()
		_ = dst.Close()
		_ = src.Close()
		_ = os.Remove(dstPath)
		return fmt.Errorf("compressing file: %w", err)
	}

	// Close gzip writer
	if err := gz.Close(); err != nil {
		_ = dst.Close()
		_ = src.Close()
		_ = os.Remove(dstPath)
		return fmt.Errorf("closing gzip writer: %w", err)
	}

	// Close destination file
	if err := dst.Close(); err != nil {
		_ = src.Close()
		_ = os.Remove(dstPath)
		return fmt.Errorf("closing destination file: %w", err)
	}

	// Close source before removing (required on Windows)
	if err := src.Close(); err != nil {
		return fmt.Errorf("closing source file: %w", err)
	}

	// Remove original file
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing original file: %w", err)
	}

	return nil
}

// LogDir returns the default directory for forge logs.
// This is a variable so it can be overridden in tests.
var LogDir = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "logs")
}

// EnsureDir ensures the log directory and subdirectories exist.
// Returns the path to the logs directory.
func EnsureDir() (string, error) {
	dirs := []string{
		LogDir(),
		filepath.Join(LogDir(), "sessions"),
		filepath.Join(LogDir(), "workers"),
		filepath.Join(LogDir(), "leaders"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, DefaultDirPerms); err != nil {
			return "", fmt.Errorf("creating log directory %s: %w", dir, err)
		}
	}

	return LogDir(), nil
}

// SessionLogPath returns the path to a session's log file.
func SessionLogPath(sessionID string) (string, error) {
	logsDir, err := EnsureDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(logsDir, "sessions", sessionID+".log"), nil
}

// WorkerLogPath returns the path to a worker's log file.
func WorkerLogPath(workerID string) string {
	return filepath.Join(LogDir(), "workers", workerID+".log")
}

// LeaderLogPath returns the path to a leader's log file.
func LeaderLogPath(leaderType string) string {
	return filepath.Join(LogDir(), "leaders", leaderType+".log")
}

// NewSessionWriter creates a new rotating writer for a session.
func NewSessionWriter(sessionID string, opts Options) (*RotatingWriter, error) {
	path, err := SessionLogPath(sessionID)
	if err != nil {
		return nil, err
	}
	return NewRotatingWriter(path, opts)
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

// CleanupOldLogs removes log files older than the specified duration.
// Returns the count of removed files.
func CleanupOldLogs(maxAge time.Duration) (int, error) {
	removed, err := CleanLogs(maxAge, false)
	if err != nil {
		return 0, err
	}
	return len(removed), nil
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
