// Package complexity provides task complexity estimation and load balancing.
package complexity

import (
	"fmt"
	"strings"
)

// Complexity represents a task size estimate.
type Complexity string

const (
	ComplexitySmall  Complexity = "S"
	ComplexityMedium Complexity = "M"
	ComplexityLarge  Complexity = "L"
	ComplexityXL     Complexity = "XL"
)

// Valid returns true if the complexity value is recognized.
func (c Complexity) Valid() bool {
	switch c {
	case ComplexitySmall, ComplexityMedium, ComplexityLarge, ComplexityXL:
		return true
	default:
		return false
	}
}

// Parse parses a string into a Complexity value.
// Accepts: S, M, L, XL (case-insensitive).
func Parse(s string) (Complexity, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "S", "SMALL":
		return ComplexitySmall, nil
	case "M", "MEDIUM":
		return ComplexityMedium, nil
	case "L", "LARGE":
		return ComplexityLarge, nil
	case "XL", "EXTRA-LARGE", "XLARGE":
		return ComplexityXL, nil
	default:
		return "", fmt.Errorf("invalid complexity: %q (valid: S, M, L, XL)", s)
	}
}

// Score returns a numeric score for load balancing.
// S=1, M=2, L=3, XL=4.
func Score(c Complexity) int {
	switch c {
	case ComplexitySmall:
		return 1
	case ComplexityMedium:
		return 2
	case ComplexityLarge:
		return 3
	case ComplexityXL:
		return 4
	default:
		return 2 // default to medium
	}
}

// Estimate holds the result of complexity estimation.
type Estimate struct {
	Size       Complexity `json:"size"`
	Factors    []string   `json:"factors"`
	FileCount  int        `json:"file_count"`
	Confidence float64    `json:"confidence"` // 0-1
}

// complexKeywords that indicate higher complexity.
var complexKeywords = []string{
	"refactor", "migrate", "redesign", "architecture",
	"rewrite", "overhaul", "distributed", "concurrent",
}

// simpleKeywords that indicate lower complexity.
var simpleKeywords = []string{
	"typo", "rename", "bump", "update version",
	"fix comment", "add label", "remove unused",
}

// EstimateFromText estimates complexity from title and description text.
func EstimateFromText(title, description string) Estimate {
	combined := strings.ToLower(title + " " + description)
	score := 0
	var factors []string

	// Check for simple keywords (reduce score)
	for _, kw := range simpleKeywords {
		if strings.Contains(combined, kw) {
			score -= 1
			factors = append(factors, fmt.Sprintf("simple keyword '%s'", kw))
		}
	}

	// Check for complex keywords (increase score)
	for _, kw := range complexKeywords {
		if strings.Contains(combined, kw) {
			score += 2
			factors = append(factors, fmt.Sprintf("complex keyword '%s'", kw))
		}
	}

	// Description length indicates scope
	if len(description) > 1000 {
		score += 2
		factors = append(factors, "long description (>1000 chars)")
	} else if len(description) > 500 {
		score += 1
		factors = append(factors, "moderate description (>500 chars)")
	}

	// Count file hints in description (lines starting with "- " or containing ".go", ".ts", etc.)
	fileCount := countFileHints(description)
	if fileCount > 5 {
		score += 2
		factors = append(factors, fmt.Sprintf("%d files mentioned", fileCount))
	} else if fileCount > 2 {
		score += 1
		factors = append(factors, fmt.Sprintf("%d files mentioned", fileCount))
	}

	// Check for multi-step indicators
	if strings.Contains(combined, "acceptance criteria") || strings.Contains(combined, "## ") {
		score += 1
		factors = append(factors, "structured requirements")
	}

	// Map score to complexity
	confidence := 0.5 // default confidence for text-based estimation
	var size Complexity
	switch {
	case score <= 0:
		size = ComplexitySmall
		confidence = 0.6
	case score <= 2:
		size = ComplexityMedium
		confidence = 0.5
	case score <= 4:
		size = ComplexityLarge
		confidence = 0.5
	default:
		size = ComplexityXL
		confidence = 0.4
	}

	if len(factors) == 0 {
		factors = append(factors, "no strong signals")
	}

	return Estimate{
		Size:       size,
		Factors:    factors,
		FileCount:  fileCount,
		Confidence: confidence,
	}
}

// countFileHints counts references to files in the description.
func countFileHints(description string) int {
	extensions := []string{".go", ".ts", ".js", ".py", ".md", ".yaml", ".yml", ".json", ".toml"}
	count := 0
	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSpace(line)
		for _, ext := range extensions {
			if strings.Contains(line, ext) {
				count++
				break
			}
		}
	}
	return count
}
