package logs

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Log Rotation Tests (from task/2gr)
// ============================================================================

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.MaxSize != DefaultMaxSize {
		t.Errorf("MaxSize = %d, want %d", opts.MaxSize, DefaultMaxSize)
	}
	if opts.MaxBackups != DefaultMaxBackups {
		t.Errorf("MaxBackups = %d, want %d", opts.MaxBackups, DefaultMaxBackups)
	}
	if !opts.Compress {
		t.Error("Compress = false, want true")
	}
}

func TestNewRotatingWriter(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	w, err := NewRotatingWriter(logPath, DefaultOptions())
	if err != nil {
		t.Fatalf("NewRotatingWriter() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	if w.Path() != logPath {
		t.Errorf("Path() = %q, want %q", w.Path(), logPath)
	}

	if w.Size() != 0 {
		t.Errorf("Size() = %d, want 0", w.Size())
	}

	// Verify file was created
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

func TestRotatingWriter_Write(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	w, err := NewRotatingWriter(logPath, DefaultOptions())
	if err != nil {
		t.Fatalf("NewRotatingWriter() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	data := []byte("hello world\n")
	n, err := w.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() = %d, want %d", n, len(data))
	}

	if w.Size() != int64(len(data)) {
		t.Errorf("Size() = %d, want %d", w.Size(), len(data))
	}

	// Verify content was written
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("file content = %q, want %q", string(content), string(data))
	}
}

func TestRotatingWriter_RotatesOnMaxSize(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	opts := Options{
		MaxSize:    100, // Very small for testing
		MaxBackups: 3,
		Compress:   false, // Disable compression for easier verification
	}

	w, err := NewRotatingWriter(logPath, opts)
	if err != nil {
		t.Fatalf("NewRotatingWriter() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	// Write enough data to trigger rotation
	data := bytes.Repeat([]byte("x"), 60)
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// This should trigger rotation
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Wait a moment for any async operations
	time.Sleep(100 * time.Millisecond)

	// Check that rotation occurred
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}

	var logFiles []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "test.log") {
			logFiles = append(logFiles, e.Name())
		}
	}

	// Should have current log plus at least one rotated file
	if len(logFiles) < 2 {
		t.Errorf("expected at least 2 log files after rotation, got %d: %v", len(logFiles), logFiles)
	}
}

func TestRotatingWriter_ManualRotate(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	opts := Options{
		MaxSize:    DefaultMaxSize,
		MaxBackups: 3,
		Compress:   false,
	}

	w, err := NewRotatingWriter(logPath, opts)
	if err != nil {
		t.Fatalf("NewRotatingWriter() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	// Write some data
	if _, err := w.Write([]byte("test data\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Force rotation
	if err := w.Rotate(); err != nil {
		t.Errorf("Rotate() error = %v", err)
	}

	// Size should be reset
	if w.Size() != 0 {
		t.Errorf("Size() = %d after rotation, want 0", w.Size())
	}

	// Check rotated file exists
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}

	rotatedCount := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "test.log.") {
			rotatedCount++
		}
	}

	if rotatedCount != 1 {
		t.Errorf("expected 1 rotated file, got %d", rotatedCount)
	}
}

func TestRotatingWriter_CompressesRotatedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	opts := Options{
		MaxSize:    100,
		MaxBackups: 3,
		Compress:   true,
	}

	w, err := NewRotatingWriter(logPath, opts)
	if err != nil {
		t.Fatalf("NewRotatingWriter() error = %v", err)
	}

	// Write data to trigger rotation
	data := bytes.Repeat([]byte("x"), 60)
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	_ = w.Close()

	// Wait for compression to complete (runs async)
	time.Sleep(500 * time.Millisecond)

	// Check for .gz file
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}

	var gzFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gz") {
			gzFiles = append(gzFiles, filepath.Join(tmpDir, e.Name()))
		}
	}

	if len(gzFiles) == 0 {
		t.Error("expected at least one .gz file after rotation with compression")
		return
	}

	// Verify the gzip file is valid
	for _, gzPath := range gzFiles {
		f, err := os.Open(gzPath)
		if err != nil {
			t.Errorf("opening gzip file: %v", err)
			continue
		}

		gz, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			t.Errorf("creating gzip reader: %v", err)
			continue
		}

		content, err := io.ReadAll(gz)
		_ = gz.Close()
		_ = f.Close()
		if err != nil {
			t.Errorf("reading gzip content: %v", err)
			continue
		}

		if len(content) == 0 {
			t.Error("compressed file has no content")
		}
	}
}

