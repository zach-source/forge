// Package detector provides completion detection for forge agents.
package detector

import (
	"regexp"
	"strings"
)

// Detector checks Claude output for completion signals.
type Detector struct {
	promise string
	pattern *regexp.Regexp
}

// New creates a new completion detector.
func New(promise string) *Detector {
	// Match <promise>TEXT</promise> tags
	pattern := regexp.MustCompile(`<promise>([^<]*)</promise>`)
	return &Detector{
		promise: strings.TrimSpace(promise),
		pattern: pattern,
	}
}

// IsComplete checks if the output contains the completion promise.
func (d *Detector) IsComplete(output string) bool {
	matches := d.pattern.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			text := strings.TrimSpace(match[1])
			if d.matches(text) {
				return true
			}
		}
	}
	return false
}

// matches checks if the extracted text matches the promise.
func (d *Detector) matches(text string) bool {
	// Exact match (case-insensitive)
	if strings.EqualFold(text, d.promise) {
		return true
	}

	// Contains match for longer promises
	if len(d.promise) > 10 && strings.Contains(strings.ToLower(text), strings.ToLower(d.promise)) {
		return true
	}

	return false
}

// ExtractPromises extracts all promise tags from the output.
func (d *Detector) ExtractPromises(output string) []string {
	matches := d.pattern.FindAllStringSubmatch(output, -1)
	var promises []string
	for _, match := range matches {
		if len(match) >= 2 {
			promises = append(promises, strings.TrimSpace(match[1]))
		}
	}
	return promises
}

// Result represents a completion detection result.
type Result struct {
	Complete bool
	Promises []string
}

// Check performs a full completion check and returns detailed results.
func (d *Detector) Check(output string) *Result {
	promises := d.ExtractPromises(output)
	complete := false

	for _, p := range promises {
		if d.matches(p) {
			complete = true
			break
		}
	}

	return &Result{
		Complete: complete,
		Promises: promises,
	}
}
