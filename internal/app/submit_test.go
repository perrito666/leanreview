package app

import (
	"context"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/review"
	"github.com/perrito666/leanreview/internal/ui"
)

// recordingForge captures submission calls.
type recordingForge struct {
	createdEvent    forge.ReviewEvent
	createdSummary  string
	createdComments []forge.ReviewComment
	replies         []string
	failReview      bool
	failReplyAt     int // 1-based reply call that fails; 0 = never
}

func (f *recordingForge) PullRequest(context.Context, forge.PullRequestRef) (*forge.PullRequest, error) {
	return &forge.PullRequest{}, nil
}
func (f *recordingForge) Diff(context.Context, forge.PullRequestRef) ([]byte, error) { return nil, nil }
func (f *recordingForge) Threads(context.Context, forge.PullRequestRef) ([]forge.Thread, error) {
	return nil, nil
}
func (f *recordingForge) CreateReview(_ context.Context, _ forge.PullRequestRef, ev forge.ReviewEvent, summary string, cs []forge.ReviewComment) (*forge.SubmittedReview, error) {
	if f.failReview {
		return nil, errFake
	}
	f.createdEvent = ev
	f.createdSummary = summary
	f.createdComments = cs
	return &forge.SubmittedReview{ID: 42, URL: "http://x/42"}, nil
}
func (f *recordingForge) FileContent(context.Context, forge.PullRequestRef, string, string) ([]byte, error) {
	return nil, errFake
}
func (f *recordingForge) Attachment(context.Context, forge.PullRequestRef, string) ([]byte, error) {
	return nil, errFake
}
func (f *recordingForge) GeneralComments(context.Context, forge.PullRequestRef) ([]forge.Comment, error) {
	return nil, nil
}
func (f *recordingForge) Reply(_ context.Context, _ forge.PullRequestRef, id int64, body string) (*forge.Comment, error) {
	if f.failReplyAt > 0 && len(f.replies)+1 == f.failReplyAt {
		return nil, errFake
	}
	f.replies = append(f.replies, body)
	return &forge.Comment{ID: id, Body: body}, nil
}

var errFake = &fakeErr{}

type fakeErr struct{}

func (*fakeErr) Error() string { return "boom" }

func prModel(t *testing.T, f forge.Forge, threads []forge.Thread) *Model {
	t.Helper()
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	m := New(Config{
		Files: loadAppFixture(t),
		Title: "o/r#7",
		Theme: ui.DefaultTheme(),
		PR: &PRContext{
			Forge:   f,
			Ref:     forge.PullRequestRef{Owner: "o", Repo: "r", Number: 7},
			PR:      &forge.PullRequest{},
			Threads: threads,
		},
	})
	m.width, m.height = 100, 24
	return m
}

func TestToReviewComment(t *testing.T) {
	single := toReviewComment(review.DraftComment{
		Location: diff.Location{Path: "f.go", Side: diff.SideRight, StartLine: 10, EndLine: 10},
		Body:     "x",
	})
	if single.Line != 10 || single.StartLine != 0 || single.Side != "RIGHT" {
		t.Errorf("single = %+v", single)
	}
	multi := toReviewComment(review.DraftComment{
		Location: diff.Location{Path: "f.go", Side: diff.SideRight, StartLine: 8, EndLine: 12},
		Body:     "y",
	})
	if multi.Line != 12 || multi.StartLine != 8 || multi.StartSide != "RIGHT" {
		t.Errorf("multi = %+v", multi)
	}
}

func TestSubmitCreatesReviewAndClearsDrafts(t *testing.T) {
	f := &recordingForge{}
	m := prModel(t, f, nil)
	// Stage two line comments.
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	m.draft.Add(loc, "handle error", snip)
	m.draft.Add(diff.Location{Path: "other.go", Side: diff.SideRight, StartLine: 3, EndLine: 3}, "typo", "x")

	m.beginSubmit(forge.EventRequestChanges)
	if m.mode != ModeConfirm {
		t.Fatalf("beginSubmit did not open confirm screen")
	}
	cmd := m.doSubmit()
	if cmd == nil {
		t.Fatalf("doSubmit returned no command")
	}
	msg := cmd() // run the network closure (fake)
	res, ok := msg.(submitResultMsg)
	if !ok {
		t.Fatalf("unexpected message %T", msg)
	}
	m.onSubmitResult(res)

	if f.createdEvent != forge.EventRequestChanges {
		t.Errorf("event = %q", f.createdEvent)
	}
	if len(f.createdComments) != 2 {
		t.Errorf("submitted %d comments, want 2", len(f.createdComments))
	}
	if len(m.draft.Comments) != 0 {
		t.Errorf("drafts not cleared after submit: %d remain", len(m.draft.Comments))
	}
	if !strings.Contains(m.status, "submitted") {
		t.Errorf("status = %q", m.status)
	}
}

