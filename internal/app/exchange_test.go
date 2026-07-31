package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/review"
	"github.com/perrito666/leanreview/internal/ui"
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

// convoModel builds a model with one authored comment carrying two replies
// and the conversation reader open on it.
func convoModel(t *testing.T) (*Model, string) {
	t.Helper()
	m := testModel(t)
	m.author = "hduran"
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	id := m.draft.Add(loc, "root note", snip)
	c := m.draft.Get(id)
	c.Author = "assistant"
	c.Replies = []review.ReviewReply{
		{Author: "hduran", Body: "first reply"},
		{Author: "assistant", Body: "second reply"},
	}
	m = key(m, "enter")
	if m.mode != ModeConvo {
		t.Fatalf("enter on a commented line should open the conversation, mode=%v", m.mode)
	}
	return m, id
}

// TestConversationReaderCRUD: every item of a conversation is addressable —
// replies can be selected, deleted, and edited, not just the root comment.
func TestConversationReaderCRUD(t *testing.T) {
	m, id := convoModel(t)
	out := m.View()
	for _, want := range []string{"@assistant", "root note", "↳ first reply", "↳ second reply"} {
		if !strings.Contains(out, want) {
			t.Errorf("conversation view missing %q:\n%s", want, out)
		}
	}

	// Select reply 2 and delete it.
	m = key(m, "j")
	m = key(m, "j")
	m = key(m, "d")
	c := m.draft.Get(id)
	if len(c.Replies) != 1 || c.Replies[0].Body != "first reply" {
		t.Fatalf("reply deletion wrong: %+v", c.Replies)
	}

	// Edit reply 1 via the editor-finished path.
	idx := 0
	sess, err := editor.NewSession("<!-- h -->\n\npolished reply\n", "r")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	m.inflight = &pendingEdit{replyToLocal: id, editReplyAt: &idx, session: sess}
	m.mode = ModeExternalEditor
	m.onEditorFinished(editorFinishedMsg{})
	c = m.draft.Get(id)
	if c.Replies[0].Body != "polished reply" || c.Replies[0].Author != "hduran" {
		t.Errorf("reply edit should replace body and keep author: %+v", c.Replies[0])
	}

	// Deleting the root removes the conversation and closes the reader.
	m.mode = ModeConvo
	m.convoSel = 0
	m = key(m, "d")
	if m.draft.Get(id) != nil || m.mode != ModeNormal {
		t.Errorf("root deletion should remove the comment and close the reader")
	}
}

// TestConversationDismissAndReply: x toggles the root state from the reader;
// r hands off to the reply editor.
func TestConversationDismissAndReply(t *testing.T) {
	m, id := convoModel(t)
	m = key(m, "x")
	if m.draft.Get(id).State != review.DraftDismissed {
		t.Errorf("x in the reader should dismiss the comment")
	}
	if cmd := m.handleConvoKey("r"); cmd == nil {
		t.Errorf("r in the reader should open the reply editor")
	}
	if m.mode != ModeExternalEditor {
		t.Errorf("mode = %v, want external editor", m.mode)
	}
}

// TestHTMLImageTagParsing uses the exact shape GitHub embeds pasted images
// with: the src must be extracted and the raw HTML removed from display.
func TestHTMLImageTagParsing(t *testing.T) {
	body := `broken here <img width="416" height="480" alt="Image" src="https://github.com/user-attachments/assets/900ae85f-976b-45ee-985a-1d16637c1901" /> as you can see`
	refs := imageRefs(body)
	if len(refs) != 1 || refs[0] != "https://github.com/user-attachments/assets/900ae85f-976b-45ee-985a-1d16637c1901" {
		t.Fatalf("refs = %v", refs)
	}
	clean := stripImageMarkup(body)
	if strings.Contains(clean, "<img") || strings.Contains(clean, "src=") {
		t.Errorf("raw HTML leaked into display text: %q", clean)
	}
	if !strings.Contains(clean, "[Image]") {
		t.Errorf("alt text lost: %q", clean)
	}
	// Markdown form: alt kept, target stripped.
	if got := stripImageMarkup("see ![shot](a/b.png) here"); got != "see [shot] here" {
		t.Errorf("markdown strip = %q", got)
	}
	// GitLab relative upload counts as remote.
	if !isRemoteRef("/uploads/abc/shot.png") || !isRemoteRef("https://x") || isRemoteRef("docs/a.png") {
		t.Errorf("isRemoteRef misclassifies")
	}
}

// TestForgeAttachmentFetchFlow: a thread with a GitHub-style <img> attachment
// fetches once through the forge, lands in a session file, and renders in
// the box; failures degrade to a tag and are not retried.
func TestForgeAttachmentFetchFlow(t *testing.T) {
	threads := []forge.Thread{{
		Root: forge.Comment{ID: 1, Author: "rev",
			Body: `look <img alt="Image" src="https://github.com/user-attachments/assets/xyz" />`},
		Location: &diff.Location{Path: "internal/api/handler.go", Side: diff.SideRight, StartLine: 72, EndLine: 72},
	}}
	m := prModel(t, &recordingForge{}, threads)
	m.images = ui.NewImageRenderer("kitty")
	png, err := os.ReadFile(writeTestPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	fetches := 0
	m.fetchImage = func(context.Context, string) ([]byte, error) {
		fetches++
		return png, nil
	}

	// Before the fetch lands: visible tag with a fetching hint.
	cmd := m.maybeFetchImages()
	if cmd == nil {
		t.Fatalf("no fetch issued for the thread attachment")
	}
	if !strings.Contains(m.View(), "(fetching…)") {
		t.Errorf("pending attachment should render as a fetching tag")
	}
	if m.maybeFetchImages() != nil {
		t.Fatalf("second scan refetched a pending URL")
	}

	msg := cmd().(imageFetchedMsg)
	m.onImageFetched(msg)
	if fetches != 1 {
		t.Fatalf("fetches = %d", fetches)
	}
	pre := 0
	for _, r := range m.rows() {
		if r.Annotation && r.Pre {
			pre++
		}
	}
	if pre == 0 {
		t.Errorf("fetched attachment did not render as image rows")
	}
	if strings.Contains(m.View(), "<img") {
		t.Errorf("raw HTML still visible in the box")
	}

	// Failure path: remembered, tagged, not retried.
	m2 := prModel(t, &recordingForge{}, threads)
	m2.images = ui.NewImageRenderer("kitty")
	m2.fetchImage = func(context.Context, string) ([]byte, error) { return nil, errFake }
	cmd2 := m2.maybeFetchImages()
	m2.onImageFetched(cmd2().(imageFetchedMsg))
	if !strings.Contains(m2.View(), "[image: https://github.com/user-attachments/assets/xyz]") {
		t.Errorf("failed attachment should render as a plain tag")
	}
	if m2.maybeFetchImages() != nil {
		t.Errorf("failed URL retried")
	}
}
