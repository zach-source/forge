package main

import (
	"testing"

	"github.com/spf13/pflag"
)

// TestNewAttachCmd tests that the attach command is created correctly.
func TestNewAttachCmd(t *testing.T) {
	cmd := newAttachCmd()

	if cmd == nil {
		t.Fatal("newAttachCmd() returned nil")
	}

	if cmd.Use != "attach [session-id]" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "attach [session-id]")
	}

	if cmd.Short == "" {
		t.Error("cmd.Short should not be empty")
	}

	if cmd.Long == "" {
		t.Error("cmd.Long should not be empty")
	}
}

// TestAttachCmdArgsValidation tests that the command accepts 0 or 1 argument.
func TestAttachCmdArgsValidation(t *testing.T) {
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
			cmd := newAttachCmd()
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

// TestAttachCmdNoFlags tests that attach command has no custom flags.
func TestAttachCmdNoFlags(t *testing.T) {
	cmd := newAttachCmd()

	// Count local flags (non-inherited)
	localCount := 0
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		localCount++
	})

	// Should have no local flags
	if localCount > 0 {
		t.Errorf("attach command should have no local flags, got %d", localCount)
	}
}

// TestSyscallExec tests the syscallExec helper function.
func TestSyscallExec(t *testing.T) {
	// Test with a command that exits quickly
	t.Run("run echo", func(t *testing.T) {
		err := syscallExec("/bin/echo", []string{"echo", "test"}, nil)
		if err != nil {
			t.Errorf("syscallExec(echo) error = %v", err)
		}
	})

	t.Run("non-existent binary", func(t *testing.T) {
		err := syscallExec("/nonexistent/binary", []string{"binary"}, nil)
		if err == nil {
			t.Error("expected error for non-existent binary")
		}
	})
}
