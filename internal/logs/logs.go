// Package logs provides unified logging with rotation support for forge/foundry.
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

const (
	// DefaultMaxSize is the default maximum size of a log file in bytes (10MB).
	DefaultMaxSize = 10 * 1024 * 1024
	// DefaultMaxBackups is the default number of rotated log files to keep.
	DefaultMaxBackups = 5
	// DefaultDirPerms is the default permissions for log directories.
	DefaultDirPerms = 0755
	// DefaultFilePerms is the default permissions for log files.
	DefaultFilePerms = 0644
)

var (
	// ErrRotationFailed indicates that log rotation failed.
	ErrRotationFailed = errors.New("log rotation failed")
)

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
		file.Close()
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
	defer src.Close()

	// Create destination file
	dstPath := path + ".gz"
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("creating gzip file: %w", err)
	}
	defer dst.Close()

	// Create gzip writer
	gz := gzip.NewWriter(dst)
	gz.Name = filepath.Base(path)
	gz.ModTime = time.Now()

	// Copy contents
	if _, err := io.Copy(gz, src); err != nil {
		os.Remove(dstPath)
		return fmt.Errorf("compressing file: %w", err)
	}

	// Close gzip writer
	if err := gz.Close(); err != nil {
		os.Remove(dstPath)
		return fmt.Errorf("closing gzip writer: %w", err)
	}

	// Remove original file
	src.Close() // Close before removing on Windows
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing original file: %w", err)
	}

	return nil
}

// EnsureDir creates the default logs directory if it doesn't exist.
// Returns the path to the logs directory.
func EnsureDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	logsDir := filepath.Join(home, ".forge", "logs")
	if err := os.MkdirAll(logsDir, DefaultDirPerms); err != nil {
		return "", fmt.Errorf("creating logs directory: %w", err)
	}

	return logsDir, nil
}

// SessionLogPath returns the path for a session's log file.
func SessionLogPath(sessionID string) (string, error) {
	logsDir, err := EnsureDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(logsDir, sessionID+".log"), nil
}

// NewSessionWriter creates a new rotating writer for a session.
func NewSessionWriter(sessionID string, opts Options) (*RotatingWriter, error) {
	path, err := SessionLogPath(sessionID)
	if err != nil {
		return nil, err
	}
	return NewRotatingWriter(path, opts)
}

// ListLogs returns all log files in the logs directory.
func ListLogs() ([]string, error) {
	logsDir, err := EnsureDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return nil, fmt.Errorf("reading logs directory: %w", err)
	}

	var logs []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			logs = append(logs, filepath.Join(logsDir, entry.Name()))
		}
	}

	return logs, nil
}

// CleanupOldLogs removes log files older than the specified duration.
func CleanupOldLogs(maxAge time.Duration) (int, error) {
	logsDir, err := EnsureDir()
	if err != nil {
		return 0, err
	}

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return 0, fmt.Errorf("reading logs directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(logsDir, entry.Name())
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}