func TestRotatingWriter_CleansUpOldBackups(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	opts := Options{
		MaxSize:    50,
		MaxBackups: 2, // Only keep 2 backups
		Compress:   false,
	}

	w, err := NewRotatingWriter(logPath, opts)
	if err != nil {
		t.Fatalf("NewRotatingWriter() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	// Trigger multiple rotations
	data := bytes.Repeat([]byte("x"), 30)
	for i := 0; i < 5; i++ {
		if _, err := w.Write(data); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		// Small delay to ensure unique timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for cleanup to complete
	time.Sleep(100 * time.Millisecond)

	// Count backup files
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}

	backupCount := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "test.log.") {
			backupCount++
		}
	}

	if backupCount > opts.MaxBackups {
		t.Errorf("expected at most %d backups, got %d", opts.MaxBackups, backupCount)
	}
}

func TestRotatingWriter_ZeroOptions(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Zero options should use defaults
	w, err := NewRotatingWriter(logPath, Options{})
	if err != nil {
		t.Fatalf("NewRotatingWriter() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	// Verify defaults were applied
	if w.opts.MaxSize != DefaultMaxSize {
		t.Errorf("opts.MaxSize = %d, want %d", w.opts.MaxSize, DefaultMaxSize)
	}
	if w.opts.MaxBackups != DefaultMaxBackups {
		t.Errorf("opts.MaxBackups = %d, want %d", w.opts.MaxBackups, DefaultMaxBackups)
	}
}

func TestRotatingWriter_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	w, err := NewRotatingWriter(logPath, DefaultOptions())
	if err != nil {
		t.Fatalf("NewRotatingWriter() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	// Concurrent writes should be safe
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				_, _ = w.Write([]byte("concurrent write\n"))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify file exists and has content
	info, err := os.Stat(logPath)
	if err != nil {
		t.Errorf("log file not found after concurrent writes: %v", err)
	}

	if info.Size() == 0 {
		t.Error("log file is empty after concurrent writes")
	}
}

func TestCompressFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content for compression")
	if err := os.WriteFile(srcPath, content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := compressFile(srcPath); err != nil {
		t.Fatalf("compressFile() error = %v", err)
	}

	// Original should be removed
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("original file should be removed after compression")
	}

	// Compressed file should exist
	gzPath := srcPath + ".gz"
	if _, err := os.Stat(gzPath); err != nil {
		t.Errorf("compressed file not found: %v", err)
	}

	// Verify content
	f, err := os.Open(gzPath)
	if err != nil {
		t.Fatalf("opening gzip file: %v", err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("creating gzip reader: %v", err)
	}
	defer func() { _ = gz.Close() }()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("reading gzip content: %v", err)
	}

	if !bytes.Equal(decompressed, content) {
		t.Errorf("decompressed content = %q, want %q", decompressed, content)
	}
}

// ============================================================================
// Log Management Tests (from task/syh)
// ============================================================================

// setupTestDir creates a temporary directory for testing.
func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "forge-logs-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// setupTestLogs creates a temporary log directory with test log files.
// Returns the temp directory path, a cleanup function, and sets up the directory structure.
func setupTestLogs(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupTestDir(t)

	// Create log directory structure
	dirs := []string{
		filepath.Join(tmpDir, "sessions"),
		filepath.Join(tmpDir, "workers"),
		filepath.Join(tmpDir, "leaders"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			cleanup()
			t.Fatalf("creating dir %s: %v", dir, err)
		}
	}

	return tmpDir, cleanup
}

// createTestLog creates a test log file with the given content and modification time.
func createTestLog(t *testing.T, path string, content string, modTime time.Time) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test log %s: %v", path, err)
	}
	if !modTime.IsZero() {
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("setting mod time for %s: %v", path, err)
		}
	}
}

