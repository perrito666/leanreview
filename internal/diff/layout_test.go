package diff

import "testing"

func TestRenderUnifiedRowsMatchLines(t *testing.T) {
	f := loadFixture(t, "simple.diff")[0]
	rows := RenderUnified(&f)

	var content, headers int
	for _, r := range rows {
		if r.Left != nil && r.Left.Kind == LineMetadata && r.Right == nil {
			headers++
			continue
		}
		content++
		if r.Source == nil {
			t.Errorf("content row %q has nil Source", r.Left.Text)
		}
	}
	if headers != len(f.Hunks) {
		t.Errorf("headers = %d, want %d", headers, len(f.Hunks))
	}
	var wantContent int
	for _, h := range f.Hunks {
		wantContent += len(h.Lines)
	}
	if content != wantContent {
		t.Errorf("content rows = %d, want %d", content, wantContent)
	}
}

func TestRenderSplitPairsDeletionsAndAdditions(t *testing.T) {
	f := loadFixture(t, "simple.diff")[0]
	rows := RenderSplit(&f)

	// The first hunk has 1 deletion and 2 additions; pairing yields one row
	// with both sides and one row with only the right side populated.
	var pairedBoth, rightOnly int
	for _, r := range rows {
		if r.Left != nil && r.Right != nil && r.Left.Kind == LineDeletion && r.Right.Kind == LineAddition {
			pairedBoth++
		}
		if r.Left == nil && r.Right != nil && r.Right.Kind == LineAddition {
			rightOnly++
		}
	}
	if pairedBoth != 1 {
		t.Errorf("paired del+add rows = %d, want 1", pairedBoth)
	}
	if rightOnly != 3 {
		// hunk 1 has one unpaired addition; hunk 2 has two pure additions.
		t.Errorf("right-only addition rows = %d, want 3", rightOnly)
	}
}

func TestLayoutInvariantAcrossToggle(t *testing.T) {
	// The same semantic line must be reachable in both layouts with a matching
	// Location, proving rows are projections and comments survive a toggle.
	f := loadFixture(t, "simple.diff")[0]
	uni := RenderUnified(&f)
	spl := RenderSplit(&f)

	find := func(rows []DisplayRow, side Side, line int) *Location {
		for _, r := range rows {
			if r.Source != nil && r.Source.Side == side && r.Source.StartLine == line {
				return r.Source
			}
		}
		return nil
	}
	u := find(uni, SideRight, 72)
	s := find(spl, SideRight, 72)
	if u == nil || s == nil {
		t.Fatalf("line missing: unified=%v split=%v", u, s)
	}
	if u.Path != s.Path || u.HunkIndex != s.HunkIndex || u.LineIndex != s.LineIndex {
		t.Errorf("location mismatch across layouts: %+v vs %+v", u, s)
	}
}
