package app

import (
	"testing"

	"github.com/perrito666/leanreview/internal/ui"
)

func keymapModel(t *testing.T, overrides map[string]string) *Model {
	t.Helper()
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	m := New(Config{
		Files: loadAppFixture(t),
		Title: "t",
		Theme: ui.DefaultTheme(),
		Keys:  overrides,
	})
	m.width, m.height = 100, 20
	return m
}

func TestDefaultKeymapStillMoves(t *testing.T) {
	m := keymapModel(t, nil)
	start := m.cursor
	m = key(m, "j")
	if m.cursor != start+1 {
		t.Errorf("default j should move down")
	}
}

func TestRemapKey(t *testing.T) {
	// Bind x to "down" (x is unbound by default) and verify it moves the cursor.
	m := keymapModel(t, map[string]string{"x": "down"})
	start := m.cursor
	m = key(m, "x")
	if m.cursor != start+1 {
		t.Errorf("remapped x did not move down (cursor %d -> %d)", start, m.cursor)
	}
}

func TestUnbindKey(t *testing.T) {
	// Empty action unbinds j; it should no longer move.
	m := keymapModel(t, map[string]string{"j": ""})
	start := m.cursor
	m = key(m, "j")
	if m.cursor != start {
		t.Errorf("unbound j should be inert, cursor moved to %d", m.cursor)
	}
}

func TestUnknownActionIgnored(t *testing.T) {
	// An override to a non-existent action is ignored; the key keeps... nothing,
	// but must not panic and must not bind.
	m := keymapModel(t, map[string]string{"x": "does-not-exist"})
	if m.keymap["x"] != "" {
		t.Errorf("unknown action should not be bound, got %q", m.keymap["x"])
	}
}

func TestBindSingleKeyToTwoKeyAction(t *testing.T) {
	// A single key may be bound to a two-key action like next-hunk.
	m := keymapModel(t, map[string]string{"m": "next-hunk"})
	startHunk := m.rowAt(m.cursor).Source.HunkIndex
	m = key(m, "m")
	got := m.rowAt(m.cursor)
	if got == nil || got.Source == nil || got.Source.HunkIndex == startHunk {
		t.Errorf("single key bound to next-hunk did not advance the hunk")
	}
}

func TestCountPrefixStillWorksThroughKeymap(t *testing.T) {
	m := keymapModel(t, nil)
	start := m.cursor
	m = key(m, "3")
	m = key(m, "j")
	if m.cursor != start+3 {
		t.Errorf("3j via keymap moved to %d, want %d", m.cursor, start+3)
	}
}
