package main

import (
	"testing"

	"github.com/spf13/pflag"
)

// TestNewStatusCmd tests that the status command is created correctly.
func TestNewStatusCmd(t *testing.T) {
	cmd := newStatusCmd()

	if cmd == nil {
		t.Fatal("newStatusCmd() returned nil")
	}

	if cmd.Use != "status [session-id]" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "status [session-id]")
	}

	if cmd.Short == "" {
		t.Error("cmd.Short should not be empty")
	}

	if cmd.Long == "" {
		t.Error("cmd.Long should not be empty")
	}
}

// TestStatusCmdArgsValidation tests that the command accepts 0 or 1 argument.
func TestStatusCmdArgsValidation(t *testing.T) {
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
			cmd := newStatusCmd()
			cmd.SetArgs(tt.args)

			// The command will try to discover sessions which may fail,
			// but args validation happens first
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

// TestStatusCmdNoFlags tests that status command has no custom flags.
func TestStatusCmdNoFlags(t *testing.T) {
	cmd := newStatusCmd()

	// Count non-inherited flags
	count := 0
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		count++
	})

	// Should only have help flag inherited
	if count > 1 {
		t.Errorf("status command should have minimal flags, got %d", count)
	}
}
