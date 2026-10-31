package engine

import (
	"errors"
	"fmt"
	"sort"
)

// Patch replaces src[Start:End] with Replacement. Offsets are byte offsets
// into the exact source being patched.
type Patch struct {
	Start       int
	End         int
	Replacement string
}

// applyPatches applies patches from the end of the file to the start so
// earlier positions do not shift. Overlapping patches are an error; adjacent
// patches (End == Start) are allowed. Same-position inserts keep their list
// order. All offsets are bytes, never runes.
func applyPatches(src string, ps []Patch) (string, error) {
	for i, p := range ps {
		if p.Start < 0 || p.End < p.Start || p.End > len(src) {
			return "", fmt.Errorf("patch %d out of bounds", i)
		}
	}
	sorted := make([]Patch, len(ps))
	copy(sorted, ps)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	for i := 0; i+1 < len(sorted); i++ {
		if sorted[i].End > sorted[i+1].Start {
			return "", errors.New("overlapping patches")
		}
	}
	out := src
	for i := len(sorted) - 1; i >= 0; i-- {
		p := sorted[i]
		out = out[:p.Start] + p.Replacement + out[p.End:]
	}
	return out, nil
}
