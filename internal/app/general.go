package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/editor"
)

// generalItem is one row source of the general-conversation screen: a host
// comment, or a staged draft (draftID non-empty).
type generalItem struct {
	author  string
	when    string
	body    string
	hostID  int64
	draftID string
}

// generalItems merges the PR's host conversation (already oldest-first) with
// the staged general drafts, drafts last — the screen reads like the thread
// will read once submitted.
func (m *Model) generalItems() []generalItem {
	var items []generalItem
	if m.pr != nil {
		for _, c := range m.pr.General {
			when := ""
			if !c.CreatedAt.IsZero() {
				when = c.CreatedAt.Format("2006-01-02 15:04")
			}
			items = append(items, generalItem{author: c.Author, when: when, body: c.Body, hostID: c.ID})
		}
	}
	for _, g := range m.draft.General {
		author := m.author
		if author == "" {
			author = "you"
		}
		items = append(items, generalItem{author: author, when: g.At, body: g.Body, draftID: g.LocalID})
	}
	return items
}

// openGeneral opens the general-conversation screen (P, or :general). It
// needs PR mode: general comments anchor to the request itself.
func (m *Model) openGeneral() {
	if !m.prActive() {
		m.setStatus("general comments need pull-request mode")
		return
	}
	m.generalSel = 0
	m.mode = ModeGeneral
}

// handleGeneralKey drives the general-conversation screen: j/k select,
// r/enter reply to the selected comment (a quoted new comment — both forges'
// conversations are flat, so replying is quoting), a adds a fresh comment,
// d deletes a selected draft, esc/q/P close.
func (m *Model) handleGeneralKey(key string) tea.Cmd {
	items := m.generalItems()
	switch key {
	case "esc", "q", "P":
		m.mode = ModeNormal
	case "j", "down":
		if m.generalSel < len(items)-1 {
			m.generalSel++
		}
	case "k", "up":
		if m.generalSel > 0 {
			m.generalSel--
		}
	case "g":
		m.generalSel = 0
	case "G":
		m.generalSel = len(items) - 1
	case "a":
		return m.startGeneralComment(nil, "")
	case "r", "enter":
		if m.generalSel < 0 || m.generalSel >= len(items) {
			m.setStatus("nothing to reply to")
			return nil
		}
		it := items[m.generalSel]
		if it.draftID != "" {
			// Replying to your own unsent draft makes no sense; edit it.
			return m.startGeneralEdit(it.draftID)
		}
		return m.startGeneralComment(&it.hostID, quoteBody(it.author, it.body))
	case "e":
		if m.generalSel >= 0 && m.generalSel < len(items) && items[m.generalSel].draftID != "" {
			return m.startGeneralEdit(items[m.generalSel].draftID)
		}
		m.setStatus("only staged drafts can be edited")
	case "I":
		if m.generalSel >= 0 && m.generalSel < len(items) {
			m.startAttachImage(items[m.generalSel].draftID)
		}
	case "d":
		if m.generalSel >= 0 && m.generalSel < len(items) && items[m.generalSel].draftID != "" {
			m.draft.RemoveGeneral(items[m.generalSel].draftID)
			m.saveDraft()
			if m.generalSel >= len(m.generalItems()) {
				m.generalSel = len(m.generalItems()) - 1
			}
			m.setStatus("general draft deleted")
			return nil
		}
		m.setStatus("only staged drafts can be deleted")
	}
	return nil
}

