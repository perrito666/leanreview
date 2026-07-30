package app

import (
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
)

func TestEditDraftUnderCursor(t *testing.T) {
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	id := m.draft.Add(loc, "first version", snip)

	// e should open an editor session prefilled with the existing body.
	cmd := m.startEditUnderCursor()
	if cmd == nil {
		t.Fatalf("expected an editor command")
	}
	if m.inflight == nil || m.inflight.editing != id {
		t.Fatalf("inflight not set to edit %s: %+v", id, m.inflight)
	}
	body, _ := m.inflight.session.Result()
	if body != "first version" {
		t.Errorf("editor not prefilled with existing body, got %q", body)
	}

	// Simulate saving new content.
	sess, _ := editor.NewSession("<!-- h -->\n\nsecond version\n", "x")
	m.inflight.session = sess
	m.onEditorFinished(editorFinishedMsg{})

	if len(m.draft.Comments) != 1 {
		t.Fatalf("edit should not add a new comment: %d", len(m.draft.Comments))
	}
	if m.draft.Comments[0].Body != "second version" {
		t.Errorf("body not updated: %q", m.draft.Comments[0].Body)
	}
}

func TestEditWithoutCommentIsNoop(t *testing.T) {
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	if cmd := m.startEditUnderCursor(); cmd != nil {
		t.Errorf("expected no command when there is no draft to edit")
	}
	if !strings.Contains(m.status, "no draft comment") {
		t.Errorf("status = %q", m.status)
	}
}
