package context

import (
	"regexp"
	"strings"
)

// DefaultRelevanceLimit is the default number of relevant learnings to return.
const DefaultRelevanceLimit = 5

// FindRelevantLearnings finds learnings relevant to a task based on keywords.
func FindRelevantLearnings(store *LearningsStore, title, description string, limit int) []Learning {
	if store == nil || len(store.Learnings) == 0 {
		return nil
	}

	if limit <= 0 {
		limit = DefaultRelevanceLimit
	}

	// Extract keywords from task
	taskKeywords := ExtractKeywords(title, description)
	if len(taskKeywords) == 0 {
		return nil
	}

	// Score learnings by keyword overlap
	type scored struct {
		learning Learning
		score    int
	}
	var results []scored

	for _, l := range store.Learnings {
		score := scoreOverlap(l.Keywords, taskKeywords)

		// Also check file path keywords against task
		for _, f := range l.Files {
			fileKeywords := ExtractKeywords(f, "")
			score += scoreOverlap(fileKeywords, taskKeywords) / 2
		}

		// Check if summary/problem/solution contain task keywords
		learningText := strings.ToLower(l.Summary + " " + l.Problem + " " + l.Solution)
		for _, kw := range taskKeywords {
			if strings.Contains(learningText, kw) {
				score++
			}
		}

		if score > 0 {
			results = append(results, scored{l, score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Take top N
	var relevant []Learning
	for i := 0; i < limit && i < len(results); i++ {
		relevant = append(relevant, results[i].learning)
	}
	return relevant
}

// ExtractKeywords extracts meaningful keywords from text.
func ExtractKeywords(texts ...string) []string {
	combined := strings.Join(texts, " ")
	combined = strings.ToLower(combined)

	// Split on word boundaries
	wordRegex := regexp.MustCompile(`[a-z0-9]+`)
	words := wordRegex.FindAllString(combined, -1)

	// Filter stop words and short words
	keywords := make(map[string]struct{})
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		if isStopWord(word) {
			continue
		}
		keywords[word] = struct{}{}
	}

	// Convert to slice
	result := make([]string, 0, len(keywords))
	for kw := range keywords {
		result = append(result, kw)
	}
	return result
}

// scoreOverlap counts how many keywords from set A appear in set B.
func scoreOverlap(setA, setB []string) int {
	bMap := make(map[string]struct{}, len(setB))
	for _, kw := range setB {
		bMap[kw] = struct{}{}
	}

	score := 0
	for _, kw := range setA {
		if _, found := bMap[kw]; found {
			score++
		}
	}
	return score
}

// isStopWord returns true if the word is a common stop word.
func isStopWord(word string) bool {
	stopWords := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "are": {}, "but": {}, "not": {},
		"you": {}, "all": {}, "can": {}, "has": {}, "her": {}, "was": {},
		"one": {}, "our": {}, "out": {}, "day": {}, "get": {}, "got": {},
		"him": {}, "his": {}, "how": {}, "its": {}, "may": {}, "new": {},
		"now": {}, "old": {}, "see": {}, "two": {}, "way": {}, "who": {},
		"did": {}, "she": {}, "use": {}, "any": {}, "had": {}, "been": {},
		"each": {}, "have": {}, "this": {}, "that": {}, "with": {}, "they": {},
		"from": {}, "were": {}, "said": {}, "more": {}, "when": {}, "will": {},
		"some": {}, "than": {}, "them": {}, "very": {}, "just": {}, "over": {},
		"such": {}, "into": {}, "only": {}, "also": {}, "then": {}, "what": {},
		"your": {}, "could": {}, "would": {}, "should": {}, "which": {},
		"their": {}, "there": {}, "these": {}, "those": {}, "being": {},
		"about": {}, "after": {}, "before": {}, "other": {}, "where": {},
		"implement": {}, "create": {}, "update": {}, "delete": {}, "need": {},
		"make": {}, "like": {}, "file": {}, "code": {}, "want": {},
	}
	_, found := stopWords[word]
	return found
}

// MatchesByFile finds learnings that reference specific files.
func MatchesByFile(store *LearningsStore, filePaths []string, limit int) []Learning {
	if store == nil || len(store.Learnings) == 0 || len(filePaths) == 0 {
		return nil
	}

	if limit <= 0 {
		limit = DefaultRelevanceLimit
	}

	// Normalize file paths for matching
	normalizedPaths := make(map[string]struct{})
	for _, fp := range filePaths {
		// Store both full path and basename
		normalizedPaths[strings.ToLower(fp)] = struct{}{}
		parts := strings.Split(fp, "/")
		if len(parts) > 0 {
			normalizedPaths[strings.ToLower(parts[len(parts)-1])] = struct{}{}
		}
	}

	var matches []Learning
	for _, l := range store.Learnings {
		for _, lf := range l.Files {
			lfLower := strings.ToLower(lf)
			parts := strings.Split(lf, "/")
			basename := ""
			if len(parts) > 0 {
				basename = strings.ToLower(parts[len(parts)-1])
			}

			if _, found := normalizedPaths[lfLower]; found {
				matches = append(matches, l)
				break
			}
			if basename != "" {
				if _, found := normalizedPaths[basename]; found {
					matches = append(matches, l)
					break
				}
			}
		}
		if len(matches) >= limit {
			break
		}
	}
	return matches
}
