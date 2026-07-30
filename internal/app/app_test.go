package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/ui"
)

func testModel(t *testing.T) *Model {
	t.Helper()
	// Keep rendered output plain so assertions can match source substrings.
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	data, err := os.ReadFile(filepath.Join("..", "diff", "testdata", "simple.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	files, err := diff.ParsePatchBytes(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := New(Config{Files: files, Title: "test", Theme: ui.DefaultTheme()})
	m.width, m.height = 100, 20
	return m
}

// key feeds a single key event into Update and returns the model.
func key(m *Model, k string) *Model {
	var msg tea.KeyMsg
	switch k {
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	next, _ := m.Update(msg)
	return next.(*Model)
}

func TestNextHunkCommand(t *testing.T) {
	m := testModel(t)
	// Cursor starts in hunk 0; ]c should land in hunk 1.
	startHunk := m.rowAt(m.cursor).Source.HunkIndex
	m = key(m, "]")
	m = key(m, "c")
	got := m.rowAt(m.cursor)
	if got == nil || got.Source == nil {
		t.Fatalf("cursor not on a content row after ]c")
	}
	if got.Source.HunkIndex == startHunk {
		t.Errorf("]c did not advance hunk (still %d)", startHunk)
	}
}

func TestToggleLayout(t *testing.T) {
	m := testModel(t)
	if m.layout != LayoutUnified {
		t.Fatalf("default layout should be unified")
	}
	m = key(m, "t")
	if m.layout != LayoutSplit {
		t.Errorf("t did not toggle to split")
	}
	m = key(m, "t")
	if m.layout != LayoutUnified {
		t.Errorf("t did not toggle back to unified")
	}
}

func TestCountPrefixMovement(t *testing.T) {
	m := testModel(t)
	start := m.cursor
	m = key(m, "3")
	m = key(m, "j")
	if m.cursor != start+3 {
		t.Errorf("3j moved to %d, want %d", m.cursor, start+3)
	}
}

func TestVisualSelectionSpansLines(t *testing.T) {
	m := testModel(t)
	m = key(m, "v")
	if m.mode != ModeVisual {
		t.Fatalf("v did not enter visual mode")
	}
	anchor := m.selAnchor
	m = key(m, "j")
	lo, hi := m.selectionRange()
	if hi-lo != 1 || lo != anchor {
		t.Errorf("selection range = [%d,%d], want span of 2 from %d", lo, hi, anchor)
	}
	m = key(m, "esc")
	if m.mode != ModeNormal || m.selAnchor != -1 {
		t.Errorf("esc did not clear selection")
	}
}

// TestCrossSideSelectionRejected verifies the GitHub side rule: a selection
// covering both a deletion and an addition cannot be commented on.
func TestCrossSideSelectionRejected(t *testing.T) {
	m := testModel(t)
	// Move to the deletion line in hunk 0, then select through the additions.
	m.cursor = mustFindRow(t, m, diff.SideLeft, 72)
	m = key(m, "v")
	// Extend down across the additions (RIGHT side).
	m = key(m, "j")
	m = key(m, "j")
	_, _, err := m.buildLocation()
	if err == nil {
		t.Fatalf("expected cross-side selection to be rejected")
	}
}

func TestSingleLineLocationResolves(t *testing.T) {
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snippet, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	if loc.Side != diff.SideRight || loc.StartLine != 72 || !loc.Single() {
		t.Errorf("location = %+v", loc)
	}
	if snippet == "" {
		t.Errorf("expected a snippet for the selected line")
	}
}

func TestExportCommand(t *testing.T) {
	m := testModel(t)
	dir := t.TempDir()
	// Add a comment directly to the draft, then export.
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	m.draft.Add(loc, "needs an error check", snip)

	out := filepath.Join(dir, "out.md")
	m.exportMarkdown(out)
	if m.err != nil {
		t.Fatalf("export error: %v", m.err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("export file is empty")
	}
}

func mustFindRow(t *testing.T, m *Model, side diff.Side, line int) int {
	t.Helper()
	for i, r := range m.rows() {
		if r.Source != nil && r.Source.Side == side && r.Source.StartLine == line {
			return i
		}
	}
	t.Fatalf("row for %s line %d not found", side, line)
	return 0
}
