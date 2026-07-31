package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/review"
)

// startReplyUnderCursor begins a reply at the cursor line: to the first host
// review thread there (PR mode — staged as a draft with ReplyTo, posted on
// submission), or, when no thread applies, to the draft comment there (a
// review-exchange conversation reply, appended to the comment and carried
// through the file). One key, two conversation media.
func (m *Model) startReplyUnderCursor() tea.Cmd {
	var idxs []int
	if m.prActive() {
		idxs = m.threadsAt(m.cursor)
	}
	if len(idxs) == 0 {
		if ids := m.commentIDsAt(m.cursor); len(ids) > 0 {
			return m.startDraftReply(ids[len(ids)-1])
		}
		m.setStatus("nothing to reply to on this line")
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

// startDraftReply opens the editor for a new conversation reply to the draft
// comment with the given local id. The reply travels with the comment
// through the exchange file rather than being a comment of its own, so the
// other side reads it in context.
func (m *Model) startDraftReply(localID string) tea.Cmd {
	c := m.draft.Get(localID)
	if c == nil {
		return nil
	}
	return m.openReplyEditor(c, &pendingEdit{replyToLocal: localID}, "")
}

// openReplyEditor launches the editor for a conversation reply — pe carries
// whether the finish appends a new reply or updates an existing one, initial
// seeds the buffer (the current body when editing).
func (m *Model) openReplyEditor(c *review.DraftComment, pe *pendingEdit, initial string) tea.Cmd {
	who := c.Author
	if who == "" {
		who = "you"
	}
	tctx := editor.TemplateContext{
		File:    c.Location.Path,
		Lines:   lineRefString(c.Location),
		Side:    c.Location.Side.String(),
		ReplyTo: fmt.Sprintf("comment by @%s: %s", who, firstLine(c.Body)),
	}
	sess, err := editor.NewSession(editor.BuildTemplate(tctx, initial), "reply-"+c.LocalID)
	if err != nil {
		m.setError(err)
		return nil
	}
	pe.session = sess
	m.inflight = pe
	m.mode = ModeExternalEditor
	cc := m.editor.Cmd(m.ctx, sess.Path)
	return tea.ExecProcess(cc, func(err error) tea.Msg { return editorFinishedMsg{err} })
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
	General     int // staged conversation-level comments
	Orphaned    int // non-reply comments with no valid location; not submitted
	Dismissed   int // comments a human rejected; kept locally, never submitted
}

// submitCounts tallies the draft's comments by how doSubmit will treat them:
// replies posted individually, orphaned and dismissed drafts skipped, the
// rest submitted as new review comments.
func (m *Model) submitCounts() submitCounts {
	var c submitCounts
	for i := range m.draft.Comments {
		cm := &m.draft.Comments[i]
		switch {
		case cm.State == review.DraftDismissed:
			c.Dismissed++
		case cm.ReplyTo != nil:
			c.Replies++
		case cm.State == review.DraftOrphaned:
			c.Orphaned++
		default:
			c.NewComments++
		}
	}
	c.General = len(m.draft.General)
	return c
}

// submitResultMsg is delivered when a submission finishes. done lists the
// draft ids that actually reached the host — on a partial failure (review
// created, a later reply refused) it holds everything already posted, so
// those drafts are cleared and a retry cannot duplicate them.
type submitResultMsg struct {
	review   *forge.SubmittedReview
	replies  int
	generals int
	done     []string // draft ids confirmed posted (removed even when err is set)
	err      error
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
		id   string
	}
	var commentIDs []string
	for i := range m.draft.Comments {
		cm := &m.draft.Comments[i]
		if cm.State == review.DraftDismissed {
			// Rejected by a human: stays in the conversation, never submits.
			continue
		}
		if cm.ReplyTo != nil {
			replies = append(replies, struct {
				to   int64
				body string
				id   string
			}{*cm.ReplyTo, cm.Body, cm.LocalID})
			continue
		}
		if cm.State == review.DraftOrphaned {
			// No valid location: keep it as a draft for the reviewer to fix.
			continue
		}
		commentIDs = append(commentIDs, cm.LocalID)
		newComments = append(newComments, toReviewComment(*cm))
	}

	var generals []struct{ body, id string }
	for _, g := range m.draft.General {
		generals = append(generals, struct{ body, id string }{g.Body, g.LocalID})
	}

	if len(newComments) == 0 && len(replies) == 0 && len(generals) == 0 && event == forge.EventComment && summary == "" {
		m.setStatus("nothing to submit")
		return nil
	}

	m.submitting = true
	m.setStatus("submitting review…")
	ctx := m.ctx

	return func() tea.Msg {
		var msg submitResultMsg
		// Create the review (also valid with zero comments for APPROVE/COMMENT).
		if len(newComments) > 0 || event != forge.EventComment || summary != "" {
			res, err := f.CreateReview(ctx, ref, event, summary, newComments)
			if err != nil {
				msg.err = err
				return msg
			}
			msg.review = res
		}
		// The review is up: its comments are posted regardless of what the
		// replies below do, so record them as done now.
		msg.done = append(msg.done, commentIDs...)
		// Post replies individually (they use the parent comment's identity),
		// recording each success so a mid-loop failure never resubmits them.
		for _, r := range replies {
			if _, err := f.Reply(ctx, ref, r.to, r.body); err != nil {
				msg.err = fmt.Errorf("posted review but a reply failed (the failed reply is kept as a draft): %w", err)
				return msg
			}
			msg.done = append(msg.done, r.id)
			msg.replies++
		}
		// General comments post individually too, with the same
		// record-as-you-go bookkeeping so retries never duplicate.
		for _, g := range generals {
			if _, err := f.AddGeneralComment(ctx, ref, g.body); err != nil {
				msg.err = fmt.Errorf("a general comment failed to post (kept as a draft): %w", err)
				return msg
			}
			msg.done = append(msg.done, g.id)
			msg.generals++
		}
		return msg
	}
}

// onSubmitResult applies the outcome of a submission. Drafts confirmed posted
// are removed even when the submission failed part-way, so retrying only sends
// what the host has not yet accepted.
func (m *Model) onSubmitResult(msg submitResultMsg) {
	m.submitting = false
	for _, id := range msg.done {
		m.draft.Remove(id)
	}
	if len(msg.done) > 0 {
		m.saveDraft()
	}
	if msg.err != nil {
		m.setError(msg.err)
		return
	}
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
