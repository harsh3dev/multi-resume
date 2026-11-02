package parser

import (
	"regexp"
	"sort"
	"strings"
)

// Hidden content is stored as a block between two marker lines, with every
// line inside commented out:
//
//	%%HIDE-START b_8a1d9f3c
//	% \resumeItem{
//	%   Built backend APIs
//	% }
//	%%HIDE-END b_8a1d9f3c
//
// The parser reads the block as one hidden node and parses its inside
// normally. The id after HIDE-START becomes the id of the first node in the
// block, so a hidden node keeps its identity across re-parses. Showing the
// block removes the markers and one comment layer, restoring the exact bytes.

// HideStart and HideEnd build marker lines.
const (
	HideStart = "%%HIDE-START "
	HideEnd   = "%%HIDE-END "
)

var markerPattern = regexp.MustCompile(`^[ \t]*(?:%[ \t]*)*%%HIDE-(START|END) ([a-z]+_[0-9a-f]{8,32})[ \t\r]*$`)

// hideBlock is one marked block. Start/End are the byte range of the content
// lines (first content byte .. end of last content line, excluding its
// newline); MarkStart/MarkEnd cover the marker lines as well.
type hideBlock struct {
	ID                 string
	Start, End         int
	MarkStart, MarkEnd int
	Depth              int // number of enclosing blocks including this one
	used               bool
}

// hideState carries what the body parser needs about hidden blocks.
type hideState struct {
	orig   string
	blocks []*hideBlock
	// Mask is the source with hidden block content blanked, for judging what
	// visible text exists. Parse is the source with comment layers inside
	// blocks removed, so hidden nodes are parsed like any others.
	Mask string
}

func (h *hideState) inBlock(pos int) bool {
	if h == nil {
		return false
	}
	for _, b := range h.blocks {
		if pos >= b.Start && pos <= b.End {
			return true
		}
	}
	return false
}

// takeID returns the marker id for the first node with the given prefix that
// starts in a block, once per block.
func (h *hideState) takeID(prefix string, pos int) string {
	if h != nil {
		for _, b := range h.blocks {
			if !b.used && pos >= b.Start && pos <= b.End && strings.HasPrefix(b.ID, prefix+"_") {
				b.used = true
				return b.ID
			}
		}
	}
	return newID(prefix)
}

// findHideBlocks pairs START/END marker lines. Unpaired or id-mismatched
// markers are ignored (the lines stay ordinary comments), and a repeated id
// is ignored so that node ids stay unique.
func findHideBlocks(src string) []*hideBlock {
	type open struct {
		id        string
		markStart int
		start     int
	}
	var stack []open
	var blocks []*hideBlock
	seen := map[string]bool{}
	pos := 0
	for pos <= len(src) {
		end := strings.IndexByte(src[pos:], '\n')
		lineEnd := len(src)
		next := len(src) + 1
		if end >= 0 {
			lineEnd = pos + end
			next = lineEnd + 1
		}
		if m := markerPattern.FindStringSubmatch(src[pos:lineEnd]); m != nil {
			if m[1] == "START" {
				stack = append(stack, open{id: m[2], markStart: pos, start: next})
			} else if n := len(stack); n > 0 && stack[n-1].id == m[2] {
				o := stack[n-1]
				stack = stack[:n-1]
				if !seen[o.id] {
					seen[o.id] = true
					contentEnd := pos - 1 // newline before the END marker
					if contentEnd < o.start {
						contentEnd = o.start
					}
					blocks = append(blocks, &hideBlock{ID: o.id, Start: o.start, End: contentEnd, MarkStart: o.markStart, MarkEnd: lineEnd})
				}
			}
		}
		pos = next
	}
	for _, b := range blocks {
		for _, o := range blocks {
			if o != b && o.MarkStart < b.MarkStart && b.MarkEnd < o.MarkEnd {
				b.Depth++
			}
		}
		b.Depth++
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].Start < blocks[j].Start })
	return blocks
}

// prepareHide builds the parse source and the mask (same length as src, so
// every offset is identical in all three) and the block list.
func prepareHide(src string) (parse string, hs *hideState) {
	if !strings.Contains(src, "%%HIDE-") {
		return src, nil
	}
	blocks := findHideBlocks(src)
	if len(blocks) == 0 {
		return src, nil
	}
	p, m := []byte(src), []byte(src)
	blank := func(b []byte, from, to int) {
		for i := from; i < to; i++ {
			if b[i] != '\n' && b[i] != '\r' {
				b[i] = ' '
			}
		}
	}
	for _, b := range blocks {
		blank(p, b.MarkStart, b.Start)
		blank(m, b.MarkStart, b.MarkEnd)
		blank(m, b.Start, b.End)
		blank(p, b.End, b.MarkEnd)
	}
	// Strip one comment layer per enclosing block from each content line.
	pos := 0
	for pos <= len(src) {
		end := strings.IndexByte(src[pos:], '\n')
		lineEnd := len(src)
		next := len(src) + 1
		if end >= 0 {
			lineEnd = pos + end
			next = lineEnd + 1
		}
		depth := 0
		for _, b := range blocks {
			if pos >= b.Start && pos <= b.End && !markerPattern.MatchString(src[pos:lineEnd]) {
				depth++
			}
		}
		i := pos
		for ; depth > 0 && i < lineEnd; depth-- {
			j := i
			for j < lineEnd && (src[j] == ' ' || src[j] == '\t') {
				j++
			}
			if j >= lineEnd || src[j] != '%' {
				break
			}
			cut := j + 1
			if cut < lineEnd && src[cut] == ' ' {
				cut++
			}
			// Only the comment mark (and one space) is blanked; indentation stays.
			blank(p, j, cut)
			i = cut
		}
		pos = next
	}
	return string(p), &hideState{orig: src, blocks: blocks, Mask: string(m)}
}

// Shadow returns src with hidden blocks parsed-ready: marker lines blanked
// and one comment layer removed inside each block. Offsets are unchanged, so
// it can be used to read the text behind a span of the parsed tree.
func Shadow(src string) string {
	p, _ := prepareHide(src)
	return p
}

// MarkerLine reports whether a line is a HIDE-START or HIDE-END marker and
// returns its kind ("START" or "END") and id. Comment layers added by an
// enclosing hidden block are allowed in front of the marker.
func MarkerLine(line string) (kind, id string, ok bool) {
	m := markerPattern.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// lift moves the start of a node that begins its line inside a hidden block
// back to the comment mark, so a hidden node's span starts where a commented
// node's always did (at the % that hides it).
func (h *hideState) lift(shadow string, pos int) int {
	if !h.inBlock(pos) {
		return pos
	}
	ls := pos
	for ls > 0 && shadow[ls-1] != '\n' {
		ls--
	}
	for i := ls; i < pos; i++ {
		if shadow[i] != ' ' && shadow[i] != '\t' {
			return pos
		}
	}
	for i := ls; i < pos; i++ {
		if h.orig[i] == '%' {
			return i
		}
		if h.orig[i] != ' ' && h.orig[i] != '\t' {
			return pos
		}
	}
	return pos
}
