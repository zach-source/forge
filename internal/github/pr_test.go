package github

import (
	"encoding/json"
	"testing"
)

func TestPRStateUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PRState
		wantErr bool
	}{
		{
			name:  "draft PR",
			input: `{"number":42,"state":"open","isDraft":true,"title":"WIP: feature","headRefName":"task/abc","url":"https://github.com/owner/repo/pull/42"}`,
			want: PRState{
				Number: 42,
				State:  "open",
				Draft:  true,
				Title:  "WIP: feature",
				Branch: "task/abc",
				URL:    "https://github.com/owner/repo/pull/42",
			},
		},
		{
			name:  "ready PR",
			input: `{"number":99,"state":"open","isDraft":false,"title":"Add feature X","headRefName":"task/xyz","url":"https://github.com/owner/repo/pull/99"}`,
			want: PRState{
				Number: 99,
				State:  "open",
				Draft:  false,
				Title:  "Add feature X",
				Branch: "task/xyz",
				URL:    "https://github.com/owner/repo/pull/99",
			},
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got PRState
			err := json.Unmarshal([]byte(tt.input), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Number != tt.want.Number {
					t.Errorf("Number = %d, want %d", got.Number, tt.want.Number)
				}
				if got.State != tt.want.State {
					t.Errorf("State = %q, want %q", got.State, tt.want.State)
				}
				if got.Draft != tt.want.Draft {
					t.Errorf("Draft = %v, want %v", got.Draft, tt.want.Draft)
				}
				if got.Title != tt.want.Title {
					t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
				}
				if got.Branch != tt.want.Branch {
					t.Errorf("Branch = %q, want %q", got.Branch, tt.want.Branch)
				}
				if got.URL != tt.want.URL {
					t.Errorf("URL = %q, want %q", got.URL, tt.want.URL)
				}
			}
		})
	}
}

func TestPRStateListUnmarshal(t *testing.T) {
	input := `[{"number":1,"state":"open","isDraft":true,"title":"WIP: A","headRefName":"task/a","url":"http://example.com/1"},{"number":2,"state":"open","isDraft":false,"title":"B","headRefName":"task/b","url":"http://example.com/2"}]`

	var prs []PRState
	if err := json.Unmarshal([]byte(input), &prs); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(prs) != 2 {
		t.Fatalf("got %d PRs, want 2", len(prs))
	}

	if !prs[0].Draft {
		t.Error("first PR should be draft")
	}
	if prs[1].Draft {
		t.Error("second PR should not be draft")
	}
}
