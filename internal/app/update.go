package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/review"
)

// editorFinishedMsg is delivered after the external editor process exits.
type editorFinishedMsg struct{ err error }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampCursor()
		return m, nil

	case editorFinishedMsg:
		return m, m.onEditorFinished(msg)

	case submitResultMsg:
		m.onSubmitResult(msg)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Command-line / search input takes precedence when active.
	if m.cmdlineActive {
		return m, m.handleCmdlineKey(key)
	}
	if m.searchActive {
		m.handleSearchKey(key)
		return m, nil
	}

	// Global quit.
	if key == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	// Overlays consume their own keys.
	switch m.mode {
	case ModeHelp:
		if key == "esc" || key == "?" || key == "q" {
			m.mode = ModeNormal
		}
		return m, nil
	case ModeFiles:
		return m, m.handleFilesKey(key)
	case ModeComments:
		return m, m.handleCommentsKey(key)
	case ModeConfirm:
		return m, m.handleConfirmKey(key)
	case ModeThread:
		return m, m.handleThreadKey(key)
	}

	cmd, ready := m.pending.Feed(key, m.keymap)
	if !ready {
		return m, nil
	}
	return m, m.execute(cmd)
}

func (m *Model) execute(cmd command) tea.Cmd {
	switch cmd.name {
	case "down":
		m.moveLine(cmd.CountOr(1))
	case "up":
		m.moveLine(-cmd.CountOr(1))
	case "next-change":
		m.nextChange(cmd.CountOr(1))
	case "prev-change":
		m.prevChange(cmd.CountOr(1))
	case "next-hunk":
		m.nextHunk(cmd.CountOr(1))
	case "prev-hunk":
		m.prevHunk(cmd.CountOr(1))
	case "next-file":
		m.nextFile(cmd.CountOr(1))
	case "prev-file":
		m.prevFile(cmd.CountOr(1))
	case "first-line":
		m.firstLine()
	case "last-line":
		m.lastLine()
	case "line-start":
		m.lineStart()
	case "line-end":
		m.lineEnd()
	case "scroll-left":
		m.scrollLeft()
	case "scroll-right":
		m.scrollRight()
	case "side-left":
		m.setActiveSide(diff.SideLeft)
	case "side-right":
		m.setActiveSide(diff.SideRight)
	case "toggle-layout":
		m.toggleLayout()
	case "sidebar":
		m.toggleSidebar()
	case "cmdline":
		m.cmdlineActive = true
		m.cmdline = ":"
	case "search":
		m.searchActive = true
		m.searchInput = "/"
	case "next-match":
		m.nextMatch(1)
	case "prev-match":
		m.nextMatch(-1)
	case "half-page-down":
		m.halfPage(1)
	case "half-page-up":
		m.halfPage(-1)
	case "reply":
		return m.startReplyUnderCursor()
	case "submit":
		if m.prActive() {
			m.beginSubmit(m.draft.Event)
		}
	case "select":
		m.toggleSelect()
	case "select-block":
		m.selectBlock()
	case "comment":
		return m.startComment("")
	case "edit":
		return m.startEditUnderCursor()
	case "files":
		m.openFiles()
	case "comments":
		m.openComments()
	case "help":
		m.mode = ModeHelp
	case "delete-comment":
		m.deleteCommentUnderCursor()
	case "toggle-fold":
		m.toggleFold()
	case "expand-all":
		m.expandAll()
	case "collapse-all":
		m.collapseAll()
	case "escape":
		m.clearSelection()
		m.clearSearch()
	case "open":
		if m.prActive() && len(m.threadsAt(m.cursor)) > 0 {
			m.openThreadReader()
		} else {
			m.openComments()
		}
	case "quit":
		m.quitting = true
		return tea.Quit
	}
	return nil
}

func (m *Model) toggleLayout() {
	// Preserve the semantic anchor across the toggle by remembering the line.
	anchor := m.rowAt(m.cursor)
	if m.layout == LayoutUnified {
		m.layout = LayoutSplit
	} else {
		m.layout = LayoutUnified
	}
	m.clearSelection()
	if anchor != nil && anchor.Source != nil {
		m.reanchor(anchor.Source.Side, anchor.Source.StartLine)
	}
	m.clampCursor()
	m.setStatus("%s view", m.layout)
}

// reanchor moves the cursor to the row matching side+line in the current layout.
func (m *Model) reanchor(side interface{ String() string }, line int) {
	rows := m.rows()
	for i, r := range rows {
		if r.Source != nil && r.Source.Side.String() == side.String() && r.Source.StartLine == line {
			m.cursor = i
			return
		}
	}
}

func (m *Model) toggleSelect() {
	if m.mode == ModeVisual {
		m.clearSelection()
		return
	}
	m.mode = ModeVisual
	m.selAnchor = m.cursor
	m.setStatus("visual: select lines, c to comment, esc to cancel")
}

