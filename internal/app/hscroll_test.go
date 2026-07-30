package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/ui"
)

// longLineModel builds a model whose single file has a line wider than the view.
func longLineModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	long := "const s = \"" + strings.Repeat("abcdefgh ", 20) + "\""
	n1, n2 := 1, 1
	f := diff.FileDiff{
		OldPath: "x.go", NewPath: "x.go",
		Hunks: []diff.Hunk{{
			Header: "@@ -1 +1 @@",
			Lines: []diff.DiffLine{
				{Kind: diff.LineContext, Text: long, OldLine: &n1, NewLine: &n2},
			},
		}},
	}
	m := New(Config{Files: []diff.FileDiff{f}, Title: "t", Theme: ui.DefaultTheme()})
	m.width, m.height = 40, 10
	return m
}

func TestHorizontalScrollShiftsText(t *testing.T) {
	m := longLineModel(t)
	m.cursor = 1 // the content line
	before := m.View()

	m.scrollRight()
	m.scrollRight()
	after := m.View()

	if before == after {
		t.Fatalf("scrolling right did not change the rendered view")
	}
	// Line start returns to the original view.
	m.lineStart()
	if m.View() != before {
		t.Errorf("line-start did not restore the unscrolled view")
	}
}

func TestScrollClampsAndEnd(t *testing.T) {
	m := longLineModel(t)
	m.scrollLeft() // already at 0, stays 0
	if m.hscroll != 0 {
		t.Errorf("scroll-left past 0 = %d, want 0", m.hscroll)
	}
	m.lineEnd()
	max := m.maxHScroll()
	if m.hscroll != max || max <= 0 {
		t.Errorf("line-end hscroll = %d, want max %d (>0)", m.hscroll, max)
	}
	m.scrollRight() // past end clamps
	if m.hscroll != max {
		t.Errorf("scroll past end = %d, want clamped %d", m.hscroll, max)
	}
}

func TestScrollKeepsFullWidthAndGutter(t *testing.T) {
	m := longLineModel(t)
	m.scrollRight()
	m.scrollRight()
	lines := strings.Split(m.View(), "\n")
	// Body lines stay full width.
	for i := 1; i < len(lines)-1; i++ {
		if w := lipgloss.Width(lines[i]); w != 40 {
			t.Fatalf("body line %d width = %d, want 40", i, w)
		}
	}
	// The line-number gutter (…" 1  1 ") is still present on the content row.
	if !strings.Contains(m.View(), " 1 ") {
		t.Errorf("number gutter should stay fixed while text scrolls")
	}
}
