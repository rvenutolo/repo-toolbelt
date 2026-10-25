package coverage

import (
	"context"
	"slices"

	"github.com/rvenutolo/repo-toolbelt/internal/vendored"
)

// WithoutVendored returns reports, in order, without the blocks in vendored
// files: those under a vendor directory and those git marks linguist-vendored
// in the checkout at root, asked once for every file the reports name. A repo
// does not own vendored code, so the ratchet never counts it. A git failure
// is an error.
func WithoutVendored(ctx context.Context, root string, reports []Report) ([]Report, error) {
	var files []string
	for _, r := range reports {
		for _, b := range r.Blocks {
			files = append(files, b.Path)
		}
	}
	slices.Sort(files)
	own, err := vendored.Filter(ctx, root, slices.Compact(files))
	if err != nil {
		return nil, err
	}
	kept := make([]Report, len(reports))
	for i, r := range reports {
		kept[i] = Report{Blocks: slices.DeleteFunc(slices.Clone(r.Blocks), func(b Block) bool {
			_, found := slices.BinarySearch(own, b.Path)
			return !found
		})}
	}
	return kept, nil
}
