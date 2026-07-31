package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/review"
)

// openConversation opens the focused reader for the draft comment under the
// cursor — the editing surface for a review-exchange conversation, where
// individual replies (not just the root comment) can be edited and deleted.
func (m *Model) openConversation() {
	ids := m.commentIDsAt(m.cursor)
	if len(ids) == 0 {
		return
	}
	m.convoID = ids[len(ids)-1]
	m.convoSel = 0
	m.mode = ModeConvo
}

// convoComment resolves the conversation's comment, closing the overlay if it
// disappeared (e.g. deleted from under us).
func (m *Model) convoComment() *review.DraftComment {
	c := m.draft.Get(m.convoID)
	if c == nil {
		m.mode = ModeNormal
	}
	return c
}

// handleConvoKey drives the conversation reader. Selection index 0 is the
// root comment; 1..n are its replies — so every item of the conversation is
// an addressable target for edit/delete, not just the root.
func (m *Model) handleConvoKey(key string) tea.Cmd {
	c := m.convoComment()
	if c == nil {
		return nil
	}
	last := len(c.Replies) // selectable items: 0 (comment) .. len(replies)
	switch key {
	case "esc", "q", "enter":
		m.mode = ModeNormal
	case "j", "down":
		if m.convoSel < last {
			m.convoSel++
		}
	case "k", "up":
		if m.convoSel > 0 {
			m.convoSel--
		}
	case "r":
		m.mode = ModeNormal
		return m.startDraftReply(c.LocalID)
	case "x":
		if c.State == review.DraftDismissed {
			c.State = review.DraftActive
		} else {
			c.State = review.DraftDismissed
		}
		m.saveDraft()
	case "e":
		if m.convoSel == 0 {
			m.mode = ModeNormal
			return m.startEdit(c.LocalID)
		}
		return m.startReplyEdit(c.LocalID, m.convoSel-1)
	case "d":
		if m.convoSel == 0 {
			// Deleting the root deletes the whole conversation, same as dd
			// on the diff line.
			m.draft.Remove(c.LocalID)
			m.saveDraft()
			m.mode = ModeNormal
			m.setStatus("comment deleted")
			return nil
		}
		idx := m.convoSel - 1
		c.Replies = append(c.Replies[:idx], c.Replies[idx+1:]...)
		if m.convoSel > len(c.Replies) {
			m.convoSel = len(c.Replies)
		}
		m.saveDraft()
		m.setStatus("reply deleted")
	}
	return nil
}

// startReplyEdit opens the editor prefilled with an existing reply's body.
// The reply keeps its author and timestamp: editing polishes the message, it
// does not re-attribute it.
func (m *Model) startReplyEdit(localID string, idx int) tea.Cmd {
	c := m.draft.Get(localID)
	if c == nil || idx < 0 || idx >= len(c.Replies) {
		return nil
	}
	return m.openReplyEditor(c, &pendingEdit{replyToLocal: localID, editReplyAt: &idx}, c.Replies[idx].Body)
}

// convoView renders the conversation: the anchored snippet for context, then
// the root comment and each reply as selectable blocks.
func (m *Model) convoView() string {
	c := m.convoComment()
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("Conversation (j/k select, e edit, d delete, r reply, x dismiss, esc close)\n\n")
	loc := fmt.Sprintf("%s %s (%s)", c.Location.Path, lineRefString(c.Location), c.Location.Side)
	if c.State != review.DraftActive {
		loc += " [" + c.State.String() + "]"
	}
	b.WriteString("── " + loc + "\n")
	if strings.TrimSpace(c.Snippet) != "" {
		b.WriteString("   " + firstLine(c.Snippet) + "\n")
	}
	b.WriteString("\n")

	writeItem := func(sel bool, author, when, body string) {
		cursor := "  "
		if sel {
			cursor = "▶ "
		}
		if author == "" {
			author = "you"
		}
		head := cursor + "@" + author
		if when != "" {
			head += "  " + when
		}
		b.WriteString(head + "\n")
		for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
			b.WriteString("      " + line + "\n")
		}
		b.WriteString("\n")
	}
	writeItem(m.convoSel == 0, c.Author, c.At, c.Body)
	for i, r := range c.Replies {
		writeItem(m.convoSel == i+1, r.Author, r.At, "↳ "+r.Body)
	}
	return b.String()
}
