package worker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NATOAlphabet contains the NATO phonetic alphabet names.
var NATOAlphabet = []string{
	"alpha", "bravo", "charlie", "delta", "echo",
	"foxtrot", "golf", "hotel", "india", "juliet",
	"kilo", "lima", "mike", "november", "oscar",
	"papa", "quebec", "romeo", "sierra", "tango",
	"uniform", "victor", "whiskey", "xray", "yankee", "zulu",
}

// GenerateID creates a unique worker ID in the format "w-xxxxxxxx".
func GenerateID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("w-%08x", uint32(0))
	}
	return "w-" + hex.EncodeToString(b)
}

// NextName returns the next available NATO alphabet name given a list of used names.
func NextName(usedNames []string) string {
	used := make(map[string]bool)
	for _, name := range usedNames {
		used[name] = true
	}

	for _, name := range NATOAlphabet {
		if !used[name] {
			return name
		}
	}

	// All NATO names used, generate numbered extension
	// e.g., alpha2, bravo2, etc.
	suffix := 2
	for {
		for _, name := range NATOAlphabet {
			numbered := fmt.Sprintf("%s%d", name, suffix)
			if !used[numbered] {
				return numbered
			}
		}
		suffix++
		if suffix > 100 {
			// Safety limit
			return GenerateID()
		}
	}
}

// IsValidName checks if a name is a valid NATO alphabet name or numbered variant.
func IsValidName(name string) bool {
	for _, nato := range NATOAlphabet {
		if name == nato {
			return true
		}
		// Check for numbered variants like alpha2, bravo3
		for i := 2; i <= 100; i++ {
			if name == fmt.Sprintf("%s%d", nato, i) {
				return true
			}
		}
	}
	return false
}

// NameIndex returns the index of a NATO name (0-25), or -1 if not found.
func NameIndex(name string) int {
	for i, nato := range NATOAlphabet {
		if name == nato {
			return i
		}
	}
	return -1
}
