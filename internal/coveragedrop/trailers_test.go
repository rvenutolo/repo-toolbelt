package coveragedrop_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/coveragedrop"
)

func TestDropTrailers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		message string
		want    []coveragedrop.Trailer
	}{
		{
			name:    "a trailer in the last paragraph",
			message: "refactor: drop the retry path\n\nCoverage-Drop: internal/x - the retry path it covered was removed\n",
			want:    []coveragedrop.Trailer{{Unit: "internal/x", Reason: "the retry path it covered was removed"}},
		},
		{
			name:    "beside Co-Authored-By",
			message: "refactor: x\n\nBody.\n\nCoverage-Drop: . - main lost its flag test\nCo-Authored-By: Marker <marker@example.invalid>\n",
			want:    []coveragedrop.Trailer{{Unit: ".", Reason: "main lost its flag test"}},
		},
		{
			name:    "two trailers",
			message: "refactor: x\n\nCoverage-Drop: . - one\nCoverage-Drop: lib - two\n",
			want:    []coveragedrop.Trailer{{Unit: ".", Reason: "one"}, {Unit: "lib", Reason: "two"}},
		},
		{
			name:    "a lower-case key",
			message: "refactor: x\n\ncoverage-drop: internal/x - removed\n",
			want:    []coveragedrop.Trailer{{Unit: "internal/x", Reason: "removed"}},
		},
		{
			name:    "a folded value",
			message: "refactor: x\n\nCoverage-Drop: internal/x -\n  the retry path it covered was removed\n",
			want:    []coveragedrop.Trailer{{Unit: "internal/x", Reason: "the retry path it covered was removed"}},
		},
		{
			name:    "no dash before the reason",
			message: "refactor: x\n\nCoverage-Drop: internal/x removed\n",
			want:    []coveragedrop.Trailer{{Unit: "internal/x", Reason: "removed"}},
		},
		{
			name:    "a dash and no reason",
			message: "refactor: x\n\nCoverage-Drop: internal/x -\n",
			want:    []coveragedrop.Trailer{{Unit: "internal/x"}},
		},
		{
			name:    "a unit and no reason",
			message: "refactor: x\n\nCoverage-Drop: internal/x\n",
			want:    []coveragedrop.Trailer{{Unit: "internal/x"}},
		},
		{
			name:    "glued to a body paragraph",
			message: "refactor: x\n\nThe retry path is gone.\nCoverage-Drop: internal/x - removed\n",
		},
		{
			name:    "in its own paragraph above the trailer block",
			message: "refactor: x\n\nCoverage-Drop: internal/x - removed\n\nCo-Authored-By: Marker <marker@example.invalid>\n",
		},
		{
			name:    "the subject alone",
			message: "Coverage-Drop: internal/x - removed\n",
		},
		{
			name:    "other trailers only",
			message: "refactor: x\n\nCo-Authored-By: Marker <marker@example.invalid>\n",
		},
		{
			name:    "an empty value",
			message: "refactor: x\n\nCoverage-Drop:\nCo-Authored-By: Marker <marker@example.invalid>\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := coveragedrop.DropTrailers(t.Context(), t.TempDir(), tc.message)
			if err != nil {
				t.Fatalf("DropTrailers error = %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("DropTrailers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDropTrailers_gitFails(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := coveragedrop.DropTrailers(ctx, t.TempDir(), "x\n\nCoverage-Drop: a - b\n"); err == nil {
		t.Error("DropTrailers with git unable to run succeeded, want an error")
	}
}
