package app

import (
	"strings"
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

// TestLastLineSkipsTrailingDisplayRows: with a comment on the file's final
// changed line, its annotation box is the tail of the row list — G must rest
// on the last real line, not inside the box.
func TestLastLineSkipsTrailingDisplayRows(t *testing.T) {
	m := testModel(t)
	rows := m.rows()
	var lastSrc int
	for i := range rows {
		if rows[i].Source != nil {
			lastSrc = i
		}
	}
	m.cursor = lastSrc
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	m.draft.Add(loc, "tail note", snip)

	m.lastLine()
	r := m.rowAt(m.cursor)
	if r == nil || r.Annotation || r.Continuation {
		t.Errorf("G rested on a display-only row: %+v", r)
	}
}

// TestJumpToCommentMissingFileKeepsCursor: jumping to a draft whose file left
// the diff must not reanchor inside the current file.
func TestJumpToCommentMissingFileKeepsCursor(t *testing.T) {
	m := testModel(t)
	m.draft.Add(diff.Location{Path: "gone/away.go", Side: diff.SideRight, StartLine: 72, EndLine: 72}, "orphan", "x")
	m.cursor = m.firstContentRow()
	before := m.cursor
	m.jumpToComment(0)
	if m.cursor != before {
		t.Errorf("cursor moved to %d for a comment in a missing file", m.cursor)
	}
	if !strings.Contains(m.status, "not in this diff") {
		t.Errorf("status = %q, want a missing-file hint", m.status)
	}
}
