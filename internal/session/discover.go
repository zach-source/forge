package session

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// GenerateSessionID generates a unique session ID from a prompt.
func GenerateSessionID(prompt string) string {
	// Create a short hash from the prompt
	hash := sha256.Sum256([]byte(prompt))
	shortHash := hex.EncodeToString(hash[:])[:8]

	// Also create a readable prefix from first few words
	words := strings.Fields(prompt)
	prefix := ""
	for i := 0; i < 2 && i < len(words); i++ {
		word := strings.ToLower(words[i])
		// Clean the word - only keep alphanumeric
		clean := ""
		for _, r := range word {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				clean += string(r)
			}
		}
		if len(clean) > 0 {
			if len(clean) > 8 {
				clean = clean[:8]
			}
			if prefix != "" {
				prefix += "-"
			}
			prefix += clean
		}
	}

	if prefix == "" {
		prefix = "forge"
	}

	return prefix + "-" + shortHash
}

// ForgeSessionName returns the full tmux session name for a session ID.
func ForgeSessionName(id string) string {
	if strings.HasPrefix(id, "forge-") {
		return id
	}
	return "forge-" + id
}

// IsForgeSession checks if a tmux session name is a forge session.
func IsForgeSession(name string) bool {
	return strings.HasPrefix(name, "forge-")
}

// SessionIDFromName extracts the session ID from a tmux session name.
func SessionIDFromName(name string) string {
	if strings.HasPrefix(name, "forge-") {
		return name
	}
	return name
}