// TestLogDir tests the LogDir function.
func TestLogDir(t *testing.T) {
	dir := LogDir()
	if dir == "" {
		t.Error("LogDir() returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("LogDir() returned non-absolute path: %s", dir)
	}
	if !strings.Contains(dir, ".forge") {
		t.Errorf("LogDir() should contain .forge: %s", dir)
	}
	if !strings.Contains(dir, "logs") {
		t.Errorf("LogDir() should contain logs: %s", dir)
	}
}

// TestSessionLogPath tests the SessionLogPath function.
func TestSessionLogPath(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wantPart  string
	}{
		{"simple id", "abc123", "sessions/abc123.log"},
		{"with dashes", "session-abc-123", "sessions/session-abc-123.log"},
		{"uuid format", "550e8400-e29b-41d4-a716-446655440000", "sessions/550e8400-e29b-41d4-a716-446655440000.log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SessionLogPath(tt.sessionID)
			if err != nil {
				t.Fatalf("SessionLogPath() error = %v", err)
			}
			if !strings.Contains(got, tt.wantPart) {
				t.Errorf("SessionLogPath(%q) = %q, want to contain %q", tt.sessionID, got, tt.wantPart)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("SessionLogPath(%q) returned non-absolute path: %s", tt.sessionID, got)
			}
		})
	}
}

// TestWorkerLogPath tests the WorkerLogPath function.
func TestWorkerLogPath(t *testing.T) {
	tests := []struct {
		name     string
		workerID string
		wantPart string
	}{
		{"simple id", "alpha", "workers/alpha.log"},
		{"with prefix", "w-abc12345", "workers/w-abc12345.log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorkerLogPath(tt.workerID)
			if !strings.Contains(got, tt.wantPart) {
				t.Errorf("WorkerLogPath(%q) = %q, want to contain %q", tt.workerID, got, tt.wantPart)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("WorkerLogPath(%q) returned non-absolute path: %s", tt.workerID, got)
			}
		})
	}
}

// TestLeaderLogPath tests the LeaderLogPath function.
func TestLeaderLogPath(t *testing.T) {
	tests := []struct {
		name       string
		leaderType string
		wantPart   string
	}{
		{"planner", "planner", "leaders/planner.log"},
		{"reviewer", "reviewer", "leaders/reviewer.log"},
		{"merge", "merge", "leaders/merge.log"},
		{"deploy", "deploy", "leaders/deploy.log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LeaderLogPath(tt.leaderType)
			if !strings.Contains(got, tt.wantPart) {
				t.Errorf("LeaderLogPath(%q) = %q, want to contain %q", tt.leaderType, got, tt.wantPart)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("LeaderLogPath(%q) returned non-absolute path: %s", tt.leaderType, got)
			}
		})
	}
}

// TestEnsureDir tests the EnsureDir function.
func TestEnsureDir(t *testing.T) {
	// Save original LogDir and restore after test
	origLogDir := LogDir

	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Override LogDir to use temp directory
	testLogDir := filepath.Join(tmpDir, "forge-logs")
	LogDir = func() string { return testLogDir }
	defer func() { LogDir = origLogDir }()

	// Verify directories don't exist yet
	if _, err := os.Stat(testLogDir); !os.IsNotExist(err) {
		t.Fatal("test log dir should not exist before EnsureDir")
	}

	// Call EnsureDir
	dir, err := EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}
	if dir != testLogDir {
		t.Errorf("EnsureDir() = %q, want %q", dir, testLogDir)
	}

	// Verify all directories were created
	expectedDirs := []string{
		testLogDir,
		filepath.Join(testLogDir, "sessions"),
		filepath.Join(testLogDir, "workers"),
		filepath.Join(testLogDir, "leaders"),
	}

	for _, d := range expectedDirs {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("EnsureDir() failed to create %s: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("EnsureDir() %s is not a directory", d)
		}
	}

	// Calling EnsureDir again should not fail (idempotent)
	if _, err := EnsureDir(); err != nil {
		t.Errorf("EnsureDir() second call error = %v", err)
	}
}

