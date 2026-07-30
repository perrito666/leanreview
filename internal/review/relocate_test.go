package review

import (
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
)

func ctxFile(path string, start int, texts ...string) diff.FileDiff {
	h := diff.Hunk{Header: "@@"}
	for i, tx := range texts {
		n := start + i
		o, nn := n, n
		h.Lines = append(h.Lines, diff.DiffLine{Kind: diff.LineContext, Text: tx, OldLine: &o, NewLine: &nn})
	}
	return diff.FileDiff{OldPath: path, NewPath: path, Hunks: []diff.Hunk{h}}
}

func locOn(f *diff.FileDiff, lineIndex int) diff.Location {
	l := f.Hunks[0].Lines[lineIndex]
	return diff.Location{
		Path:      f.Path(),
		Side:      diff.SideRight,
		StartLine: *l.NewLine,
		EndLine:   *l.NewLine,
		LineIndex: lineIndex,
		Anchor:    diff.NewContextAnchor(f, 0, lineIndex, 3),
	}
}

func TestRelocateDrafts(t *testing.T) {
	old := ctxFile("f.go", 10, "A", "B", "C", "D", "E", "F", "G", "H", "I", "J")
	d := NewDraftReview("k", "t", "oldhead")

	// A comment that will move (context intact), and one that will be orphaned
	// (its anchor line is replaced, far from the first comment's context).
	d.Add(locOn(&old, 2), "on C", "C") // moves
	d.Add(locOn(&old, 7), "on H", "H") // H replaced in new diff -> orphan

	// A reply — must be left untouched by relocation.
	replyTo := int64(99)
	d.Add(locOn(&old, 1), "reply", "B")
	d.Comments[2].ReplyTo = &replyTo

	// Shift by 10 and replace H (index 7) with X; C's neighborhood is intact.
	newFiles := []diff.FileDiff{ctxFile("f.go", 20, "A", "B", "C", "D", "E", "F", "G", "X", "I", "J")}

	s := RelocateDrafts(d, newFiles, "newhead")

	if s.Moved != 1 || s.Orphaned != 1 {
		t.Errorf("summary = %+v, want 1 moved 1 orphaned", s)
	}
	if !s.Changed() {
		t.Errorf("summary should report a change")
	}
	if d.HeadOID != "newhead" {
		t.Errorf("head not updated: %q", d.HeadOID)
	}

	// Comment on C moved to line 22 and is active.
	if d.Comments[0].State != DraftActive || d.Comments[0].Location.StartLine != 22 {
		t.Errorf("moved comment = state %v line %d", d.Comments[0].State, d.Comments[0].Location.StartLine)
	}
	// Comment on D orphaned, location unchanged.
	if d.Comments[1].State != DraftOrphaned {
		t.Errorf("expected orphaned, got %v", d.Comments[1].State)
	}
	// Reply untouched (still active, still a reply).
	if d.Comments[2].ReplyTo == nil {
		t.Errorf("reply lost its ReplyTo")
	}
}
