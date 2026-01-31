package worker

import (
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id := GenerateID()

	if !strings.HasPrefix(id, "w-") {
		t.Errorf("GenerateID() = %q, want prefix 'w-'", id)
	}

	if len(id) != 10 { // "w-" + 8 hex chars
		t.Errorf("GenerateID() length = %d, want 10", len(id))
	}

	// Test uniqueness
	id2 := GenerateID()
	if id == id2 {
		t.Errorf("GenerateID() produced duplicate IDs")
	}
}

func TestNextName(t *testing.T) {
	tests := []struct {
		name      string
		usedNames []string
		want      string
	}{
		{"empty", nil, "alpha"},
		{"first used", []string{"alpha"}, "bravo"},
		{"some used", []string{"alpha", "bravo", "charlie"}, "delta"},
		{"all nato used", NATOAlphabet, "alpha2"},
		{"with numbered", append(NATOAlphabet, "alpha2"), "bravo2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextName(tt.usedNames)
			if got != tt.want {
				t.Errorf("NextName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"alpha", true},
		{"bravo", true},
		{"zulu", true},
		{"alpha2", true},
		{"zulu99", true},
		{"invalid", false},
		{"", false},
		{"Alpha", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidName(tt.name)
			if got != tt.valid {
				t.Errorf("IsValidName(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestNameIndex(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"alpha", 0},
		{"bravo", 1},
		{"zulu", 25},
		{"invalid", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NameIndex(tt.name)
			if got != tt.want {
				t.Errorf("NameIndex(%q) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}