// TestListLogs tests the ListLogs function with various options.
func TestListLogs(t *testing.T) {
	// Save original LogDir and restore after test
	origLogDir := LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	// Override LogDir to use temp directory
	LogDir = func() string { return tmpDir }
	defer func() { LogDir = origLogDir }()

	// Create test log files
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	recentTime := now.Add(-1 * time.Hour)

	createTestLog(t, filepath.Join(tmpDir, "sessions", "session1.log"), "session1 content", recentTime)
	createTestLog(t, filepath.Join(tmpDir, "sessions", "session2.log"), "session2 content", oldTime)
	createTestLog(t, filepath.Join(tmpDir, "workers", "alpha.log"), "alpha worker log", recentTime)
	createTestLog(t, filepath.Join(tmpDir, "workers", "bravo.log"), "bravo worker log", oldTime)
	createTestLog(t, filepath.Join(tmpDir, "leaders", "planner.log"), "planner log", recentTime)
	createTestLog(t, filepath.Join(tmpDir, "leaders", "reviewer.log"), "reviewer log", oldTime)

	// Create a non-log file (should be ignored)
	createTestLog(t, filepath.Join(tmpDir, "sessions", "readme.txt"), "not a log", now)

	t.Run("list all logs", func(t *testing.T) {
		entries, err := ListLogs(ListOptions{})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		if len(entries) != 6 {
			t.Errorf("ListLogs() returned %d entries, want 6", len(entries))
		}
	})

	t.Run("filter by session type", func(t *testing.T) {
		entries, err := ListLogs(ListOptions{Type: LogTypeSession})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("ListLogs(session) returned %d entries, want 2", len(entries))
		}
		for _, e := range entries {
			if e.Type != LogTypeSession {
				t.Errorf("entry type = %s, want session", e.Type)
			}
		}
	})

	t.Run("filter by worker type", func(t *testing.T) {
		entries, err := ListLogs(ListOptions{Type: LogTypeWorker})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("ListLogs(worker) returned %d entries, want 2", len(entries))
		}
		for _, e := range entries {
			if e.Type != LogTypeWorker {
				t.Errorf("entry type = %s, want worker", e.Type)
			}
		}
	})

	t.Run("filter by leader type", func(t *testing.T) {
		entries, err := ListLogs(ListOptions{Type: LogTypeLeader})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("ListLogs(leader) returned %d entries, want 2", len(entries))
		}
		for _, e := range entries {
			if e.Type != LogTypeLeader {
				t.Errorf("entry type = %s, want leader", e.Type)
			}
		}
	})

	t.Run("filter by before time", func(t *testing.T) {
		cutoff := now.Add(-24 * time.Hour)
		entries, err := ListLogs(ListOptions{Before: cutoff})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		// Should only get the old logs (3 files)
		if len(entries) != 3 {
			t.Errorf("ListLogs(before) returned %d entries, want 3", len(entries))
		}
		for _, e := range entries {
			if e.ModTime.After(cutoff) {
				t.Errorf("entry mod time %v is after cutoff %v", e.ModTime, cutoff)
			}
		}
	})

	t.Run("filter by after time", func(t *testing.T) {
		cutoff := now.Add(-24 * time.Hour)
		entries, err := ListLogs(ListOptions{After: cutoff})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		// Should only get the recent logs (3 files)
		if len(entries) != 3 {
			t.Errorf("ListLogs(after) returned %d entries, want 3", len(entries))
		}
		for _, e := range entries {
			if e.ModTime.Before(cutoff) {
				t.Errorf("entry mod time %v is before cutoff %v", e.ModTime, cutoff)
			}
		}
	})

	t.Run("filter by pattern", func(t *testing.T) {
		entries, err := ListLogs(ListOptions{Pattern: "alpha*.log"})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("ListLogs(pattern alpha*) returned %d entries, want 1", len(entries))
		}
		if len(entries) > 0 && entries[0].Name != "alpha" {
			t.Errorf("entry name = %s, want alpha", entries[0].Name)
		}
	})

	t.Run("pattern with wildcard", func(t *testing.T) {
		entries, err := ListLogs(ListOptions{Pattern: "session*.log"})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("ListLogs(pattern session*) returned %d entries, want 2", len(entries))
		}
	})

	t.Run("invalid pattern", func(t *testing.T) {
		_, err := ListLogs(ListOptions{Pattern: "[invalid"})
		if err == nil {
			t.Error("ListLogs() should return error for invalid pattern")
		}
	})

	t.Run("sort by age oldest first", func(t *testing.T) {
		entries, err := ListLogs(ListOptions{SortByAge: true})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		if len(entries) < 2 {
			t.Skip("not enough entries to test sorting")
		}
		// First entry should be older than last
		if !entries[0].ModTime.Before(entries[len(entries)-1].ModTime) {
			t.Error("entries should be sorted oldest first when SortByAge=true")
		}
	})

	t.Run("default sort newest first", func(t *testing.T) {
		entries, err := ListLogs(ListOptions{SortByAge: false})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		if len(entries) < 2 {
			t.Skip("not enough entries to test sorting")
		}
		// First entry should be newer than last
		if !entries[0].ModTime.After(entries[len(entries)-1].ModTime) {
			t.Error("entries should be sorted newest first by default")
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		cutoff := now.Add(-24 * time.Hour)
		entries, err := ListLogs(ListOptions{
			Type:   LogTypeWorker,
			Before: cutoff,
		})
		if err != nil {
			t.Fatalf("ListLogs() error = %v", err)
		}
		// Should only get old worker logs (1 file: bravo)
		if len(entries) != 1 {
			t.Errorf("ListLogs(worker, before) returned %d entries, want 1", len(entries))
		}
	})
}

