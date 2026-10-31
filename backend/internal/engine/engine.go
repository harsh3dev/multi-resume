// Package engine applies ops to a draft: patch the .tex, mirror the op on
// the tree, re-parse, carry IDs, and compare. Any mismatch rejects the whole
// batch and the old draft stays untouched (data-model §4, BUILD-PLAN §7.4).
package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
)

// ApplyResult is the outcome of an accepted batch.
type ApplyResult struct {
	Source string
	Tree   *model.Resume
}

// corruptForTest, when set by tests, rewrites the patched source between
// application and verification. It proves corrupted ops are rejected.
var corruptForTest func(src string) string

func sourceHash(src string) string {
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:])
}

// ApplyOps is the only way anything (UI or AI) mutates a draft. It returns
// the new source and tree, or rejects the whole batch: the inputs are never
// mutated on failure.
func ApplyOps(src string, tree *model.Resume, ops []model.Op) (*ApplyResult, error) {
	for i, op := range ops {
		if err := op.Validate(); err != nil {
			return nil, fmt.Errorf("op %d (%s): %w", i, op.Kind, err)
		}
	}
	// Stale or missing tree: re-parse first and carry old IDs over by
	// structural position (data-model §4 step 1).
	if tree == nil || tree.SourceHash != sourceHash(src) {
		reparsed, err := parser.Parse(src, parser.ParseOptions{})
		if err != nil {
			return nil, fmt.Errorf("cannot parse source: %w", err)
		}
		if tree != nil {
			carryIDs(tree, reparsed)
		}
		tree = reparsed
	}
	curSrc, curTree := src, tree
	for i, op := range ops {
		pl, err := planOp(curSrc, curTree, op)
		if err != nil {
			return nil, fmt.Errorf("op %d (%s): %w", i, op.Kind, err)
		}
		newSrc, err := applyPatches(curSrc, pl.patches)
		if err != nil {
			return nil, fmt.Errorf("op %d (%s): %w", i, op.Kind, err)
		}
		if corruptForTest != nil {
			newSrc = corruptForTest(newSrc)
		}
		expected, err := deepCopy(curTree)
		if err != nil {
			return nil, fmt.Errorf("op %d (%s): %w", i, op.Kind, err)
		}
		if err := pl.mirror(expected); err != nil {
			return nil, fmt.Errorf("op %d (%s): %w", i, op.Kind, err)
		}
		reparsed, err := parser.Parse(newSrc, parser.ParseOptions{})
		if err != nil {
			return nil, fmt.Errorf("op %d (%s): result does not parse: %w", i, op.Kind, err)
		}
		carryIDs(expected, reparsed)
		if !treesEqual(expected, reparsed) {
			return nil, fmt.Errorf("op %d (%s) rejected: re-parsed tree does not match the expected tree", i, op.Kind)
		}
		curSrc, curTree = newSrc, reparsed
	}
	return &ApplyResult{Source: curSrc, Tree: curTree}, nil
}
