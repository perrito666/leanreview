package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/review"
)

// startReplyUnderCursor begins a reply to the first thread anchored at the
// cursor line. Replies are staged as draft comments (with ReplyTo set) and
// posted when the review is submitted.
func (m *Model) startReplyUnderCursor() tea.Cmd {
	if !m.prActive() {
		m.setStatus("replies require pull-request mode")
		return nil
	}
	idxs := m.threadsAt(m.cursor)
	if len(idxs) == 0 {
		m.setStatus("no thread to reply to on this line")
		return nil
	}
	th := m.pr.Threads[idxs[0]]
	if th.Location == nil {
		m.setStatus("thread has no line location")
		return nil
	}
	loc := *th.Location
	loc.CommitOID = m.headOID
	replyTo := th.Root.ID

	tctx := editor.TemplateContext{
		File:    loc.Path,
		Lines:   lineRefString(loc),
		Side:    loc.Side.String(),
		ReplyTo: fmt.Sprintf("comment %d by @%s", replyTo, th.Root.Author),
	}
	sess, err := editor.NewSession(editor.BuildTemplate(tctx, ""), fmt.Sprintf("reply-%d", replyTo))
	if err != nil {
		m.setError(err)
		return nil
	}
	m.inflight = &pendingEdit{loc: loc, snippet: firstLine(th.Root.Body), replyTo: &replyTo, session: sess}
	m.mode = ModeExternalEditor
	c := m.editor.Cmd(m.ctx, sess.Path)
	return tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
}

// beginSubmit sets the review event and opens the confirmation screen.
func (m *Model) beginSubmit(event forge.ReviewEvent) {
	if !m.prActive() {
		m.setError(fmt.Errorf("review submission requires pull-request mode"))
		return
	}
	if m.submitting {
		return
	}
	m.pendingEvent = event
	m.draft.Event = event
	m.mode = ModeConfirm
}

// submitCounts summarises the pending draft for the confirmation screen.
type submitCounts struct {
	NewComments int // submittable line comments (active, non-reply)
	Replies     int
	Orphaned    int // non-reply comments with no valid location; not submitted
}

// submitCounts tallies the draft's comments by how doSubmit will treat them:
// replies posted individually, orphaned drafts skipped, the rest submitted as
// new review comments.
func (m *Model) submitCounts() submitCounts {
	var c submitCounts
	for i := range m.draft.Comments {
		cm := &m.draft.Comments[i]
		switch {
		case cm.ReplyTo != nil:
			c.Replies++
		case cm.State == review.DraftOrphaned:
			c.Orphaned++
		default:
			c.NewComments++
		}
	}
	return c
}

// submitResultMsg is delivered when a submission finishes.
type submitResultMsg struct {
	review  *forge.SubmittedReview
	replies int
	ids     []string // draft ids that were submitted (removed on success)
	err     error
}

// doSubmit builds the network command that submits the review and posts replies.
func (m *Model) doSubmit() tea.Cmd {
	m.mode = ModeNormal
	if !m.prActive() {
		m.setError(fmt.Errorf("review submission requires pull-request mode"))
		return nil
	}

	f := m.pr.Forge
	ref := m.pr.Ref
	event := m.pendingEvent
	summary := m.draft.Summary

	var newComments []forge.ReviewComment
	var replies []struct {
		to   int64
		body string
	}
	var ids []string
	for i := range m.draft.Comments {
		cm := &m.draft.Comments[i]
		if cm.ReplyTo != nil {
			ids = append(ids, cm.LocalID)
			replies = append(replies, struct {
				to   int64
				body string
			}{*cm.ReplyTo, cm.Body})
			continue
		}
		if cm.State == review.DraftOrphaned {
			// No valid location: keep it as a draft for the reviewer to fix.
			continue
		}
		ids = append(ids, cm.LocalID)
		newComments = append(newComments, toReviewComment(*cm))
	}

	if len(newComments) == 0 && len(replies) == 0 && event == forge.EventComment && summary == "" {
		m.setStatus("nothing to submit")
		return nil
	}

	m.submitting = true
	m.setStatus("submitting review…")
	ctx := m.ctx

	return func() tea.Msg {
		msg := submitResultMsg{ids: ids}
		// Create the review (also valid with zero comments for APPROVE/COMMENT).
		if len(newComments) > 0 || event != forge.EventComment || summary != "" {
			res, err := f.CreateReview(ctx, ref, event, summary, newComments)
			if err != nil {
				msg.err = err
				return msg
			}
			msg.review = res
		}
		// Post replies individually (they use the parent comment's identity).
		for _, r := range replies {
			if _, err := f.Reply(ctx, ref, r.to, r.body); err != nil {
				msg.err = fmt.Errorf("posted review but a reply failed: %w", err)
				return msg
			}
			msg.replies++
		}
		return msg
	}
}

// onSubmitResult applies the outcome of a submission.
func (m *Model) onSubmitResult(msg submitResultMsg) {
	m.submitting = false
	if msg.err != nil {
		m.setError(msg.err)
		return
	}
	// Remove the submitted drafts and persist.
	for _, id := range msg.ids {
		m.draft.Remove(id)
	}
	m.saveDraft()
	url := ""
	if msg.review != nil {
		url = msg.review.URL
	}
	m.setStatus("review submitted (%s)  %s", m.pendingEvent, url)
}

// toReviewComment maps a draft comment to the host API representation.
func toReviewComment(c review.DraftComment) forge.ReviewComment {
	rc := forge.ReviewComment{
		Path: c.Location.Path,
		Body: c.Body,
		Line: c.Location.EndLine,
		Side: c.Location.Side.String(),
	}
	if !c.Location.Single() {
		rc.StartLine = c.Location.StartLine
		rc.StartSide = c.Location.Side.String()
	}
	return rc
}
