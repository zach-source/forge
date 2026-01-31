package detector

import (
	"testing"
)

func TestDetectorIsComplete(t *testing.T) {
	tests := []struct {
		name    string
		promise string
		output  string
		want    bool
	}{
		{
			name:    "exact match",
			promise: "DONE",
			output:  "Task completed. <promise>DONE</promise>",
			want:    true,
		},
		{
			name:    "case insensitive",
			promise: "COMPLETE",
			output:  "Finished everything. <promise>complete</promise>",
			want:    true,
		},
		{
			name:    "with whitespace",
			promise: "DONE",
			output:  "<promise> DONE </promise>",
			want:    true,
		},
		{
			name:    "no match",
			promise: "DONE",
			output:  "Still working on it...",
			want:    false,
		},
		{
			name:    "wrong promise text",
			promise: "DONE",
			output:  "<promise>NOT_DONE</promise>",
			want:    false,
		},
		{
			name:    "multiple promises - one matches",
			promise: "COMPLETE",
			output:  "<promise>PARTIAL</promise> then later <promise>COMPLETE</promise>",
			want:    true,
		},
		{
			name:    "multiline output",
			promise: "API DONE",
			output: `Working on the API...
Created endpoints.
Running tests.
All tests passing!
<promise>API DONE</promise>`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(tt.promise)
			got := d.IsComplete(tt.output)
			if got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectorExtractPromises(t *testing.T) {
	d := New("anything")

	output := "Start <promise>ONE</promise> middle <promise>TWO</promise> end"
	promises := d.ExtractPromises(output)

	if len(promises) != 2 {
		t.Errorf("Expected 2 promises, got %d", len(promises))
	}

	if promises[0] != "ONE" {
		t.Errorf("First promise = %q, want %q", promises[0], "ONE")
	}
	if promises[1] != "TWO" {
		t.Errorf("Second promise = %q, want %q", promises[1], "TWO")
	}
}

func TestDetectorCheck(t *testing.T) {
	d := New("COMPLETE")

	result := d.Check("<promise>PARTIAL</promise> and <promise>COMPLETE</promise>")

	if !result.Complete {
		t.Error("Expected Complete to be true")
	}

	if len(result.Promises) != 2 {
		t.Errorf("Expected 2 promises, got %d", len(result.Promises))
	}
}
