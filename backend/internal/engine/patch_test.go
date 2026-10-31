package engine

import (
	"strings"
	"testing"
)

func TestApplyPatchesBasic(t *testing.T) {
	src := "hello world"
	out, err := applyPatches(src, []Patch{{Start: 6, End: 11, Replacement: "there"}})
	if err != nil || out != "hello there" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestApplyPatchesEndToStart(t *testing.T) {
	// Two patches: earlier offsets must not shift.
	src := "aXb Yc"
	out, err := applyPatches(src, []Patch{
		{Start: 1, End: 2, Replacement: "XX"},
		{Start: 4, End: 5, Replacement: "YY"},
	})
	if err != nil || out != "aXXb YYc" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestApplyPatchesAdjacent(t *testing.T) {
	src := "abcdef"
	out, err := applyPatches(src, []Patch{
		{Start: 0, End: 3, Replacement: "1"},
		{Start: 3, End: 6, Replacement: "2"},
	})
	if err != nil || out != "12" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestApplyPatchesOverlap(t *testing.T) {
	if _, err := applyPatches("abcdef", []Patch{
		{Start: 0, End: 4},
		{Start: 2, End: 5},
	}); err == nil {
		t.Fatal("expected overlap error")
	}
}

func TestApplyPatchesUnicodeByteOffsets(t *testing.T) {
	src := "héllo wörld" // é and ö are 2 bytes each in UTF-8
	start := strings.Index(src, "wörld")
	if start != 8 { // h(1) é(2) l l o space = 7 bytes... verified below
		t.Logf("wörld starts at byte %d", start)
	}
	out, err := applyPatches(src, []Patch{{Start: start, End: start + len("wörld"), Replacement: "there"}})
	if err != nil || out != "héllo there" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestApplyPatchesBounds(t *testing.T) {
	for _, p := range []Patch{{Start: -1, End: 0}, {Start: 0, End: 100}, {Start: 5, End: 2}} {
		if _, err := applyPatches("short", []Patch{p}); err == nil {
			t.Fatalf("expected bounds error for %+v", p)
		}
	}
}

func TestApplyPatchesInsertAtSamePointKeepsOrder(t *testing.T) {
	out, err := applyPatches("ab", []Patch{
		{Start: 1, End: 1, Replacement: "X"},
		{Start: 1, End: 1, Replacement: "Y"},
	})
	if err != nil || out != "aXYb" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestApplyPatchesDeleteInsertSameOp(t *testing.T) {
	// The move-op shape: cut one region, insert at a later point.
	src := "line1\nline2\nline3\n"
	out, err := applyPatches(src, []Patch{
		{Start: 0, End: 6},
		{Start: 12, End: 12, Replacement: "line1\n"},
	})
	if err != nil || out != "line2\nline1\nline3\n" {
		t.Fatalf("got %q, %v", out, err)
	}
}
