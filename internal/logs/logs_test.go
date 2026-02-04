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
	defer w.Close()

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
	defer w.Close()

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
	defer w.Close()

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
	defer w.Close()

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

	w.Close()

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
		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Errorf("creating gzip reader: %v", err)
			continue
		}
		defer gz.Close()

		content, err := io.ReadAll(gz)
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
	defer w.Close()

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

func TestEnsureDir(t *testing.T) {
	// This test uses the real home directory, so we just verify it doesn't error
	dir, err := EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	if dir == "" {
		t.Error("EnsureDir() returned empty path")
	}

	// Verify directory exists
	info, err := os.Stat(dir)
	if err != nil {
		t.Errorf("EnsureDir() created invalid path: %v", err)
	}

	if !info.IsDir() {
		t.Error("EnsureDir() path is not a directory")
	}
}

func TestSessionLogPath(t *testing.T) {
	path, err := SessionLogPath("test-session")
	if err != nil {
		t.Fatalf("SessionLogPath() error = %v", err)
	}

	if !strings.Contains(path, "test-session.log") {
		t.Errorf("SessionLogPath() = %q, expected to contain 'test-session.log'", path)
	}

	if !strings.Contains(path, ".forge") {
		t.Errorf("SessionLogPath() = %q, expected to contain '.forge'", path)
	}
}

func TestCompressFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content for compression")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
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
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("creating gzip reader: %v", err)
	}
	defer gz.Close()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("reading gzip content: %v", err)
	}

	if !bytes.Equal(decompressed, content) {
		t.Errorf("decompressed content = %q, want %q", decompressed, content)
	}
}

func TestCleanupOldLogs(t *testing.T) {
	// Create a temporary test directory structure
	tmpDir := t.TempDir()

	// Create some test files with different ages
	oldFile := filepath.Join(tmpDir, "old.log")
	newFile := filepath.Join(tmpDir, "new.log")

	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatalf("writing old file: %v", err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatalf("writing new file: %v", err)
	}

	// Set old file's mtime to the past
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("setting old file time: %v", err)
	}

	// Note: CleanupOldLogs uses EnsureDir() which creates ~/.forge/logs
	// This test verifies the function doesn't error, but actual cleanup
	// depends on the real logs directory
	_, err := CleanupOldLogs(24 * time.Hour)
	if err != nil {
		t.Errorf("CleanupOldLogs() error = %v", err)
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
	defer w.Close()

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
	defer w.Close()

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

func TestListLogs(t *testing.T) {
	// This test uses the real logs directory
	logs, err := ListLogs()
	if err != nil {
		t.Fatalf("ListLogs() error = %v", err)
	}

	// Just verify it returns without error
	// logs may be nil or empty if no .log files exist, which is valid
	for _, log := range logs {
		if !strings.HasSuffix(log, ".log") {
			t.Errorf("ListLogs() returned non-.log file: %s", log)
		}
	}
}