func TestSubmitPostsReplies(t *testing.T) {
	f := &recordingForge{}
	threads := []forge.Thread{{
		Root:     forge.Comment{ID: 555, Author: "a", Body: "why?"},
		Location: &diff.Location{Path: "internal/api/handler.go", Side: diff.SideRight, StartLine: 72, EndLine: 72},
	}}
	m := prModel(t, f, threads)

	// Stage a reply by simulating the editor result on a reply pendingEdit.
	replyTo := int64(555)
	loc := *threads[0].Location
	sess, err := editor.NewSession("<!-- header -->\n\nbecause tests\n", "reply")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	m.inflight = &pendingEdit{loc: loc, snippet: "why?", replyTo: &replyTo, session: sess}
	m.mode = ModeExternalEditor
	m.onEditorFinished(editorFinishedMsg{})

	if len(m.draft.Comments) != 1 || m.draft.Comments[0].ReplyTo == nil {
		t.Fatalf("reply not staged with ReplyTo: %+v", m.draft.Comments)
	}

	m.beginSubmit(forge.EventComment)
	msg := m.doSubmit()()
	m.onSubmitResult(msg.(submitResultMsg))

	if len(f.replies) != 1 || f.replies[0] != "because tests" {
		t.Errorf("replies = %+v", f.replies)
	}
}

func TestSubmitSkipsOrphanedComments(t *testing.T) {
	f := &recordingForge{}
	m := prModel(t, f, nil)
	// One active line comment and one orphaned one.
	m.draft.Add(diff.Location{Path: "f.go", Side: diff.SideRight, StartLine: 5, EndLine: 5}, "active", "x")
	id := m.draft.Add(diff.Location{Path: "f.go", Side: diff.SideRight, StartLine: 9, EndLine: 9}, "orphaned", "y")
	m.draft.Get(id).State = review.DraftOrphaned

	if c := m.submitCounts(); c.NewComments != 1 || c.Orphaned != 1 {
		t.Fatalf("counts = %+v, want 1 new, 1 orphaned", c)
	}

	m.beginSubmit(forge.EventComment)
	m.onSubmitResult(m.doSubmit()().(submitResultMsg))

	if len(f.createdComments) != 1 || f.createdComments[0].Body != "active" {
		t.Errorf("submitted comments = %+v, want only the active one", f.createdComments)
	}
	// The orphaned comment must be retained as a draft; the active one cleared.
	if len(m.draft.Comments) != 1 || m.draft.Comments[0].Body != "orphaned" {
		t.Errorf("remaining drafts = %+v, want only the orphaned one", m.draft.Comments)
	}
}

func TestReplyRequiresThread(t *testing.T) {
	m := prModel(t, &recordingForge{}, nil)
	// Cursor on a line with no thread.
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	if cmd := m.startReplyUnderCursor(); cmd != nil {
		t.Errorf("expected no editor command when there is no thread")
	}
	if !strings.Contains(m.status, "nothing to reply to") {
		t.Errorf("status = %q", m.status)
	}
}

func TestSubmitBlockedInLocalMode(t *testing.T) {
	m := testModel(t) // no PR context
	m.beginSubmit(forge.EventApprove)
	if m.mode == ModeConfirm {
		t.Errorf("local mode should not open submit confirmation")
	}
	if m.err == nil {
		t.Errorf("expected an error explaining PR mode is required")
	}
}

// TestSubmitPartialFailureClearsPostedDrafts covers the retry-safety contract:
// when the review goes up but a reply fails, everything already accepted by
// the host must leave the draft, so a retry only sends the failed reply.
func TestSubmitPartialFailureClearsPostedDrafts(t *testing.T) {
	f := &recordingForge{failReplyAt: 2}
	m := prModel(t, f, nil)
	m.draft.Add(diff.Location{Path: "f.go", Side: diff.SideRight, StartLine: 5, EndLine: 5}, "line note", "x")
	ok := int64(1)
	bad := int64(2)
	id1 := m.draft.Add(diff.Location{Path: "f.go", Side: diff.SideRight, StartLine: 6, EndLine: 6}, "first reply", "x")
	m.draft.Get(id1).ReplyTo = &ok
	id2 := m.draft.Add(diff.Location{Path: "f.go", Side: diff.SideRight, StartLine: 7, EndLine: 7}, "second reply", "x")
	m.draft.Get(id2).ReplyTo = &bad

	m.beginSubmit(forge.EventComment)
	m.onSubmitResult(m.doSubmit()().(submitResultMsg))

	if m.err == nil {
		t.Fatalf("partial failure should surface an error")
	}
	if len(f.createdComments) != 1 || len(f.replies) != 1 {
		t.Fatalf("host state: %d comments, %d replies; want 1 and 1", len(f.createdComments), len(f.replies))
	}
	// Only the failed reply remains; retrying cannot duplicate the review.
	if len(m.draft.Comments) != 1 || m.draft.Comments[0].Body != "second reply" {
		t.Errorf("remaining drafts = %+v, want only the failed reply", m.draft.Comments)
	}
}

func TestSubmitReviewFailureSurfaces(t *testing.T) {
	f := &recordingForge{failReview: true}
	m := prModel(t, f, nil)
	m.draft.Add(diff.Location{Path: "f.go", Side: diff.SideRight, StartLine: 1, EndLine: 1}, "x", "y")
	m.beginSubmit(forge.EventComment)
	msg := m.doSubmit()()
	m.onSubmitResult(msg.(submitResultMsg))
	if m.err == nil {
		t.Errorf("submission failure should surface an error")
	}
	if len(m.draft.Comments) != 1 {
		t.Errorf("drafts should be retained on failure")
	}
}
