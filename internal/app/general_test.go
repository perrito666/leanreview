package app

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/forge"
)

func generalModel(t *testing.T) (*Model, *recordingForge) {
	t.Helper()
	f := &recordingForge{}
	m := prModel(t, f, nil)
	m.pr.General = []forge.Comment{
		{ID: 9, Author: "alice", Body: "shipping this today?", CreatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)},
	}
	return m, f
}

// TestGeneralScreenOpensAndLists: P opens the conversation screen in PR mode
// (and only there), listing host comments and staged drafts.
func TestGeneralScreenOpensAndLists(t *testing.T) {
	m, _ := generalModel(t)
	m = key(m, "P")
	if m.mode != ModeGeneral {
		t.Fatalf("P did not open the general screen, mode=%v", m.mode)
	}
	m.draft.AddGeneral("draft answer", nil)
	out := m.View()
	for _, want := range []string{"@alice", "shipping this today?", "draft answer", "(draft — posts on submit)"} {
		if !strings.Contains(out, want) {
			t.Errorf("general screen missing %q:\n%s", want, out)
		}
	}
	m = key(m, "esc")
	if m.mode != ModeNormal {
		t.Errorf("esc did not close, mode=%v", m.mode)
	}

	local := testModel(t)
	local.width, local.height = 100, 24
	local = key(local, "P")
	if local.mode == ModeGeneral {
		t.Errorf("general screen must need PR mode")
	}
}

// TestGeneralReplyPrefillsQuote: r on a host comment opens the editor with a
// Markdown quote — the only threading either forge offers at this level.
func TestGeneralReplyPrefillsQuote(t *testing.T) {
	m, _ := generalModel(t)
	m.openGeneral()
	cmd := m.handleGeneralKey("r")
	if cmd == nil {
		t.Fatalf("no editor command: %v %q", m.err, m.status)
	}
	buf, err := os.ReadFile(m.inflight.session.Path)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if !strings.Contains(string(buf), "> @alice:") || !strings.Contains(string(buf), "> shipping this today?") {
		t.Errorf("prefill missing the quote:\n%s", buf)
	}
	if m.inflight.generalQuote == nil || *m.inflight.generalQuote != 9 {
		t.Errorf("quote id not recorded: %+v", m.inflight)
	}
	m.inflight.session.Close()
}

// TestGeneralDraftLifecycle: the editor result stages a draft; d deletes it.
func TestGeneralDraftLifecycle(t *testing.T) {
	m, _ := generalModel(t)
	m.openGeneral()
	m.inflight = &pendingEdit{general: true}
	sessionWithBody(t, m, "a staged general note")
	m.onEditorFinished(editorFinishedMsg{})
	if len(m.draft.General) != 1 || m.draft.General[0].Body != "a staged general note" {
		t.Fatalf("draft not staged: %+v", m.draft.General)
	}
	if m.mode != ModeGeneral {
		t.Errorf("should return to the general screen, mode=%v", m.mode)
	}
	m.generalSel = 1 // the draft (host comment is 0)
	m.handleGeneralKey("d")
	if len(m.draft.General) != 0 {
		t.Errorf("d did not delete the draft")
	}
}

// TestGeneralsPostOnSubmit: staged general comments post individually after
// the review, with record-as-you-go bookkeeping like replies.
func TestGeneralsPostOnSubmit(t *testing.T) {
	m, f := generalModel(t)
	m.draft.AddGeneral("first note", nil)
	m.draft.AddGeneral("second note", nil)
	m.pendingEvent = forge.EventComment
	cmd := m.doSubmit()
	if cmd == nil {
		t.Fatalf("nothing submitted: %q", m.status)
	}
	msg := cmd().(submitResultMsg)
	m.onSubmitResult(msg)
	if len(f.generals) != 2 || f.generals[0] != "first note" {
		t.Errorf("generals posted = %v", f.generals)
	}
	if len(m.draft.General) != 0 {
		t.Errorf("posted generals must be cleared from the draft: %+v", m.draft.General)
	}
}

// TestGeneralFailureKeepsDraft: a failed post keeps the failed draft (and
// only it) for retry.
func TestGeneralFailureKeepsDraft(t *testing.T) {
	m, f := generalModel(t)
	f.failGeneral = true
	m.draft.AddGeneral("will fail", nil)
	m.pendingEvent = forge.EventComment
	msg := m.doSubmit()().(submitResultMsg)
	m.onSubmitResult(msg)
	if msg.err == nil {
		t.Fatalf("expected an error")
	}
	if len(m.draft.General) != 1 {
		t.Errorf("failed general must stay drafted: %+v", m.draft.General)
	}
}

// TestSummaryEditFromConfirm: g on the confirmation screen edits the review
// summary and returns to the confirmation, which then shows it.
func TestSummaryEditFromConfirm(t *testing.T) {
	m, f := generalModel(t)
	m.beginSubmit(forge.EventComment)
	if cmd := m.handleConfirmKey("g"); cmd == nil {
		t.Fatalf("g did not open the editor: %v", m.err)
	}
	sessionWithBody(t, m, "overall: looks good, two nits")
	m.onEditorFinished(editorFinishedMsg{})
	if m.draft.Summary != "overall: looks good, two nits" {
		t.Errorf("summary = %q", m.draft.Summary)
	}
	if m.mode != ModeConfirm {
		t.Errorf("should return to the confirmation, mode=%v", m.mode)
	}
	if out := m.View(); !strings.Contains(out, "overall: looks good, two nits") {
		t.Errorf("confirmation does not show the summary:\n%s", out)
	}
	msg := m.doSubmit()().(submitResultMsg)
	m.onSubmitResult(msg)
	if f.createdSummary != "overall: looks good, two nits" {
		t.Errorf("summary not submitted: %q", f.createdSummary)
	}
}

// sessionWithBody replaces the inflight session with one whose Result is body.
func sessionWithBody(t *testing.T, m *Model, body string) {
	t.Helper()
	if m.inflight.session != nil {
		m.inflight.session.Close()
	}
	sess, err := editor.NewSession("<!-- h -->\n\n"+body+"\n", "test")
	if err != nil {
		t.Fatal(err)
	}
	m.inflight.session = sess
}