// selectBlock selects the contiguous changed block around the cursor.
func (m *Model) selectBlock() {
	rows := m.rows()
	if !isChange(m.rowAt(m.cursor)) {
		m.setStatus("no changed block under cursor")
		return
	}
	lo, hi := m.cursor, m.cursor
	for lo > 0 && isChange(&rows[lo-1]) {
		lo--
	}
	for hi < len(rows)-1 && isChange(&rows[hi+1]) {
		hi++
	}
	m.mode = ModeVisual
	m.selAnchor = lo
	m.cursor = hi
	m.clampCursor()
}

func (m *Model) clearSelection() {
	m.mode = ModeNormal
	m.selAnchor = -1
}

func (m *Model) deleteCommentUnderCursor() {
	ids := m.commentIDsAt(m.cursor)
	if len(ids) == 0 {
		m.setStatus("no comment under cursor")
		return
	}
	m.draft.Remove(ids[len(ids)-1])
	m.saveDraft()
	m.setStatus("comment deleted")
}

// startComment resolves the selection, opens the editor, and returns the
// process command. editing is the local id when editing an existing comment.
func (m *Model) startComment(editing string) tea.Cmd {
	loc, snippet, err := m.buildLocation()
	if err != nil {
		m.setError(err)
		return nil
	}
	body := ""
	if editing != "" {
		if c := m.draft.Get(editing); c != nil {
			body = c.Body
		}
	}
	tctx := editor.TemplateContext{
		File:  loc.Path,
		Lines: lineRefString(loc),
		Side:  loc.Side.String(),
	}
	initial := editor.BuildTemplate(tctx, body)
	name := fmt.Sprintf("%s-%s", filepath.Base(loc.Path), lineRefString(loc))
	sess, err := editor.NewSession(initial, name)
	if err != nil {
		m.setError(err)
		return nil
	}
	m.inflight = &pendingEdit{loc: loc, snippet: snippet, editing: editing, session: sess}
	m.mode = ModeExternalEditor
	c := m.editor.Cmd(m.ctx, sess.Path)
	return tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
}

// startEditUnderCursor edits the draft comment anchored at the cursor line.
func (m *Model) startEditUnderCursor() tea.Cmd {
	ids := m.commentIDsAt(m.cursor)
	if len(ids) == 0 {
		m.setStatus("no draft comment under cursor to edit")
		return nil
	}
	return m.startEdit(ids[len(ids)-1])
}

// startEdit opens the editor prefilled with an existing draft comment's body,
// anchored to that comment's own location (not the cursor).
func (m *Model) startEdit(localID string) tea.Cmd {
	c := m.draft.Get(localID)
	if c == nil {
		m.setStatus("comment not found")
		return nil
	}
	loc := c.Location
	tctx := editor.TemplateContext{File: loc.Path, Lines: lineRefString(loc), Side: loc.Side.String()}
	if c.ReplyTo != nil {
		tctx.ReplyTo = fmt.Sprintf("comment %d", *c.ReplyTo)
	}
	sess, err := editor.NewSession(editor.BuildTemplate(tctx, c.Body), fmt.Sprintf("%s-%s", filepath.Base(loc.Path), lineRefString(loc)))
	if err != nil {
		m.setError(err)
		return nil
	}
	m.inflight = &pendingEdit{loc: loc, snippet: c.Snippet, editing: localID, replyTo: c.ReplyTo, session: sess}
	m.mode = ModeExternalEditor
	cc := m.editor.Cmd(m.ctx, sess.Path)
	return tea.ExecProcess(cc, func(err error) tea.Msg { return editorFinishedMsg{err} })
}

func (m *Model) onEditorFinished(msg editorFinishedMsg) tea.Cmd {
	pe := m.inflight
	m.inflight = nil
	if m.mode == ModeExternalEditor {
		m.mode = ModeNormal
	}
	if pe == nil {
		return nil
	}
	defer pe.session.Close()

	if msg.err != nil {
		m.setError(fmt.Errorf("editor: %w", msg.err))
		return nil
	}
	body, err := pe.session.Result()
	if err != nil {
		m.setError(err)
		return nil
	}
	if strings.TrimSpace(body) == "" {
		m.setStatus("comment discarded (empty)")
		return nil
	}
	if pe.editing != "" {
		if c := m.draft.Get(pe.editing); c != nil {
			c.Body = body
		}
		m.setStatus("comment updated")
	} else {
		id := m.draft.Add(pe.loc, body, pe.snippet)
		if pe.replyTo != nil {
			if c := m.draft.Get(id); c != nil {
				c.ReplyTo = pe.replyTo
			}
			m.setStatus("reply staged at %s %s", pe.loc.Path, lineRefString(pe.loc))
		} else {
			m.setStatus("comment added at %s %s", pe.loc.Path, lineRefString(pe.loc))
		}
	}
	m.clearSelection()
	m.saveDraft()
	return nil
}

