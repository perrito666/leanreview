package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/ui"
)

// wrapModel builds a model with a long line and wrapping enabled at a narrow
// width so the wrap point is exercised deterministically.
func wrapModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	long := "const value = \"" + strings.Repeat("word ", 40) + "\""
	n1, n2 := 1, 1
	n3 := 2
	f := diff.FileDiff{
		OldPath: "x.go", NewPath: "x.go",
		Hunks: []diff.Hunk{{
			Header: "@@ -1,2 +1,2 @@",
			Lines: []diff.DiffLine{
				{Kind: diff.LineContext, Text: long, OldLine: &n1, NewLine: &n2},
				{Kind: diff.LineContext, Text: "short", OldLine: &n3, NewLine: &n3},
			},
		}},
	}
	m := New(Config{Files: []diff.FileDiff{f}, Title: "t", Theme: ui.DefaultTheme(), Wrap: true, WrapWidth: 120})
	m.width, m.height = 60, 20
	return m
}

func TestWrapProducesContinuations(t *testing.T) {
	m := wrapModel(t)
	rows := m.rows()
	var content, cont int
	for _, r := range rows {
		if r.Continuation {
			cont++
		} else if r.Source != nil {
			content++
		}
	}
	if content != 2 {
		t.Errorf("content rows = %d, want 2 (one per logical line)", content)
	}
	if cont < 2 {
		t.Errorf("expected several continuation rows for the long line, got %d", cont)
	}
	// Continuations carry neither Source nor line numbers.
	for _, r := range rows {
		if r.Continuation && (r.Source != nil || (r.Left != nil && r.Left.LineNumber != nil)) {
			t.Errorf("continuation carries identity: %+v", r)
		}
	}
	// All view lines stay exactly terminal-width.
	for i, ln := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(ln); w != 60 {
			t.Fatalf("line %d width = %d, want 60", i, w)
		}
	}
}

func TestWrapToggleRestoresClipping(t *testing.T) {
	m := wrapModel(t)
	wrapped := len(m.rows())
	m = key(m, "w")
	if m.wrapText {
		t.Fatal("w did not toggle wrap off")
	}
	if got := len(m.rows()); got >= wrapped {
		t.Errorf("rows with wrap off = %d, want fewer than %d", got, wrapped)
	}
	m = key(m, "w")
	if len(m.rows()) != wrapped {
		t.Errorf("toggling back did not restore wrapped rows")
	}
}

func TestWrapWidthCapsUnified(t *testing.T) {
	m := wrapModel(t)
	m.width = 400 // terminal far wider than the cap
	m.wrapWidth = 40
	for _, r := range m.rows() {
		if r.Left != nil && lipgloss.Width(r.Left.Text) > 40 {
			t.Errorf("row text exceeds the configured wrap width: %q", r.Left.Text)
		}
	}
}

func TestCursorSkipsContinuations(t *testing.T) {
	m := wrapModel(t)
	m.cursor = m.firstContentRow() // the long line
	m = key(m, "j")
	r := m.rowAt(m.cursor)
	if r == nil || r.Continuation {
		t.Fatalf("cursor rested on a continuation row")
	}
	if r.Source == nil || r.Source.StartLine != 2 {
		t.Errorf("j should land on the next logical line, got %+v", r.Source)
	}
}

func TestCommentPreviewWordWraps(t *testing.T) {
	m := wrapModel(t)
	m.cursor = m.firstContentRow()
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	body := "This explanation is deliberately long so that the preview must wrap across rows " + strings.Repeat("because reasons ", 10)
	m.draft.Add(loc, body, snip)

	var annRows []string
	for _, r := range m.rows() {
		if r.Annotation {
			annRows = append(annRows, r.Left.Text)
		}
	}
	if len(annRows) < 2 {
		t.Fatalf("long comment should wrap to multiple annotation rows, got %d", len(annRows))
	}
	// Word wrapping: no annotation row is cut mid-word (each ends at a word
	// boundary; the joined text reconstructs the body words in order).
	joined := strings.Join(annRows, " ")
	if !strings.Contains(strings.Join(strings.Fields(joined), " "), "because reasons because reasons") {
		t.Errorf("wrapped preview lost body text:\n%s", joined)
	}

	// With wrap off, the preview collapses to a single truncated row.
	m = key(m, "w")
	ann := 0
	for _, r := range m.rows() {
		if r.Annotation {
			ann++
		}
	}
	if ann != 1 {
		t.Errorf("wrap off should show one preview row, got %d", ann)
	}
}

func TestSplitWrapUsesPanelWidth(t *testing.T) {
	m := wrapModel(t)
	m = key(m, "t") // split layout
	panel := m.splitPanelWidth()
	for _, r := range m.rows() {
		if r.Left != nil && lipgloss.Width(r.Left.Text) > panel {
			t.Errorf("split left cell exceeds panel width %d: %q", panel, r.Left.Text)
		}
		if r.Right != nil && lipgloss.Width(r.Right.Text) > panel {
			t.Errorf("split right cell exceeds panel width %d: %q", panel, r.Right.Text)
		}
	}
	// Rendered view still exactly terminal-width.
	for i, ln := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(ln); w != 60 {
			t.Fatalf("split line %d width = %d, want 60", i, w)
		}
	}
}
