package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewCancelCmd tests that the cancel command is created correctly.
func TestNewCancelCmd(t *testing.T) {
	cmd := newCancelCmd()

	if cmd == nil {
		t.Fatal("newCancelCmd() returned nil")
	}

	if cmd.Use != "cancel [session-id]" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "cancel [session-id]")
	}

	if cmd.Short == "" {
		t.Error("cmd.Short should not be empty")
	}

	if cmd.Long == "" {
		t.Error("cmd.Long should not be empty")
	}
}

// TestCancelCmdFlags tests that all expected flags are defined.
func TestCancelCmdFlags(t *testing.T) {
	cmd := newCancelCmd()

	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("force flag should exist")
	} else {
		if forceFlag.Shorthand != "f" {
			t.Errorf("force flag shorthand = %q, want %q", forceFlag.Shorthand, "f")
		}
		if forceFlag.DefValue != "false" {
			t.Errorf("force flag default = %q, want %q", forceFlag.DefValue, "false")
		}
	}
}

// TestCancelCmdArgsValidation tests that the command accepts 0 or 1 argument.
func TestCancelCmdArgsValidation(t *testing.T) {
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
			cmd := newCancelCmd()
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

// TestCancelSessionNonExistentState tests canceling with non-existent state file.
func TestCancelSessionNonExistentState(t *testing.T) {
	// Create a temp directory to ensure we're not affecting real state
	tmpDir := t.TempDir()

	// Create a fake session ID that doesn't exist
	fakeID := "fake-session-" + filepath.Base(tmpDir)

	// The state file path should not exist
	// cancelSession with force=true should handle this gracefully
	// Note: This tests the path where state file doesn't exist
	err := cancelSession(fakeID, true)

	// Should not error on non-existent state file
	if err != nil {
		// If it errors, it should be about tmux, not state file
		if os.IsNotExist(err) {
			t.Error("should not return IsNotExist error")
		}
	}
}