// TestListLogsEmptyDirectory tests ListLogs when directories don't exist.
func TestListLogsEmptyDirectory(t *testing.T) {
	origLogDir := LogDir

	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Point to a non-existent directory
	LogDir = func() string { return filepath.Join(tmpDir, "nonexistent") }
	defer func() { LogDir = origLogDir }()

	entries, err := ListLogs(ListOptions{})
	if err != nil {
		t.Fatalf("ListLogs() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("ListLogs() returned %d entries, want 0 for non-existent dir", len(entries))
	}
}

// TestCleanLogs tests the CleanLogs function.
func TestCleanLogs(t *testing.T) {
	origLogDir := LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	LogDir = func() string { return tmpDir }
	defer func() { LogDir = origLogDir }()

	// Create test logs with different ages
	now := time.Now()
	oldTime := now.Add(-10 * 24 * time.Hour)   // 10 days old
	recentTime := now.Add(-1 * 24 * time.Hour) // 1 day old

	createTestLog(t, filepath.Join(tmpDir, "sessions", "old-session.log"), "old content", oldTime)
	createTestLog(t, filepath.Join(tmpDir, "sessions", "recent-session.log"), "recent content", recentTime)
	createTestLog(t, filepath.Join(tmpDir, "workers", "old-worker.log"), "old worker", oldTime)
	createTestLog(t, filepath.Join(tmpDir, "workers", "recent-worker.log"), "recent worker", recentTime)

	t.Run("dry run", func(t *testing.T) {
		// Clean logs older than 7 days (dry run)
		maxAge := 7 * 24 * time.Hour
		removed, err := CleanLogs(maxAge, true)
		if err != nil {
			t.Fatalf("CleanLogs() error = %v", err)
		}

		// Should identify 2 old files
		if len(removed) != 2 {
			t.Errorf("CleanLogs() dry run returned %d entries, want 2", len(removed))
		}

		// Verify files still exist (dry run shouldn't delete)
		if _, err := os.Stat(filepath.Join(tmpDir, "sessions", "old-session.log")); os.IsNotExist(err) {
			t.Error("CleanLogs() dry run should not delete files")
		}
	})

	t.Run("actual cleanup", func(t *testing.T) {
		maxAge := 7 * 24 * time.Hour
		removed, err := CleanLogs(maxAge, false)
		if err != nil {
			t.Fatalf("CleanLogs() error = %v", err)
		}

		// Should have removed 2 old files
		if len(removed) != 2 {
			t.Errorf("CleanLogs() returned %d entries, want 2", len(removed))
		}

		// Verify old files are deleted
		if _, err := os.Stat(filepath.Join(tmpDir, "sessions", "old-session.log")); !os.IsNotExist(err) {
			t.Error("CleanLogs() should have deleted old-session.log")
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "workers", "old-worker.log")); !os.IsNotExist(err) {
			t.Error("CleanLogs() should have deleted old-worker.log")
		}

		// Verify recent files still exist
		if _, err := os.Stat(filepath.Join(tmpDir, "sessions", "recent-session.log")); os.IsNotExist(err) {
			t.Error("CleanLogs() should not delete recent-session.log")
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "workers", "recent-worker.log")); os.IsNotExist(err) {
			t.Error("CleanLogs() should not delete recent-worker.log")
		}
	})
}

