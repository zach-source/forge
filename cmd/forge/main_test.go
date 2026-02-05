package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestRootCommandStructure tests that the root command is set up correctly.
func TestRootCommandStructure(t *testing.T) {
	// Create the root command as main() would
	rootCmd := &cobra.Command{
		Use:   "forge",
		Short: "Claude session runner",
	}

	rootCmd.AddCommand(
		newStartCmd(),
		newAttachCmd(),
		newStatusCmd(),
		newCancelCmd(),
		newListCmd(),
		newLogCmd(),
	)

	// Verify all subcommands are registered
	expectedCommands := []string{
		"start",
		"attach",
		"status",
		"cancel",
		"list",
		"log",
	}

	commands := rootCmd.Commands()
	commandMap := make(map[string]bool)
	for _, cmd := range commands {
		commandMap[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !commandMap[expected] {
			t.Errorf("expected command %q to be registered", expected)
		}
	}
}

// TestAllCommandsHaveRunE tests that all commands have RunE defined.
func TestAllCommandsHaveRunE(t *testing.T) {
	commands := []*cobra.Command{
		newStartCmd(),
		newAttachCmd(),
		newStatusCmd(),
		newCancelCmd(),
		newListCmd(),
		newLogCmd(),
	}

	for _, cmd := range commands {
		if cmd.RunE == nil {
			t.Errorf("command %q should have RunE defined", cmd.Name())
		}
	}
}

// TestAllCommandsHaveShortDescription tests that all commands have short descriptions.
func TestAllCommandsHaveShortDescription(t *testing.T) {
	commands := []*cobra.Command{
		newStartCmd(),
		newAttachCmd(),
		newStatusCmd(),
		newCancelCmd(),
		newListCmd(),
		newLogCmd(),
	}

	for _, cmd := range commands {
		if cmd.Short == "" {
			t.Errorf("command %q should have Short description", cmd.Name())
		}
	}
}

// TestAllCommandsHaveLongDescription tests that all commands have long descriptions.
func TestAllCommandsHaveLongDescription(t *testing.T) {
	commands := []*cobra.Command{
		newStartCmd(),
		newAttachCmd(),
		newStatusCmd(),
		newCancelCmd(),
		newListCmd(),
		newLogCmd(),
	}

	for _, cmd := range commands {
		if cmd.Long == "" {
			t.Errorf("command %q should have Long description", cmd.Name())
		}
	}
}

// TestVersionIsSet tests that Version variable exists.
func TestVersionIsSet(t *testing.T) {
	// Version should be defined (set at build time or default to "dev")
	if Version == "" {
		t.Error("Version should not be empty")
	}
}

// TestCommandUsage tests that commands have proper usage strings.
func TestCommandUsage(t *testing.T) {
	tests := []struct {
		cmd  *cobra.Command
		want string
	}{
		{newStartCmd(), "start <prompt>"},
		{newAttachCmd(), "attach [session-id]"},
		{newStatusCmd(), "status [session-id]"},
		{newCancelCmd(), "cancel [session-id]"},
		{newListCmd(), "list"},
		{newLogCmd(), "log [session-id]"},
	}

	for _, tt := range tests {
		if tt.cmd.Use != tt.want {
			t.Errorf("%s.Use = %q, want %q", tt.cmd.Name(), tt.cmd.Use, tt.want)
		}
	}
}
