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
	if !strings.Contains(out, "test") {
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

func TestCursorSkipsAnnotationRows(t *testing.T) {
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	m.draft.Add(loc, "note", snip)

	// The row after the commented line is now an annotation; j must glide over
	// it onto the next real line.
	at := mustFindRow(t, m, diff.SideRight, 72)
	m.cursor = at
	m = key(m, "j")
	r := m.rowAt(m.cursor)
	if r == nil || r.Annotation {
		t.Errorf("cursor rested on an annotation row")
	}
	if r.Source == nil || r.Source.StartLine == 72 {
		t.Errorf("cursor did not advance past the annotation (row %+v)", r.Source)
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

	// The gutter marker and inline preview should render for the comment.
	out := m.View()
	if !strings.Contains(out, "●") {
		t.Errorf("comment gutter marker not rendered:\n%s", out)
	}
	if !strings.Contains(out, "Needs an error check.") {
		t.Errorf("inline comment preview not rendered:\n%s", out)
	}

	// i hides the inline preview but keeps the gutter marker.
	m = key(m, "i")
	out = m.View()
	if strings.Contains(out, "Needs an error check.") {
		t.Errorf("inline preview should be hidden after i:\n%s", out)
	}
	if !strings.Contains(out, "●") {
		t.Errorf("gutter marker should survive hiding inline previews")
	}

	// Export includes the note.
	tmp := t.TempDir() + "/out.md"
	m.exportMarkdown(tmp)
	data, _ := os.ReadFile(tmp)
	if !strings.Contains(string(data), "Needs an error check.") {
		t.Errorf("export missing comment body:\n%s", data)
	}
}