// TestCleanLogsNoOldFiles tests CleanLogs when no files are old enough.
func TestCleanLogsNoOldFiles(t *testing.T) {
	origLogDir := LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	LogDir = func() string { return tmpDir }
	defer func() { LogDir = origLogDir }()

	// Create only recent logs
	recentTime := time.Now().Add(-1 * time.Hour)
	createTestLog(t, filepath.Join(tmpDir, "sessions", "recent.log"), "content", recentTime)

	removed, err := CleanLogs(24*time.Hour, false)
	if err != nil {
		t.Fatalf("CleanLogs() error = %v", err)
	}

	if len(removed) != 0 {
		t.Errorf("CleanLogs() returned %d entries, want 0 (no old files)", len(removed))
	}
}

// TestCleanupOldLogs tests the CleanupOldLogs compatibility function.
func TestCleanupOldLogs(t *testing.T) {
	origLogDir := LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	LogDir = func() string { return tmpDir }
	defer func() { LogDir = origLogDir }()

	// Create test files with different ages
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)

	createTestLog(t, filepath.Join(tmpDir, "sessions", "old.log"), "old", oldTime)
	createTestLog(t, filepath.Join(tmpDir, "sessions", "new.log"), "new", now)

	count, err := CleanupOldLogs(24 * time.Hour)
	if err != nil {
		t.Errorf("CleanupOldLogs() error = %v", err)
	}
	if count != 1 {
		t.Errorf("CleanupOldLogs() = %d, want 1", count)
	}
}

// TestTotalSize tests the TotalSize function.
func TestTotalSize(t *testing.T) {
	origLogDir := LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	LogDir = func() string { return tmpDir }
	defer func() { LogDir = origLogDir }()

	t.Run("empty directory", func(t *testing.T) {
		size, err := TotalSize()
		if err != nil {
			t.Fatalf("TotalSize() error = %v", err)
		}
		if size != 0 {
			t.Errorf("TotalSize() = %d, want 0 for empty directory", size)
		}
	})

	// Create test logs
	content1 := "1234567890" // 10 bytes
	content2 := "abcdefghij" // 10 bytes
	createTestLog(t, filepath.Join(tmpDir, "sessions", "test1.log"), content1, time.Time{})
	createTestLog(t, filepath.Join(tmpDir, "workers", "test2.log"), content2, time.Time{})

	t.Run("with logs", func(t *testing.T) {
		size, err := TotalSize()
		if err != nil {
			t.Fatalf("TotalSize() error = %v", err)
		}
		if size != 20 {
			t.Errorf("TotalSize() = %d, want 20", size)
		}
	})
}

