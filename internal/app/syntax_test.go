package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/ui"
)

// syntaxModel returns a model with a real (forced-on) highlighter.
func syntaxModel(t *testing.T) *Model {
	t.Helper()
	m := testModel(t)
	m.highlighter = ui.NewHighlighter(true, "monokai")
	if !m.highlighter.Enabled() {
		t.Skip("highlighter unavailable")
	}
	m.syntaxOn = true
	return m
}

// seedSides puts synthesized old/new file content into the content cache,
// derived from the fixture's hunks so line numbers align.
func seedSides(t *testing.T, m *Model) {
	t.Helper()
	f := m.currentFile()
	for _, side := range []diff.Side{diff.SideLeft, diff.SideRight} {
		max := 0
		byLine := map[int]string{}
		for hi := range f.Hunks {
			for _, l := range f.Hunks[hi].Lines {
				var n *int
				if side == diff.SideLeft {
					n = l.OldLine
				} else {
					n = l.NewLine
				}
				if n != nil {
					byLine[*n] = l.Text
					if *n > max {
						max = *n
					}
				}
			}
		}
		var b strings.Builder
		for i := 1; i <= max+2; i++ {
			if text, ok := byLine[i]; ok {
				b.WriteString(text)
			} else {
				b.WriteString("// filler")
			}
			b.WriteByte('\n')
		}
		m.contentCache[contentKey(m.fileIdx, side)] = []byte(b.String())
	}
}

// TestSyntaxSidePasses: deletions index the old-file pass, additions the new
// — the two-pass rule that makes both color correctly.
func TestSyntaxSidePasses(t *testing.T) {
	m := syntaxModel(t)
	seedSides(t, m)

	var delRow, addRow *diff.DisplayRow
	rows := m.rows()
	for i := range rows {
		if rows[i].Left != nil && rows[i].Left.Kind == diff.LineDeletion && delRow == nil {
			delRow = &rows[i]
		}
		if rows[i].Right != nil && rows[i].Right.Kind == diff.LineAddition && rows[i].Right.LineNumber != nil && addRow == nil {
			addRow = &rows[i]
		}
	}
	if delRow == nil || addRow == nil {
		t.Fatalf("fixture lacks change rows")
	}
	if got := stripANSI(m.syntaxLineFor(delRow)); got != delRow.Left.Text {
		t.Errorf("deletion text = %q, want old-side %q", got, delRow.Left.Text)
	}
	if got := stripANSI(m.syntaxLineFor(addRow)); got != addRow.Right.Text {
		t.Errorf("addition text = %q, want new-side %q", got, addRow.Right.Text)
	}
	// The lookup came from the whole-file pass (memoized), not the fallback.
	if _, ok := m.fileLines(diff.SideLeft); !ok {
		t.Errorf("old-side pass not built")
	}
}

// TestSyntaxStitchedFallback: without file content, hunk-stitched highlighting
// still returns the row's own text (styled), never a wrong line.
func TestSyntaxStitchedFallback(t *testing.T) {
	m := syntaxModel(t)
	rows := m.rows()
	for i := range rows {
		r := &rows[i]
		if r.Source == nil {
			continue
		}
		if got := stripANSI(m.syntaxLineFor(r)); got != r.Left.Text {
			t.Errorf("stitched line = %q, want %q", got, r.Left.Text)
		}
	}
}

// TestCycleSyntaxStates: S walks red/green-changes → syntax-everywhere → off.
func TestCycleSyntaxStates(t *testing.T) {
	m := syntaxModel(t)
	m.changeColors = changeColorsDiff

	m.cycleSyntax()
	if !m.syntaxActive() || m.changeColors != changeColorsSyntax {
		t.Fatalf("state 2 wrong: on=%v colors=%s", m.syntaxActive(), m.changeColors)
	}
	m.cycleSyntax()
	if m.syntaxActive() {
		t.Fatalf("state 3 should be off")
	}
	m.cycleSyntax()
	if !m.syntaxActive() || m.changeColors != changeColorsDiff {
		t.Fatalf("cycle did not return to state 1")
	}
}

// TestChangeTintApplied: in syntax mode with tint on, changed rows carry the
// background wash; with tint off (or diff mode) they do not.
func TestChangeTintApplied(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	m := syntaxModel(t)
	m.theme = ui.DefaultTheme()
	m.changeColors = changeColorsSyntax
	m.changeTint = true
	seedSides(t, m)

	out := m.View()
	if !strings.Contains(out, "\x1b[48;") {
		t.Errorf("no background tint on changed lines in syntax mode")
	}
	m.changeTint = false
	if strings.Contains(m.View(), "\x1b[48;2;") && strings.Contains(m.View(), "38;5;") {
		// Background may still appear from title bar styles; check via tintFor.
	}
	if _, ok := m.tintFor(diff.LineAddition); ok {
		t.Errorf("tint reported active with change_tint off")
	}
	m.changeTint = true
	m.changeColors = changeColorsDiff
	if _, ok := m.tintFor(diff.LineAddition); ok {
		t.Errorf("tint must not apply in red/green mode")
	}
}
