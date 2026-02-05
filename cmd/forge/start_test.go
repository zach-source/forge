package main

import (
	"testing"
)

// TestNewStartCmd tests that the start command is created correctly.
func TestNewStartCmd(t *testing.T) {
	cmd := newStartCmd()

	if cmd == nil {
		t.Fatal("newStartCmd() returned nil")
	}

	if cmd.Use != "start <prompt>" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "start <prompt>")
	}

	if cmd.Short == "" {
		t.Error("cmd.Short should not be empty")
	}

	if cmd.Long == "" {
		t.Error("cmd.Long should not be empty")
	}
}

// TestStartCmdFlags tests that all expected flags are defined.
func TestStartCmdFlags(t *testing.T) {
	cmd := newStartCmd()

	expectedFlags := []struct {
		name      string
		shorthand string
	}{
		{"promise", "p"},
		{"max", "m"},
		{"mcp", ""},
		{"mcp-config", ""},
		{"workdir", "w"},
		{"no-skip", ""},
		{"id", ""},
	}

	for _, ef := range expectedFlags {
		flag := cmd.Flags().Lookup(ef.name)
		if flag == nil {
			t.Errorf("flag %q should exist", ef.name)
			continue
		}

		if ef.shorthand != "" && flag.Shorthand != ef.shorthand {
			t.Errorf("flag %q shorthand = %q, want %q", ef.name, flag.Shorthand, ef.shorthand)
		}
	}
}

// TestStartCmdFlagDefaults tests that flag defaults are correct.
func TestStartCmdFlagDefaults(t *testing.T) {
	cmd := newStartCmd()

	tests := []struct {
		flag string
		want string
	}{
		{"max", "50"},
		{"mcp", "graphiti,context7"},
		{"workdir", ""},
		{"mcp-config", ""},
		{"id", ""},
	}

	for _, tt := range tests {
		flag := cmd.Flags().Lookup(tt.flag)
		if flag == nil {
			t.Errorf("flag %q not found", tt.flag)
			continue
		}

		if flag.DefValue != tt.want {
			t.Errorf("flag %q default = %q, want %q", tt.flag, flag.DefValue, tt.want)
		}
	}
}

// TestStartCmdRequiredFlags tests that the promise flag is marked required.
func TestStartCmdRequiredFlags(t *testing.T) {
	cmd := newStartCmd()

	// Check that promise is required
	promiseFlag := cmd.Flags().Lookup("promise")
	if promiseFlag == nil {
		t.Fatal("promise flag should exist")
	}

	// When the flag is marked required, annotations are set
	annotations := promiseFlag.Annotations
	if annotations == nil {
		t.Error("promise flag should have required annotation")
	} else {
		if _, ok := annotations["cobra_annotation_bash_completion_one_required_flag"]; !ok {
			// Check for alternative required annotation
			if len(annotations) == 0 {
				t.Error("promise flag should have required annotation")
			}
		}
	}
}

// TestStartCmdArgsValidation tests that the command requires exactly 1 argument.
func TestStartCmdArgsValidation(t *testing.T) {
	cmd := newStartCmd()

	// Set flags to satisfy requirements
	cmd.SetArgs([]string{})
	cmd.Flags().Set("promise", "DONE")

	// Execute should fail without args (but won't run the RunE due to Args check)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing prompt argument")
	}
}

// TestStartCmdTooManyArgs tests that the command rejects too many arguments.
func TestStartCmdTooManyArgs(t *testing.T) {
	cmd := newStartCmd()

	cmd.SetArgs([]string{"arg1", "arg2"})
	cmd.Flags().Set("promise", "DONE")

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for too many arguments")
	}
}

// TestStartCmdMissingPromise tests that the command requires --promise flag.
func TestStartCmdMissingPromise(t *testing.T) {
	cmd := newStartCmd()

	cmd.SetArgs([]string{"test prompt"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing --promise flag")
	}
}