// TestCountByType tests the CountByType function.
func TestCountByType(t *testing.T) {
	origLogDir := LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	LogDir = func() string { return tmpDir }
	defer func() { LogDir = origLogDir }()

	t.Run("empty directory", func(t *testing.T) {
		counts, err := CountByType()
		if err != nil {
			t.Fatalf("CountByType() error = %v", err)
		}
		if len(counts) != 0 {
			t.Errorf("CountByType() should return empty map for empty directory")
		}
	})

	// Create test logs
	createTestLog(t, filepath.Join(tmpDir, "sessions", "s1.log"), "s1", time.Time{})
	createTestLog(t, filepath.Join(tmpDir, "sessions", "s2.log"), "s2", time.Time{})
	createTestLog(t, filepath.Join(tmpDir, "sessions", "s3.log"), "s3", time.Time{})
	createTestLog(t, filepath.Join(tmpDir, "workers", "w1.log"), "w1", time.Time{})
	createTestLog(t, filepath.Join(tmpDir, "workers", "w2.log"), "w2", time.Time{})
	createTestLog(t, filepath.Join(tmpDir, "leaders", "l1.log"), "l1", time.Time{})

	t.Run("with logs", func(t *testing.T) {
		counts, err := CountByType()
		if err != nil {
			t.Fatalf("CountByType() error = %v", err)
		}

		if counts[LogTypeSession] != 3 {
			t.Errorf("CountByType()[session] = %d, want 3", counts[LogTypeSession])
		}
		if counts[LogTypeWorker] != 2 {
			t.Errorf("CountByType()[worker] = %d, want 2", counts[LogTypeWorker])
		}
		if counts[LogTypeLeader] != 1 {
			t.Errorf("CountByType()[leader] = %d, want 1", counts[LogTypeLeader])
		}
	})
}

// TestLogEntry tests the LogEntry struct fields.
func TestLogEntry(t *testing.T) {
	origLogDir := LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	LogDir = func() string { return tmpDir }
	defer func() { LogDir = origLogDir }()

	content := "test log content"
	logPath := filepath.Join(tmpDir, "sessions", "test-session.log")
	modTime := time.Now().Add(-1 * time.Hour)
	createTestLog(t, logPath, content, modTime)

	entries, err := ListLogs(ListOptions{Type: LogTypeSession})
	if err != nil {
		t.Fatalf("ListLogs() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]

	if entry.Path != logPath {
		t.Errorf("entry.Path = %s, want %s", entry.Path, logPath)
	}
	if entry.Name != "test-session" {
		t.Errorf("entry.Name = %s, want test-session", entry.Name)
	}
	if entry.Type != LogTypeSession {
		t.Errorf("entry.Type = %s, want session", entry.Type)
	}
	if entry.Size != int64(len(content)) {
		t.Errorf("entry.Size = %d, want %d", entry.Size, len(content))
	}
	if entry.SessionID != "test-session" {
		t.Errorf("entry.SessionID = %s, want test-session", entry.SessionID)
	}
}

// TestLogTypeConstants tests the LogType constant values.
func TestLogTypeConstants(t *testing.T) {
	if LogTypeSession != "session" {
		t.Errorf("LogTypeSession = %s, want session", LogTypeSession)
	}
	if LogTypeWorker != "worker" {
		t.Errorf("LogTypeWorker = %s, want worker", LogTypeWorker)
	}
	if LogTypeLeader != "leader" {
		t.Errorf("LogTypeLeader = %s, want leader", LogTypeLeader)
	}
}

// TestListLogsIgnoresDirectories tests that ListLogs ignores subdirectories.
func TestListLogsIgnoresDirectories(t *testing.T) {
	origLogDir := LogDir

	tmpDir, cleanup := setupTestLogs(t)
	defer cleanup()

	LogDir = func() string { return tmpDir }
	defer func() { LogDir = origLogDir }()

	// Create a log file
	createTestLog(t, filepath.Join(tmpDir, "sessions", "real.log"), "content", time.Time{})

	// Create a subdirectory (should be ignored)
	subdir := filepath.Join(tmpDir, "sessions", "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	entries, err := ListLogs(ListOptions{Type: LogTypeSession})
	if err != nil {
		t.Fatalf("ListLogs() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("ListLogs() returned %d entries, want 1 (should ignore subdirectory)", len(entries))
	}
}
