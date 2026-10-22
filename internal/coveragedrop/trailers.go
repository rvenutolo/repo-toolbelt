package coveragedrop

import (
	"context"
	"strings"
)

// trailerKey is the trailer that records why a commit raises a unit's count.
const trailerKey = "coverage-drop"

// Trailer is one `Coverage-Drop: <unit> - <reason>` trailer.
type Trailer struct {
	// Unit is the baseline unit the trailer names.
	Unit string
	// Reason is why the count rises; empty when the trailer gives none.
	Reason string
}

// DropTrailers returns the Coverage-Drop trailers of a commit message, in
// order. Only the message's trailer block counts, as git itself reads it, so
// a Coverage-Drop line in a body paragraph is not a trailer. The key is
// case-insensitive and a folded value is unfolded. The `-` between the unit
// and the reason is optional. git runs in dir.
func DropTrailers(ctx context.Context, dir, message string) ([]Trailer, error) {
	out, err := gitInput(ctx, dir, message, "interpret-trailers", "--parse")
	if err != nil {
		return nil, err
	}
	var trailers []Trailer
	for line := range strings.Lines(out) {
		key, value, ok := strings.Cut(strings.TrimSuffix(line, "\n"), ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), trailerKey) {
			continue
		}
		unit, reason, _ := strings.Cut(strings.TrimSpace(value), " ")
		if unit == "" {
			continue
		}
		reason = strings.TrimSpace(reason)
		reason = strings.TrimSpace(strings.TrimPrefix(reason, "-"))
		trailers = append(trailers, Trailer{Unit: unit, Reason: reason})
	}
	return trailers, nil
}
