package coverage

import "path"

// Units counts the uncovered statements in each unit of r: the directory of
// each file's repo-relative path, `.` for the root. For a Go profile that is
// the package's import path relative to the module; for Cobertura it is the
// directory, counted in lines. Every unit with a block is present, at 0 when
// all of it is covered.
func Units(r Report) map[string]int {
	units := map[string]int{}
	for _, b := range r.Blocks {
		unit := path.Dir(b.Path)
		if b.Covered {
			units[unit] += 0
			continue
		}
		units[unit] += b.Statements
	}
	return units
}

// Merge combines reports of one format, as for two test runs over the same
// code: a block at the same position in several is one block, covered when
// any covers it, with the statement count of its first listing.
func Merge(reports ...Report) Report {
	blocks := map[blockKey]*Block{}
	for _, r := range reports {
		for _, b := range r.Blocks {
			key := blockKey{b.Path, b.StartLine, b.StartCol, b.EndLine, b.EndCol}
			if seen, ok := blocks[key]; ok {
				seen.Covered = seen.Covered || b.Covered
				continue
			}
			blocks[key] = &b
		}
	}
	return sortedReport(blocks)
}

// Tally counts the uncovered statements per unit across reports, where
// reports[i] was read as specs[i]. Reports of one format are merged first, so
// two runs over the same code count each block once.
func Tally(specs []ReportSpec, reports []Report) Baseline {
	byFormat := map[string][]Report{}
	for i, spec := range specs {
		byFormat[spec.Format] = append(byFormat[spec.Format], reports[i])
	}
	tally := Baseline{}
	for format, rs := range byFormat {
		for unit, n := range Units(Merge(rs...)) {
			tally[Unit{Format: format, Name: unit}] = n
		}
	}
	return tally
}
