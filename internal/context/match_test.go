package context

import (
	"testing"
)

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name     string
		texts    []string
		contains []string
		excludes []string
	}{
		{
			name:     "basic extraction",
			texts:    []string{"Fix worker registry bug"},
			contains: []string{"fix", "worker", "registry", "bug"},
			excludes: []string{},
		},
		{
			name:     "filters short words",
			texts:    []string{"a an the to fix bug"},
			contains: []string{"fix", "bug"},
			excludes: []string{"a", "an", "to"},
		},
		{
			name:     "filters stop words",
			texts:    []string{"implement the new feature with this approach"},
			contains: []string{"feature", "approach"},
			excludes: []string{"the", "with", "this", "implement"},
		},
		{
			name:     "multiple texts",
			texts:    []string{"worker bug", "registry fix"},
			contains: []string{"worker", "bug", "registry", "fix"},
		},
		{
			name:     "handles paths",
			texts:    []string{"internal/worker/lifecycle.go"},
			contains: []string{"internal", "worker", "lifecycle"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keywords := ExtractKeywords(tt.texts...)
			keywordMap := make(map[string]bool)
			for _, k := range keywords {
				keywordMap[k] = true
			}

			for _, want := range tt.contains {
				if !keywordMap[want] {
					t.Errorf("Expected keyword %q not found in %v", want, keywords)
				}
			}

			for _, exclude := range tt.excludes {
				if keywordMap[exclude] {
					t.Errorf("Stop word %q should be excluded from %v", exclude, keywords)
				}
			}
		})
	}
}

func TestFindRelevantLearnings(t *testing.T) {
	store := &LearningsStore{
		Learnings: []Learning{
			{
				ID:       "1",
				Summary:  "Worker registry deadlock fix",
				Problem:  "Concurrent access to registry caused deadlock",
				Solution: "Use RWMutex instead of Mutex",
				Keywords: []string{"worker", "registry", "deadlock", "mutex", "concurrent"},
			},
			{
				ID:       "2",
				Summary:  "Tmux session cleanup",
				Problem:  "Sessions not cleaned up on error",
				Solution: "Add defer cleanup in lifecycle",
				Keywords: []string{"tmux", "session", "cleanup", "lifecycle"},
			},
			{
				ID:       "3",
				Summary:  "Build error undefined symbol",
				Problem:  "Missing import for context package",
				Solution: "Add context import to worker.go",
				Keywords: []string{"build", "error", "import", "context"},
				Files:    []string{"internal/worker/worker.go"},
			},
		},
	}

	tests := []struct {
		name        string
		title       string
		description string
		wantIDs     []string
	}{
		{
			name:        "matches worker keywords",
			title:       "Fix worker registry issue",
			description: "There's a bug in the worker registry",
			wantIDs:     []string{"1"},
		},
		{
			name:        "matches tmux keywords",
			title:       "Tmux session management",
			description: "Improve session handling",
			wantIDs:     []string{"2"},
		},
		{
			name:        "matches by file path",
			title:       "Update worker.go",
			description: "Changes to internal/worker/worker.go",
			wantIDs:     []string{"3", "1"},
		},
		{
			name:        "no matches for unrelated task",
			title:       "Update pricing page",
			description: "Change monthly subscription rates",
			wantIDs:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindRelevantLearnings(store, tt.title, tt.description, 5)

			if len(tt.wantIDs) == 0 && len(got) > 0 {
				t.Errorf("Expected no matches, got %d", len(got))
				return
			}

			if len(tt.wantIDs) > 0 && len(got) == 0 {
				t.Errorf("Expected matches %v, got none", tt.wantIDs)
				return
			}

			// Check first result if expected
			if len(tt.wantIDs) > 0 && len(got) > 0 {
				found := false
				for _, id := range tt.wantIDs {
					if got[0].ID == id {
						found = true
						break
					}
				}
				if !found {
					gotIDs := make([]string, len(got))
					for i, l := range got {
						gotIDs[i] = l.ID
					}
					t.Errorf("Expected one of %v as top result, got %v", tt.wantIDs, gotIDs)
				}
			}
		})
	}
}

func TestFindRelevantLearningsLimit(t *testing.T) {
	store := &LearningsStore{
		Learnings: []Learning{
			{ID: "1", Summary: "Test 1", Keywords: []string{"test", "worker"}},
			{ID: "2", Summary: "Test 2", Keywords: []string{"test", "worker"}},
			{ID: "3", Summary: "Test 3", Keywords: []string{"test", "worker"}},
			{ID: "4", Summary: "Test 4", Keywords: []string{"test", "worker"}},
			{ID: "5", Summary: "Test 5", Keywords: []string{"test", "worker"}},
		},
	}

	got := FindRelevantLearnings(store, "test worker", "", 3)
	if len(got) != 3 {
		t.Errorf("Expected 3 results with limit, got %d", len(got))
	}
}

func TestFindRelevantLearningsNilStore(t *testing.T) {
	got := FindRelevantLearnings(nil, "test", "description", 5)
	if got != nil {
		t.Errorf("Expected nil for nil store, got %v", got)
	}
}

func TestFindRelevantLearningsEmptyStore(t *testing.T) {
	store := &LearningsStore{Learnings: []Learning{}}
	got := FindRelevantLearnings(store, "test", "description", 5)
	if got != nil {
		t.Errorf("Expected nil for empty store, got %v", got)
	}
}

func TestMatchesByFile(t *testing.T) {
	store := &LearningsStore{
		Learnings: []Learning{
			{
				ID:    "1",
				Files: []string{"internal/worker/registry.go"},
			},
			{
				ID:    "2",
				Files: []string{"internal/tmux/session.go"},
			},
			{
				ID:    "3",
				Files: []string{"cmd/forge/main.go", "internal/worker/lifecycle.go"},
			},
		},
	}

	tests := []struct {
		name      string
		filePaths []string
		wantIDs   []string
	}{
		{
			name:      "match by full path",
			filePaths: []string{"internal/worker/registry.go"},
			wantIDs:   []string{"1"},
		},
		{
			name:      "match by basename",
			filePaths: []string{"registry.go"},
			wantIDs:   []string{"1"},
		},
		{
			name:      "match multiple files",
			filePaths: []string{"registry.go", "lifecycle.go"},
			wantIDs:   []string{"1", "3"},
		},
		{
			name:      "no match",
			filePaths: []string{"nonexistent.go"},
			wantIDs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesByFile(store, tt.filePaths, 10)

			gotIDs := make([]string, len(got))
			for i, l := range got {
				gotIDs[i] = l.ID
			}

			if len(tt.wantIDs) != len(gotIDs) {
				t.Errorf("Expected %d matches, got %d: %v", len(tt.wantIDs), len(gotIDs), gotIDs)
				return
			}

			for i, wantID := range tt.wantIDs {
				if gotIDs[i] != wantID {
					t.Errorf("Match %d: expected ID %s, got %s", i, wantID, gotIDs[i])
				}
			}
		})
	}
}

func TestIsStopWord(t *testing.T) {
	stopWords := []string{"the", "and", "for", "are", "but", "not", "implement", "create"}
	for _, word := range stopWords {
		if !isStopWord(word) {
			t.Errorf("%q should be a stop word", word)
		}
	}

	normalWords := []string{"worker", "registry", "tmux", "session", "error"}
	for _, word := range normalWords {
		if isStopWord(word) {
			t.Errorf("%q should not be a stop word", word)
		}
	}
}
