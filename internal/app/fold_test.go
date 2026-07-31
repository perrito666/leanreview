package app

import (
	"strings"
	"testing"
)

func countHeaders(m *Model) int {
	n := 0
	for i := range m.rows() {
		if isHeader(&m.rows()[i]) {
			n++
		}
	}
	return n
}

func TestFoldHidesHunkContent(t *testing.T) {
	m := testModel(t) // simple.diff has 2 hunks
	full := len(m.rows())
	headers := countHeaders(m)
	if headers != 2 {
		t.Fatalf("expected 2 hunk headers, got %d", headers)
	}

	// Fold the first hunk (cursor starts in it).
	m.cursor = 0
	m.toggleFold()
	folded := len(m.rows())
	if folded >= full {
		t.Errorf("folding did not reduce row count (%d -> %d)", full, folded)
	}
	// Header count is unchanged; both hunks still have a header row.
	if countHeaders(m) != 2 {
		t.Errorf("folding should keep hunk headers; got %d", countHeaders(m))
	}
	// The folded header shows a hidden-line count.
	if !strings.Contains(m.rows()[0].Left.Text, "lines") {
		t.Errorf("folded header not annotated: %q", m.rows()[0].Left.Text)
	}
}

func TestExpandAndCollapseAll(t *testing.T) {
	m := testModel(t)
	full := len(m.rows())

	m.collapseAll()
	if len(m.rows()) != countHeaders(m) {
		t.Errorf("collapseAll should leave only header rows, got %d rows / %d headers", len(m.rows()), countHeaders(m))
	}

	m.expandAll()
	if len(m.rows()) != full {
		t.Errorf("expandAll did not restore all rows (%d != %d)", len(m.rows()), full)
	}
}

func TestFoldTogglesBack(t *testing.T) {
	m := testModel(t)
	full := len(m.rows())
	m.cursor = 0
	m.toggleFold()
	m.cursor = m.headerRowIndex(0)
	m.toggleFold()
	if len(m.rows()) != full {
		t.Errorf("unfolding did not restore rows (%d != %d)", len(m.rows()), full)
	}
}

// TestHunkSeparatorRendersAndIsSkipped: without file context, hunk boundaries
// get a visible rule, and the cursor glides over it in both directions.
func TestHunkSeparatorRendersAndIsSkipped(t *testing.T) {
	m := testModel(t)
	if !strings.Contains(m.View(), "┄") {
		t.Fatalf("no separator between hunks:\n%s", m.View())
	}
	rows := m.rows()
	sep := -1
	for i := range rows {
		if rows[i].Separator {
			sep = i
			break
		}
	}
	if sep < 0 {
		t.Fatalf("no separator row in the projection")
	}
	// Land the cursor just before the separator and step over it.
	m.cursor = sep - 1
	m = key(m, "j")
	if m.cursor == sep || m.rowAt(m.cursor).Separator {
		t.Errorf("cursor rested on the separator going down")
	}
	m = key(m, "k")
	if m.rowAt(m.cursor).Separator {
		t.Errorf("cursor rested on the separator going up")
	}
}
