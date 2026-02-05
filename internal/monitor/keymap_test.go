package monitor

import (
	"testing"
)

func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	// Test navigation keys
	if len(km.Up.Keys()) == 0 {
		t.Error("Up key should have bindings")
	}
	if len(km.Down.Keys()) == 0 {
		t.Error("Down key should have bindings")
	}

	// Test tab navigation
	if len(km.TabNext.Keys()) == 0 {
		t.Error("TabNext key should have bindings")
	}
	if len(km.TabPrev.Keys()) == 0 {
		t.Error("TabPrev key should have bindings")
	}
	if len(km.Tab1.Keys()) == 0 {
		t.Error("Tab1 key should have bindings")
	}
	if len(km.Tab2.Keys()) == 0 {
		t.Error("Tab2 key should have bindings")
	}
	if len(km.Tab3.Keys()) == 0 {
		t.Error("Tab3 key should have bindings")
	}
	if len(km.Tab4.Keys()) == 0 {
		t.Error("Tab4 key should have bindings")
	}

	// Test action keys
	if len(km.Attach.Keys()) == 0 {
		t.Error("Attach key should have bindings")
	}
	if len(km.Cancel.Keys()) == 0 {
		t.Error("Cancel key should have bindings")
	}
	if len(km.Refresh.Keys()) == 0 {
		t.Error("Refresh key should have bindings")
	}
	if len(km.Help.Keys()) == 0 {
		t.Error("Help key should have bindings")
	}
	if len(km.Quit.Keys()) == 0 {
		t.Error("Quit key should have bindings")
	}
}

func TestDefaultKeyMap_SpecificKeys(t *testing.T) {
	km := DefaultKeyMap()

	// Check specific key bindings
	tests := []struct {
		name     string
		keys     []string
		expected []string
	}{
		{"Up", km.Up.Keys(), []string{"up", "k"}},
		{"Down", km.Down.Keys(), []string{"down", "j"}},
		{"TabNext", km.TabNext.Keys(), []string{"tab", "l"}},
		{"TabPrev", km.TabPrev.Keys(), []string{"shift+tab", "h"}},
		{"Tab1", km.Tab1.Keys(), []string{"1"}},
		{"Tab2", km.Tab2.Keys(), []string{"2"}},
		{"Tab3", km.Tab3.Keys(), []string{"3"}},
		{"Tab4", km.Tab4.Keys(), []string{"4"}},
		{"Attach", km.Attach.Keys(), []string{"a", "enter"}},
		{"Cancel", km.Cancel.Keys(), []string{"c"}},
		{"Refresh", km.Refresh.Keys(), []string{"r"}},
		{"Help", km.Help.Keys(), []string{"?"}},
		{"Quit", km.Quit.Keys(), []string{"q", "ctrl+c"}},
	}

	for _, tt := range tests {
		if len(tt.keys) != len(tt.expected) {
			t.Errorf("%s: expected %d keys, got %d", tt.name, len(tt.expected), len(tt.keys))
			continue
		}
		for i, key := range tt.keys {
			if key != tt.expected[i] {
				t.Errorf("%s[%d]: expected %q, got %q", tt.name, i, tt.expected[i], key)
			}
		}
	}
}

func TestKeyMap_ShortHelp(t *testing.T) {
	km := DefaultKeyMap()
	shortHelp := km.ShortHelp()

	if len(shortHelp) == 0 {
		t.Error("ShortHelp should return bindings")
	}

	// Should contain essential bindings
	expectedCount := 6 // TabNext, Up, Down, Attach, Refresh, Quit
	if len(shortHelp) != expectedCount {
		t.Errorf("ShortHelp should return %d bindings, got %d", expectedCount, len(shortHelp))
	}
}

func TestKeyMap_FullHelp(t *testing.T) {
	km := DefaultKeyMap()
	fullHelp := km.FullHelp()

	if len(fullHelp) == 0 {
		t.Error("FullHelp should return binding groups")
	}

	// Should have 4 groups
	expectedGroups := 4
	if len(fullHelp) != expectedGroups {
		t.Errorf("FullHelp should return %d groups, got %d", expectedGroups, len(fullHelp))
	}

	// Each group should have bindings
	for i, group := range fullHelp {
		if len(group) == 0 {
			t.Errorf("FullHelp group %d should not be empty", i)
		}
	}
}

func TestKeyMap_HelpText(t *testing.T) {
	km := DefaultKeyMap()

	// Check help text is set
	if km.Up.Help().Key == "" {
		t.Error("Up key should have help text")
	}
	if km.Down.Help().Key == "" {
		t.Error("Down key should have help text")
	}
	if km.Quit.Help().Key == "" {
		t.Error("Quit key should have help text")
	}
}
