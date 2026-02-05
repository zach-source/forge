package main

import (
	"testing"
)

// TestNewListCmd tests that the list command is created correctly.
func TestNewListCmd(t *testing.T) {
	cmd := newListCmd()

	if cmd == nil {
		t.Fatal("newListCmd() returned nil")
	}

	if cmd.Use != "list" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "list")
	}

	if cmd.Short == "" {
		t.Error("cmd.Short should not be empty")
	}

	if cmd.Long == "" {
		t.Error("cmd.Long should not be empty")
	}
}

// TestListCmdAliases tests that the command has the expected aliases.
func TestListCmdAliases(t *testing.T) {
	cmd := newListCmd()

	found := false
	for _, alias := range cmd.Aliases {
		if alias == "ls" {
			found = true
			break
		}
	}

	if !found {
		t.Error("list command should have 'ls' alias")
	}
}

// TestListCmdFlags tests that all expected flags are defined.
func TestListCmdFlags(t *testing.T) {
	cmd := newListCmd()

	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Error("all flag should exist")
	} else {
		if allFlag.Shorthand != "a" {
			t.Errorf("all flag shorthand = %q, want %q", allFlag.Shorthand, "a")
		}
		if allFlag.DefValue != "false" {
			t.Errorf("all flag default = %q, want %q", allFlag.DefValue, "false")
		}
	}
}

// TestListCmdArgsValidation tests that the command accepts no arguments.
func TestListCmdArgsValidation(t *testing.T) {
	cmd := newListCmd()

	// List command should not require any args
	// Since list command doesn't set Args, it accepts any args by default
	cmd.SetArgs([]string{})

	// Verify it's usable (no Args validator set means any args work)
	if cmd.Args != nil {
		err := cmd.Args(cmd, []string{})
		if err != nil {
			t.Errorf("list command should accept no arguments: %v", err)
		}
	}
}