// saveDraft persists the draft when a store and key are available.
func (m *Model) saveDraft() {
	if m.store == nil || m.draft == nil || m.draft.SourceKey == "" {
		return
	}
	if err := m.store.Save(m.draft); err != nil {
		m.setError(fmt.Errorf("save drafts: %w", err))
	}
}

// --- command line (:...) ---

func (m *Model) handleCmdlineKey(key string) tea.Cmd {
	switch key {
	case "esc":
		m.cmdlineActive = false
		m.cmdline = ""
	case "enter":
		line := strings.TrimPrefix(m.cmdline, ":")
		m.cmdlineActive = false
		m.cmdline = ""
		return m.runCmdline(strings.TrimSpace(line))
	case "backspace":
		if len(m.cmdline) > 1 {
			m.cmdline = m.cmdline[:len(m.cmdline)-1]
		} else {
			m.cmdlineActive = false
			m.cmdline = ""
		}
	default:
		if len(key) == 1 {
			m.cmdline += key
		}
	}
	return nil
}

func (m *Model) runCmdline(line string) tea.Cmd {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	switch fields[0] {
	case "q", "quit":
		m.quitting = true
		return tea.Quit
	case "w", "save":
		m.saveDraft()
		m.setStatus("drafts saved")
	case "wq":
		m.saveDraft()
		m.quitting = true
		return tea.Quit
	case "export":
		path := "review.md"
		if len(fields) > 1 {
			path = fields[1]
		}
		m.exportMarkdown(path)
	case "help":
		m.mode = ModeHelp
	case "comment":
		m.beginSubmit(forge.EventComment)
	case "approve":
		m.beginSubmit(forge.EventApprove)
	case "request":
		m.beginSubmit(forge.EventRequestChanges)
	case "submit":
		m.beginSubmit(m.draft.Event)
	default:
		m.setError(fmt.Errorf("unknown command: %s", fields[0]))
	}
	return nil
}

func (m *Model) exportMarkdown(path string) {
	md := review.ExportMarkdown(m.draft)
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		m.setError(fmt.Errorf("export: %w", err))
		return
	}
	abs, _ := filepath.Abs(path)
	m.setStatus("exported %d comment(s) to %s", len(m.draft.Comments), abs)
}

// --- overlays ---

func (m *Model) openFiles() {
	m.mode = ModeFiles
	m.listCursor = m.fileIdx
}

func (m *Model) handleFilesKey(key string) tea.Cmd {
	switch key {
	case "esc", "q", "f":
		m.mode = ModeNormal
	case "j", "down":
		if m.listCursor < len(m.files)-1 {
			m.listCursor++
		}
	case "k", "up":
		if m.listCursor > 0 {
			m.listCursor--
		}
	case "enter":
		m.gotoFile(m.listCursor)
	}
	return nil
}

func (m *Model) openComments() {
	m.mode = ModeComments
	m.listCursor = 0
}

func (m *Model) handleCommentsKey(key string) tea.Cmd {
	switch key {
	case "esc", "q", "C":
		m.mode = ModeNormal
	case "j", "down":
		if m.listCursor < len(m.draft.Comments)-1 {
			m.listCursor++
		}
	case "k", "up":
		if m.listCursor > 0 {
			m.listCursor--
		}
	case "enter":
		m.jumpToComment(m.listCursor)
	case "e":
		if m.listCursor >= 0 && m.listCursor < len(m.draft.Comments) {
			id := m.draft.Comments[m.listCursor].LocalID
			m.mode = ModeNormal
			return m.startEdit(id)
		}
	case "d":
		m.deleteCommentFromList(m.listCursor)
	}
	return nil
}

func (m *Model) handleConfirmKey(key string) tea.Cmd {
	switch key {
	case "y", "enter":
		return m.doSubmit()
	case "a":
		m.pendingEvent = forge.EventApprove
		m.draft.Event = forge.EventApprove
	case "c":
		m.pendingEvent = forge.EventComment
		m.draft.Event = forge.EventComment
	case "R":
		m.pendingEvent = forge.EventRequestChanges
		m.draft.Event = forge.EventRequestChanges
	case "esc", "n", "q":
		m.mode = ModeNormal
		m.setStatus("submission cancelled")
	}
	return nil
}

func (m *Model) jumpToComment(idx int) {
	if idx < 0 || idx >= len(m.draft.Comments) {
		return
	}
	c := m.draft.Comments[idx]
	m.mode = ModeNormal
	// Find the file and row for the comment's location.
	for fi := range m.files {
		if m.files[fi].Path() == c.Location.Path {
			m.gotoFile(fi)
			break
		}
	}
	m.reanchor(c.Location.Side, c.Location.StartLine)
	m.clampCursor()
}

func (m *Model) deleteCommentFromList(idx int) {
	if idx < 0 || idx >= len(m.draft.Comments) {
		return
	}
	m.draft.Remove(m.draft.Comments[idx].LocalID)
	m.saveDraft()
	if m.listCursor >= len(m.draft.Comments) && m.listCursor > 0 {
		m.listCursor--
	}
	m.setStatus("comment deleted")
}
