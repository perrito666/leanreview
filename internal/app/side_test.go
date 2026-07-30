package app

import (
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
)

// pairedRowIndex finds the split row that carries both a deletion and an
// addition (the paired del/add row of the first hunk).
func pairedRowIndex(t *testing.T, m *Model) int {
	t.Helper()
	for i, r := range m.rows() {
		if r.Source != nil && r.AltSource != nil &&
			r.Left != nil && r.Left.Kind == diff.LineDeletion &&
			r.Right != nil && r.Right.Kind == diff.LineAddition {
			return i
		}
	}
	t.Fatal("no paired del/add row found in split layout")
	return 0
}

func TestSplitCursorCarriesSide(t *testing.T) {
	m := testModel(t)
	m = key(m, "t") // split layout
	if m.layout != LayoutSplit {
		t.Fatal("not in split layout")
	}
	m.cursor = pairedRowIndex(t, m)

	// Default (right side): the addition anchors the location.
	loc, _, err := m.buildLocation()
	if err != nil {
		t.Fatalf("right-side buildLocation: %v", err)
	}
	if loc.Side != diff.SideRight {
		t.Fatalf("default side = %v, want RIGHT", loc.Side)
	}

	// h switches to the left side: the deletion anchors the location.
	m = key(m, "h")
	loc, snippet, err := m.buildLocation()
	if err != nil {
		t.Fatalf("left-side buildLocation: %v", err)
	}
	if loc.Side != diff.SideLeft {
		t.Errorf("after h, side = %v, want LEFT", loc.Side)
	}
	if !strings.Contains(snippet, "result := calculate(input)") {
		t.Errorf("left-side snippet should be the deleted text, got %q", snippet)
	}

	// l switches back.
	m = key(m, "l")
	loc, _, err = m.buildLocation()
	if err != nil {
		t.Fatalf("back-to-right buildLocation: %v", err)
	}
	if loc.Side != diff.SideRight {
		t.Errorf("after l, side = %v, want RIGHT", loc.Side)
	}
}

func TestSideKeysHintInUnified(t *testing.T) {
	m := testModel(t) // unified layout
	m = key(m, "h")
	if !strings.Contains(m.status, "split view") {
		t.Errorf("h in unified mode should hint at split view, status=%q", m.status)
	}
	// And the side must not silently change behavior in unified mode.
	loc, _, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	if loc.Side != m.rowAt(m.cursor).Source.Side {
		t.Errorf("unified side should come from the row itself")
	}
}

func TestLeftCommentMarkerOnPairedRow(t *testing.T) {
	m := testModel(t)
	m = key(m, "t")
	m.cursor = pairedRowIndex(t, m)
	m = key(m, "h")
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	m.draft.Add(loc, "old side was fine", snip)
	if got := m.commentIDsAt(m.cursor); len(got) != 1 {
		t.Errorf("left-side comment should mark the paired row, got %d ids", len(got))
	}
}
