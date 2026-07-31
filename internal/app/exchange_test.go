package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/review"
)

// TestDismissToggle: x flags the comment under the cursor as dismissed (kept,
// excluded from submission) and restores it on a second press.
func TestDismissToggle(t *testing.T) {
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	id := m.draft.Add(loc, "questionable note", snip)

	m = key(m, "x")
	if got := m.draft.Get(id).State; got != review.DraftDismissed {
		t.Fatalf("state after x = %v, want dismissed", got)
	}
	if !strings.Contains(m.View(), "[dismissed]") {
		t.Errorf("dismissed tag not rendered in the preview")
	}
	if c := m.submitCounts(); c.Dismissed != 1 || c.NewComments != 0 {
		t.Errorf("submit counts = %+v, want the dismissed comment excluded", c)
	}
	m = key(m, "x")
	if got := m.draft.Get(id).State; got != review.DraftActive {
		t.Errorf("state after second x = %v, want active", got)
	}
}

// TestExchangeWriteback: with a writeback path set, any draft mutation
// rewrites the conversation file so quitting leaves it current.
func TestExchangeWriteback(t *testing.T) {
	m := testModel(t)
	path := filepath.Join(t.TempDir(), "review.json")
	m.rawPatch = []byte(loadPatchFixture(t))
	m.SetExchangeWriteback(path)

	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	m.draft.Add(loc, "please handle this", snip)
	m.saveDraft()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("writeback file not written: %v", err)
	}
	e, err := review.ParseExchange(data)
	if err != nil {
		t.Fatalf("writeback is not a valid exchange: %v", err)
	}
	if len(e.Comments) != 1 || e.Comments[0].Body != "please handle this" {
		t.Errorf("writeback comments = %+v", e.Comments)
	}
	if !strings.Contains(string(e.Patch), "diff --git") {
		t.Errorf("writeback lost the patch")
	}
}

// TestImportedAuthorShownInPreview: exchange comments carry attribution the
// inline box must surface.
func TestImportedAuthorShownInPreview(t *testing.T) {
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	id := m.draft.Add(loc, "from the model", snip)
	m.draft.Get(id).Author = "assistant"
	m.draft.Get(id).Replies = []review.ReviewReply{{Author: "human", Body: "ok"}}

	out := m.View()
	if !strings.Contains(out, "@assistant:") {
		t.Errorf("imported author missing from preview:\n%s", out)
	}
	// Replies render inside the same box, attributed.
	if !strings.Contains(out, "↳ @human: ok") {
		t.Errorf("reply missing from the comment box:\n%s", out)
	}
}

// TestReplyToDraftFlow: r on a draft comment opens the editor; the finished
// body lands as an attributed, timestamped reply on that comment.
func TestReplyToDraftFlow(t *testing.T) {
	m := testModel(t)
	m.author = "hduran"
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	id := m.draft.Add(loc, "from the model", snip)
	m.draft.Get(id).Author = "assistant"

	sess, err := editor.NewSession("<!-- header -->\n\ndisagree, this is intentional\n", "reply")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	m.inflight = &pendingEdit{replyToLocal: id, session: sess}
	m.mode = ModeExternalEditor
	m.onEditorFinished(editorFinishedMsg{})

	c := m.draft.Get(id)
	if len(c.Replies) != 1 {
		t.Fatalf("reply not recorded: %+v", c.Replies)
	}
	r := c.Replies[0]
	if r.Author != "hduran" || r.Body != "disagree, this is intentional" || r.At == "" {
		t.Errorf("reply = %+v, want attributed and timestamped", r)
	}
	// Still one comment: a conversation reply is not a new draft.
	if len(m.draft.Comments) != 1 {
		t.Errorf("reply created a new comment: %d", len(m.draft.Comments))
	}
}

func loadPatchFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "diff", "testdata", "simple.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}
