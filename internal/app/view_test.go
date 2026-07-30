package app

import (
	"os"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
)

func TestViewRendersDiff(t *testing.T) {
	m := testModel(t)
	out := m.View()
	if !strings.Contains(out, "leanreview") {
		t.Errorf("title bar missing from view")
	}
	if !strings.Contains(out, "calculate(input)") {
		t.Errorf("diff content missing from view:\n%s", out)
	}
	// The status bar advertises the command entry points.
	if !strings.Contains(out, "help") {
		t.Errorf("status hint missing")
	}
}

func TestSplitViewRenders(t *testing.T) {
	m := testModel(t)
	m = key(m, "t") // switch to split
	out := m.View()
	if !strings.Contains(out, "│") {
		t.Errorf("split separator missing from split view")
	}
}

// TestCommentRoundTripHeadless drives the editor-finished path with a session we
// control, then asserts the comment lands in the draft, renders a marker, and
// exports.
func TestCommentRoundTripHeadless(t *testing.T) {
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snippet, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}

	// Simulate the editor having written a note.
	sess, err := editor.NewSession("<!-- header -->\n\nNeeds an error check.\n", "x")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	m.inflight = &pendingEdit{loc: loc, snippet: snippet, session: sess}
	m.mode = ModeExternalEditor

	m.onEditorFinished(editorFinishedMsg{})

	if len(m.draft.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(m.draft.Comments))
	}
	if m.mode != ModeNormal {
		t.Errorf("mode not restored to normal after editor")
	}

	// The marker should render on the commented row.
	out := m.View()
	if !strings.Contains(out, "●1") {
		t.Errorf("comment marker not rendered:\n%s", out)
	}

	// Export includes the note.
	tmp := t.TempDir() + "/out.md"
	m.exportMarkdown(tmp)
	data, _ := os.ReadFile(tmp)
	if !strings.Contains(string(data), "Needs an error check.") {
		t.Errorf("export missing comment body:\n%s", data)
	}
}
