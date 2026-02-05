package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewLogCmd tests that the log command is created correctly.
func TestNewLogCmd(t *testing.T) {
	cmd := newLogCmd()

	if cmd == nil {
		t.Fatal("newLogCmd() returned nil")
	}

	if cmd.Use != "log [session-id]" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "log [session-id]")
	}

	if cmd.Short == "" {
		t.Error("cmd.Short should not be empty")
	}

	if cmd.Long == "" {
		t.Error("cmd.Long should not be empty")
	}
}

// TestLogCmdFlags tests that all expected flags are defined.
func TestLogCmdFlags(t *testing.T) {
	cmd := newLogCmd()

	expectedFlags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"follow", "f", "false"},
		{"lines", "n", "20"},
	}

	for _, ef := range expectedFlags {
		flag := cmd.Flags().Lookup(ef.name)
		if flag == nil {
			t.Errorf("flag %q should exist", ef.name)
			continue
		}

		if flag.Shorthand != ef.shorthand {
			t.Errorf("flag %q shorthand = %q, want %q", ef.name, flag.Shorthand, ef.shorthand)
		}

		if flag.DefValue != ef.defValue {
			t.Errorf("flag %q default = %q, want %q", ef.name, flag.DefValue, ef.defValue)
		}
	}
}

// TestLogCmdArgsValidation tests that the command accepts 0 or 1 argument.
func TestLogCmdArgsValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, false},
		{"one arg", []string{"session-id"}, false},
		{"two args", []string{"arg1", "arg2"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newLogCmd()
			cmd.SetArgs(tt.args)

			err := cmd.Args(cmd, tt.args)

			if tt.wantErr && err == nil {
				t.Error("expected args validation error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected args validation error: %v", err)
			}
		})
	}
}

// TestTailLines tests the tailLines utility function.
func TestTailLines(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("read last n lines", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "test.log")
		content := "line1\nline2\nline3\nline4\nline5\n"
		if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
			t.Fatalf("writing test log: %v", err)
		}

		// tailLines reads and prints to stdout, so we just verify no error
		err := tailLines(logPath, 3)
		if err != nil {
			t.Errorf("tailLines() error = %v", err)
		}
	})

	t.Run("request more lines than exist", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "small.log")
		content := "line1\nline2\n"
		if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
			t.Fatalf("writing test log: %v", err)
		}

		err := tailLines(logPath, 100)
		if err != nil {
			t.Errorf("tailLines() error = %v", err)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "nonexistent.log")

		// Should handle gracefully (print message, no error)
		err := tailLines(logPath, 20)
		if err != nil {
			t.Errorf("tailLines() should handle non-existent file gracefully, got error = %v", err)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "empty.log")
		if err := os.WriteFile(logPath, []byte(""), 0o644); err != nil {
			t.Fatalf("writing test log: %v", err)
		}

		err := tailLines(logPath, 20)
		if err != nil {
			t.Errorf("tailLines() error = %v", err)
		}
	})

	t.Run("single line no newline", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "single.log")
		content := "single line without newline"
		if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
			t.Fatalf("writing test log: %v", err)
		}

		err := tailLines(logPath, 5)
		if err != nil {
			t.Errorf("tailLines() error = %v", err)
		}
	})
}

// TestTailLinesEdgeCases tests edge cases for tailLines.
func TestTailLinesEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("zero lines requested", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "test.log")
		content := "line1\nline2\n"
		if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
			t.Fatalf("writing test log: %v", err)
		}

		// Request 0 lines - should still work
		err := tailLines(logPath, 0)
		if err != nil {
			t.Errorf("tailLines(0) error = %v", err)
		}
	})

	t.Run("negative lines", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "test2.log")
		content := "line1\nline2\n"
		if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
			t.Fatalf("writing test log: %v", err)
		}

		// Negative lines should be handled gracefully
		err := tailLines(logPath, -1)
		if err != nil {
			t.Errorf("tailLines(-1) error = %v", err)
		}
	})
}
