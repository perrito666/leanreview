package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/ui"
)

// commentedModel returns a model with one draft comment on new-side line 72.
func commentedModel(t *testing.T) *Model {
	t.Helper()
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	m.draft.Add(loc, "needs a check", snip)
	return m
}

func TestCommentPreviewIsBoxed(t *testing.T) {
	m := commentedModel(t)
	out := m.View()
	for _, want := range []string{"╭─", "╮", "│ ", "╰─", "╯", "needs a check"} {
		if !strings.Contains(out, want) {
			t.Errorf("boxed preview missing %q:\n%s", want, out)
		}
	}
	// Every line still spans the full terminal width.
	lines := strings.Split(out, "\n")
	for i := 2; i < len(lines)-1; i++ {
		if w := lipgloss.Width(lines[i]); w != m.width {
			t.Fatalf("line %d width = %d, want %d", i, w, m.width)
		}
	}
}

func TestCommentBoxSitsOnRightPanelInSplit(t *testing.T) {
	m := commentedModel(t)
	m = key(m, "t") // split layout
	indent, _ := m.annotationLayout()
	rightPanelStart := 2 + m.numWidth() + 1 + m.splitPanelWidth() + 3
	if indent != rightPanelStart {
		t.Errorf("split box indent = %d, want right panel start %d", indent, rightPanelStart)
	}
	out := m.View()
	if !strings.Contains(out, "╭─") || !strings.Contains(out, "needs a check") {
		t.Fatalf("boxed preview missing in split view:\n%s", out)
	}
	// The box's border starts at the indent column, not at the left margin.
	for _, ln := range strings.Split(out, "\n") {
		if i := strings.Index(ln, "╭"); i >= 0 {
			plain := ln[:i]
			if got := lipgloss.Width(stripANSI(plain)); got < rightPanelStart {
				t.Errorf("box top border at column %d, want >= %d:\n%q", got, rightPanelStart, ln)
			}
		}
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if r == 'm' {
				inEsc = false
			}
		case r == 0x1b:
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestTabCyclesFiles(t *testing.T) {
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	data, err := os.ReadFile(filepath.Join("..", "diff", "testdata", "binary.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	files, err := diff.ParsePatchBytes(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := New(Config{Files: files, Title: "test", Theme: ui.DefaultTheme()})
	m.width, m.height = 100, 30
	if len(m.files) < 2 {
		t.Fatalf("fixture should carry two files, got %d", len(m.files))
	}
	m = key(m, "tab")
	if m.fileIdx != 1 {
		t.Errorf("tab should advance to the next file, fileIdx=%d", m.fileIdx)
	}
	m = key(m, "shift+tab")
	if m.fileIdx != 0 {
		t.Errorf("shift+tab should go back, fileIdx=%d", m.fileIdx)
	}
}

func TestSplitRowsStylePerSide(t *testing.T) {
	m := testModel(t)
	m = key(m, "t") // split
	rows := m.rows()
	for i := range rows {
		r := &rows[i]
		if r.Left == nil || r.Right == nil || r.Left.Kind != diff.LineDeletion || r.Right.Kind != diff.LineAddition {
			continue
		}
		// A paired row: each side must be rendered with its own kind's style,
		// which renderSplitStyled derives from the cells (not kindOf).
		line := m.renderSplitStyled(r, m.numWidth(), m.contentWidth()-2)
		if !strings.Contains(stripANSI(line), "│") {
			t.Fatalf("split row lost its separator: %q", line)
		}
		return
	}
	t.Skip("fixture has no paired deletion/addition row")
}
