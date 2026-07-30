package diff

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Location is a semantic, view-independent position in a diff. It is the
// canonical anchor for a review comment: it survives terminal resize, context
// folding, and switching between unified and split layouts, and it maps cleanly
// onto GitHub's line-oriented review-comment fields (path, side, line,
// start_line, start_side).
type Location struct {
	Path string

	// CommitOID is the head commit the location was captured against, when
	// known (empty in pure patch-file mode).
	CommitOID string

	Side Side

	// StartLine and EndLine are 1-based line numbers on Side, inclusive.
	StartLine int
	EndLine   int

	// HunkIndex and LineIndex point at the anchor line within a specific
	// FileDiff (the start of the selection). They are convenient for rendering
	// but are always re-derivable from Path+Side+StartLine.
	HunkIndex int
	LineIndex int

	// Anchor captures surrounding content so the comment can be relocated when
	// the underlying diff changes (e.g. the PR is updated).
	Anchor ContextAnchor
}

// Single reports whether the location covers exactly one line.
func (l Location) Single() bool { return l.StartLine == l.EndLine }

// ContextAnchor records enough surrounding content to relocate a comment after
// the diff changes: the hunk header, a few lines of context around the anchor,
// and a hash of the anchored content.
type ContextAnchor struct {
	HunkHeader  string
	Before      []string
	AnchorText  string
	After       []string
	ContentHash string
}

// NewContextAnchor builds an anchor for the line at (hunkIndex, lineIndex) in f,
// capturing up to n lines of context on each side.
func NewContextAnchor(f *FileDiff, hunkIndex, lineIndex, n int) ContextAnchor {
	a := ContextAnchor{}
	if hunkIndex < 0 || hunkIndex >= len(f.Hunks) {
		return a
	}
	h := f.Hunks[hunkIndex]
	a.HunkHeader = h.Header
	if lineIndex < 0 || lineIndex >= len(h.Lines) {
		return a
	}
	a.AnchorText = h.Lines[lineIndex].Text
	for i := lineIndex - n; i < lineIndex; i++ {
		if i >= 0 {
			a.Before = append(a.Before, h.Lines[i].Text)
		}
	}
	for i := lineIndex + 1; i <= lineIndex+n && i < len(h.Lines); i++ {
		a.After = append(a.After, h.Lines[i].Text)
	}
	sum := sha256.Sum256([]byte(strings.Join(append(append(append([]string{}, a.Before...), a.AnchorText), a.After...), "\n")))
	a.ContentHash = hex.EncodeToString(sum[:8])
	return a
}
