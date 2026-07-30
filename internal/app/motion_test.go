package app

import (
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
)

func TestArrowKeysMoveCursor(t *testing.T) {
	m := testModel(t)
	start := m.cursor
	m = key(m, "down")
	if m.cursor != start+1 {
		t.Errorf("down arrow moved to %d, want %d", m.cursor, start+1)
	}
	m = key(m, "up")
	if m.cursor != start {
		t.Errorf("up arrow did not move back")
	}
}

func TestArrowsMirrorHLInSplit(t *testing.T) {
	m := testModel(t)
	m = key(m, "t") // split
	m = key(m, "left")
	if m.activeSide != diff.SideLeft {
		t.Errorf("left arrow in split should target LEFT, got %v", m.activeSide)
	}
	m = key(m, "right")
	if m.activeSide != diff.SideRight {
		t.Errorf("right arrow in split should target RIGHT, got %v", m.activeSide)
	}
}

func TestPageKeys(t *testing.T) {
	m := testModel(t)
	m.height = 12 // contentHeight 10
	m.clampCursor()
	start := m.cursor
	m = key(m, "pgdown")
	if m.cursor <= start {
		t.Fatalf("pgdown did not advance (%d -> %d)", start, m.cursor)
	}
	moved := m.cursor - start
	if moved != m.contentHeight() && m.cursor != len(m.rows())-1 {
		t.Errorf("pgdown moved %d rows, want a full page (%d) or clamp to end", moved, m.contentHeight())
	}
	m = key(m, "pgup")
	if m.cursor != start && m.cursor != 0 {
		t.Errorf("pgup did not return (cursor %d)", m.cursor)
	}
}