// quoteBody renders a comment as a Markdown quote for a reply prefill — the
// only threading either forge offers at conversation level.
func quoteBody(author, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "> @%s:\n", author)
	for _, ln := range strings.Split(strings.TrimRight(stripImageMarkup(body), "\n"), "\n") {
		b.WriteString("> " + ln + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// startGeneralComment opens the editor for a new general (conversation-level)
// comment; prefill carries the quote when replying.
func (m *Model) startGeneralComment(quoteOf *int64, prefill string) tea.Cmd {
	tctx := editor.TemplateContext{File: "(general conversation)"}
	if quoteOf != nil {
		tctx.ReplyTo = fmt.Sprintf("general comment %d", *quoteOf)
	}
	sess, err := editor.NewSession(editor.BuildTemplate(tctx, prefill), "general")
	if err != nil {
		m.setError(err)
		return nil
	}
	m.inflight = &pendingEdit{general: true, generalQuote: quoteOf, session: sess}
	m.mode = ModeExternalEditor
	c := m.editor.Cmd(m.ctx, sess.Path)
	return tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
}

// startGeneralEdit reopens a staged general draft in the editor.
func (m *Model) startGeneralEdit(localID string) tea.Cmd {
	g := m.draft.GetGeneral(localID)
	if g == nil {
		return nil
	}
	sess, err := editor.NewSession(editor.BuildTemplate(editor.TemplateContext{File: "(general conversation)"}, g.Body), "general-edit")
	if err != nil {
		m.setError(err)
		return nil
	}
	m.inflight = &pendingEdit{editingGeneral: localID, session: sess}
	m.mode = ModeExternalEditor
	c := m.editor.Cmd(m.ctx, sess.Path)
	return tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
}

// startSummaryEdit opens the editor on the review summary — the general
// comment attached to the submission itself (the review body on GitHub, the
// note leading the review on GitLab). Reached from the confirmation screen.
func (m *Model) startSummaryEdit() tea.Cmd {
	sess, err := editor.NewSession(editor.BuildTemplate(editor.TemplateContext{File: "(review summary)"}, m.draft.Summary), "summary")
	if err != nil {
		m.setError(err)
		return nil
	}
	m.inflight = &pendingEdit{editSummary: true, session: sess}
	m.mode = ModeExternalEditor
	c := m.editor.Cmd(m.ctx, sess.Path)
	return tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
}

// generalView renders the conversation screen: host comments then staged
// drafts, the selection marked, bodies wrapped with image markup replaced by
// tags (or art via the shared overlay path).
func (m *Model) generalView() string {
	items := m.generalItems()
	w := m.width - 4
	if w < 20 {
		w = 20
	}
	var lines []string
	lines = append(lines, m.theme.Faint.Render("General conversation  (r/enter reply · a add · e edit draft · d delete draft · esc close)"), "")
	if len(items) == 0 {
		lines = append(lines, m.theme.Faint.Render("(no general comments yet — press a to add one)"))
	}
	for i, it := range items {
		marker := "  "
		if i == m.generalSel {
			marker = m.theme.Key.Render("▸ ")
		}
		head := m.theme.Key.Render("@" + it.author)
		if it.when != "" {
			head += m.theme.Faint.Render("  " + it.when)
		}
		if it.draftID != "" {
			head += m.theme.Metadata.Render("  (draft — posts on submit)")
		}
		lines = append(lines, marker+head)
		for _, ln := range wrapPlain(stripImageMarkup(it.body), w) {
			lines = append(lines, "    "+ln)
		}
		lines = append(lines, m.imageLines(it.body, w)...)
		lines = append(lines, "")
	}

	// Keep the selection on screen: center a window over the rendered lines
	// on the selected item's header row.
	h := m.contentHeight()
	if len(lines) > h {
		anchor := 0
		for i, ln := range lines {
			if strings.Contains(ln, "▸ ") {
				anchor = i
				break
			}
		}
		top := anchor - h/2
		if top < 0 {
			top = 0
		}
		if top > len(lines)-h {
			top = len(lines) - h
		}
		lines = lines[top:]
	}
	return strings.Join(lines, "\n")
}

// wrapPlain word-wraps s at width, preserving blank lines.
func wrapPlain(s string, w int) []string {
	return strings.Split(hardWrapJoin(s, w), "\n")
}

// hardWrapJoin is a tiny helper over the reflow wrappers used elsewhere.
func hardWrapJoin(s string, w int) string {
	out := make([]string, 0, 8)
	for _, ln := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		out = append(out, hardWrap(ln, w)...)
	}
	return strings.Join(out, "\n")
}
