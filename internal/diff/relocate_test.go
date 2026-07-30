package diff

import "testing"

// ctxFile builds a single-hunk file of context lines numbered from start on both
// sides, one per text.
func ctxFile(oldPath, newPath string, start int, texts ...string) FileDiff {
	h := Hunk{Header: "@@"}
	for i, tx := range texts {
		n := start + i
		o, nn := n, n
		h.Lines = append(h.Lines, DiffLine{Kind: LineContext, Text: tx, OldLine: &o, NewLine: &nn})
	}
	return FileDiff{OldPath: oldPath, NewPath: newPath, Hunks: []Hunk{h}}
}

// anchorLoc captures a Location on the new side at the given line index of f.
func anchorLoc(f *FileDiff, lineIndex int) Location {
	l := f.Hunks[0].Lines[lineIndex]
	return Location{
		Path:      f.Path(),
		Side:      SideRight,
		StartLine: *l.NewLine,
		EndLine:   *l.NewLine,
		HunkIndex: 0,
		LineIndex: lineIndex,
		Anchor:    NewContextAnchor(f, 0, lineIndex, anchorContext),
	}
}

func TestRelocateExact(t *testing.T) {
	old := ctxFile("f.go", "f.go", 10, "A", "B", "C", "D", "E")
	loc := anchorLoc(&old, 2) // "C" at line 12
	newFiles := []FileDiff{ctxFile("f.go", "f.go", 10, "A", "B", "C", "D", "E")}
	got, res := Relocate(newFiles, loc)
	if res != RelocateExact {
		t.Fatalf("res = %v, want Exact", res)
	}
	if got.StartLine != 12 {
		t.Errorf("start line = %d, want 12", got.StartLine)
	}
}

func TestRelocateMovedByShift(t *testing.T) {
	old := ctxFile("f.go", "f.go", 10, "A", "B", "C", "D", "E")
	loc := anchorLoc(&old, 2) // "C" at line 12
	// Same content, shifted down by 10 (e.g. lines inserted above).
	newFiles := []FileDiff{ctxFile("f.go", "f.go", 20, "A", "B", "C", "D", "E")}
	got, res := Relocate(newFiles, loc)
	if res != RelocateMoved {
		t.Fatalf("res = %v, want Moved", res)
	}
	if got.StartLine != 22 {
		t.Errorf("relocated line = %d, want 22", got.StartLine)
	}
}

func TestRelocateMovedPreservesSpan(t *testing.T) {
	old := ctxFile("f.go", "f.go", 10, "A", "B", "C", "D", "E")
	loc := anchorLoc(&old, 1) // start at "B" line 11
	loc.EndLine = 13          // range B..D (span 2)
	newFiles := []FileDiff{ctxFile("f.go", "f.go", 30, "A", "B", "C", "D", "E")}
	got, res := Relocate(newFiles, loc)
	if res != RelocateMoved {
		t.Fatalf("res = %v, want Moved", res)
	}
	if got.StartLine != 31 || got.EndLine != 33 {
		t.Errorf("relocated range = %d-%d, want 31-33", got.StartLine, got.EndLine)
	}
}

func TestRelocateOrphanedWhenMissing(t *testing.T) {
	old := ctxFile("f.go", "f.go", 10, "A", "B", "C", "D", "E")
	loc := anchorLoc(&old, 2)
	newFiles := []FileDiff{ctxFile("f.go", "f.go", 10, "A", "B", "X", "D", "E")} // C removed
	_, res := Relocate(newFiles, loc)
	if res != RelocateOrphaned {
		t.Errorf("res = %v, want Orphaned", res)
	}
}

func TestRelocateOrphanedWhenAmbiguous(t *testing.T) {
	old := ctxFile("f.go", "f.go", 10, "A", "B", "C", "D", "E")
	loc := anchorLoc(&old, 2) // "C"
	// New file has two context-free "C" lines in separate hunks: both match the
	// anchor (empty overlap), so the match is not unique.
	f := FileDiff{OldPath: "f.go", NewPath: "f.go"}
	n1, n2 := 5, 40
	f.Hunks = []Hunk{
		{Header: "@@", Lines: []DiffLine{{Kind: LineContext, Text: "C", OldLine: &n1, NewLine: &n1}}},
		{Header: "@@", Lines: []DiffLine{{Kind: LineContext, Text: "C", OldLine: &n2, NewLine: &n2}}},
	}
	_, res := Relocate([]FileDiff{f}, loc)
	if res != RelocateOrphaned {
		t.Errorf("res = %v, want Orphaned (ambiguous)", res)
	}
}

func TestRelocateFollowsRename(t *testing.T) {
	old := ctxFile("old.go", "old.go", 10, "A", "B", "C", "D", "E")
	loc := anchorLoc(&old, 2) // path "old.go"
	// File renamed to new.go, content shifted.
	renamed := ctxFile("old.go", "new.go", 20, "A", "B", "C", "D", "E")
	renamed.Status = StatusRenamed
	got, res := Relocate([]FileDiff{renamed}, loc)
	if res != RelocateMoved {
		t.Fatalf("res = %v, want Moved", res)
	}
	if got.Path != "new.go" {
		t.Errorf("path = %q, want new.go (updated on rename)", got.Path)
	}
	if got.StartLine != 22 {
		t.Errorf("line = %d, want 22", got.StartLine)
	}
}
