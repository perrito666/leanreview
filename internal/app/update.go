package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/review"
)

// editorFinishedMsg is delivered after the external editor process exits.
type editorFinishedMsg struct{ err error }

// Update implements tea.Model. It handles the async messages (window resize,
// external-editor exit, submission result) directly and funnels every
// keystroke through handleKey, which owns all mode-dependent dispatch.
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

	case contextContentMsg:
		m.onContextContent(msg)
		return m, nil

	case imageFetchedMsg:
		m.onImageFetched(msg)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey routes a keystroke in strict priority order: text inputs
// (cmdline/search) first so typed characters are never interpreted as
// bindings, then the global ctrl+c quit, then the active overlay's own
// handler, and only in normal/visual mode the count-and-prefix grammar that
// resolves keys into commands for execute.
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
	if m.attachActive {
		m.handleAttachKey(key)
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
	case ModePR:
		m.handlePRKey(key)
		return m, nil
	case ModeConvo:
		return m, m.handleConvoKey(key)
	case ModeGeneral:
		return m, m.handleGeneralKey(key)
	}

	cmd, ready := m.pending.Feed(key, m.keymap, m.sequences)
	if !ready {
		return m, nil
	}
	// Batch in the once-per-file highlight fetch and the once-per-URL image
	// fetch, so file switches and freshly added comments upgrade themselves
	// without dedicated plumbing.
	return m, tea.Batch(m.execute(cmd), m.maybeFetchHighlight(), m.maybeFetchImages())
}

// execute performs a resolved normal-mode command, applying its numeric count
// to the motions that repeat. Most actions mutate the model in place; only the
// ones that must leave the TUI (spawning the external editor, quitting) return
// a tea.Cmd.
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
		if m.contextActive() {
			m.contextNextHunk(cmd.CountOr(1))
		} else {
			m.nextHunk(cmd.CountOr(1))
		}
	case "prev-hunk":
		if m.contextActive() {
			m.contextPrevHunk(cmd.CountOr(1))
		} else {
			m.prevHunk(cmd.CountOr(1))
		}
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
	case "motion-left":
		// Arrows and h/l share one motion: side targeting where sides exist
		// (split view), horizontal scroll otherwise.
		if m.layout == LayoutSplit {
			m.setActiveSide(diff.SideLeft)
		} else {
			m.scrollLeft()
		}
	case "motion-right":
		if m.layout == LayoutSplit {
			m.setActiveSide(diff.SideRight)
		} else {
			m.scrollRight()
		}
	case "page-down":
		m.fullPage(1)
	case "page-up":
		m.fullPage(-1)
	case "toggle-layout":
		m.toggleLayout()
	case "toggle-context":
		return m.toggleContext()
	case "toggle-syntax":
		return m.cycleSyntax()
	case "sidebar":
		m.toggleSidebar()
	case "toggle-inline":
		m.toggleInlineComments()
	case "toggle-wrap":
		m.toggleWrap()
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
	case "suggest":
		return m.startSuggestion()
	case "edit":
		return m.startEditUnderCursor()
	case "files":
		m.openFiles()
	case "comments":
		m.openComments()
	case "help":
		m.mode = ModeHelp
	case "pr-info":
		m.openPRInfo()
	case "general":
		m.openGeneral()
	case "attach-image":
		ids := m.commentIDsAt(m.cursor)
		target := ""
		if len(ids) > 0 {
			target = ids[len(ids)-1]
		}
		m.startAttachImage(target)
	case "delete-comment":
		m.deleteCommentUnderCursor()
	case "dismiss":
		m.dismissComment()
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
		// Priority: host threads (PR mode), then the draft conversation on
		// this line, then the comment list.
		if m.prActive() && len(m.threadsAt(m.cursor)) > 0 {
			m.openThreadReader()
		} else if len(m.commentIDsAt(m.cursor)) > 0 {
			m.openConversation()
		} else {
			m.openComments()
		}
	case "quit":
		m.quitting = true
		return tea.Quit
	}
	return nil
}

// toggleLayout switches between unified and split rendering. Row indices are
// projection-specific, so it captures the semantic (side, line) under the
// cursor and reanchors to the matching row afterwards; the selection is
// dropped because its anchor index would be meaningless in the new layout.
func (m *Model) toggleLayout() {
	// Preserve the semantic anchor across the toggle by remembering the line.
	anchor := m.rowAt(m.cursor)
	if m.layout == LayoutUnified {
		m.layout = LayoutSplit
		// The context projection is unified-shaped; leaving unified leaves it.
		m.contextView = false
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
func (m *Model) reanchor(side diff.Side, line int) {
	rows := m.rows()
	for i, r := range rows {
		if r.Source != nil && r.Source.Side == side && r.Source.StartLine == line {
			m.cursor = i
			return
		}
	}
}

// toggleSelect starts a visual selection anchored at the cursor, or cancels
// the one in progress — v acts as its own off switch.
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

// clearSelection returns to normal mode and drops the selection anchor
// (-1 is the "no selection" sentinel selectionRange keys off).
func (m *Model) clearSelection() {
	m.mode = ModeNormal
	m.selAnchor = -1
}

// deleteCommentUnderCursor removes a draft comment anchored at the cursor row
// and persists the draft. When several comments share the line the most
// recently added one is deleted, so repeated dd peels them off in reverse
// order of creation.
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

// onEditorFinished consumes the pending edit captured before the external
// editor was spawned: an editor error or an empty body discards it, otherwise
// the text updates the draft being edited or lands as a new comment/reply at
// the location recorded at spawn time (the cursor may have moved since). The
// temp session is always closed and the draft persisted on success.
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
	if pe.editSummary {
		m.draft.Summary = body
		m.saveDraft()
		m.mode = ModeConfirm
		m.setStatus("review summary updated")
		return nil
	}
	if pe.editingGeneral != "" {
		if g := m.draft.GetGeneral(pe.editingGeneral); g != nil {
			g.Body = body
			m.saveDraft()
			m.setStatus("general draft updated")
		}
		m.mode = ModeGeneral
		return nil
	}
	if pe.general {
		m.draft.AddGeneral(body, pe.generalQuote)
		m.saveDraft()
		m.mode = ModeGeneral
		m.setStatus("general comment staged (posts on submit)")
		return nil
	}
	if pe.replyToLocal != "" {
		c := m.draft.Get(pe.replyToLocal)
		if c == nil {
			m.setStatus("comment disappeared before the reply landed")
			return nil
		}
		if pe.editReplyAt != nil {
			// Editing an existing reply: replace the body, keep the author
			// and timestamp — polishing a message is not re-sending it.
			if i := *pe.editReplyAt; i >= 0 && i < len(c.Replies) {
				c.Replies[i].Body = body
				m.saveDraft()
				m.setStatus("reply updated")
			}
			return nil
		}
		author := m.author
		if author == "" {
			author = "reviewer"
		}
		c.Replies = append(c.Replies, review.ReviewReply{
			Author: author,
			Body:   body,
			At:     time.Now().UTC().Format(time.RFC3339),
		})
		m.saveDraft()
		m.setStatus("reply added to the conversation")
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

// saveDraft persists the draft when a store and key are available. In an
// exchange session it also rewrites the conversation file, so the on-disk
// document is current the moment the TUI exits — no separate export step.
func (m *Model) saveDraft() {
	if m.draft == nil {
		return
	}
	if m.store != nil && m.draft.SourceKey != "" {
		if err := m.store.Save(m.draft); err != nil {
			m.setError(fmt.Errorf("save drafts: %w", err))
		}
	}
	if m.exchangePath != "" {
		out, err := review.RenderExport(m.exchangePath, m.draft, m.rawPatch)
		if err == nil {
			err = os.WriteFile(m.exchangePath, out, 0o644)
		}
		if err != nil {
			m.setError(fmt.Errorf("write review exchange: %w", err))
		}
	}
}

// --- command line (:...) ---

// handleCmdlineKey edits the ":" command line one key at a time: printable
// keys append (named keys like arrows are ignored), enter strips the prompt
// and runs the command, and esc — or backspacing past the ":" — cancels, like
// Vim's command line.
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
		if r := []rune(m.cmdline); len(r) > 1 {
			m.cmdline = string(r[:len(r)-1])
		} else {
			m.cmdlineActive = false
			m.cmdline = ""
		}
	default:
		// One rune = typed text (multi-rune strings are named keys like "up").
		// Counting runes rather than bytes keeps non-ASCII input typable.
		if utf8.RuneCountInString(key) == 1 {
			m.cmdline += key
		}
	}
	return nil
}

// runCmdline executes an ex-style command. Beyond quit/save it is the only
// place a review event can be chosen explicitly (:approve, :request,
// :comment) and the entry point for markdown export, so infrequent actions
// don't need key bindings.
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
	case "pr":
		m.openPRInfo()
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

// dismissComment toggles the draft under the cursor between active and
// dismissed. Dismissal (unlike dd) keeps the comment: in a review-exchange
// conversation the verdict itself is information the other side needs.
func (m *Model) dismissComment() {
	ids := m.commentIDsAt(m.cursor)
	if len(ids) == 0 {
		m.setStatus("no draft comment under cursor to dismiss")
		return
	}
	c := m.draft.Get(ids[len(ids)-1])
	if c == nil {
		return
	}
	if c.State == review.DraftDismissed {
		c.State = review.DraftActive
		m.setStatus("comment restored")
	} else {
		c.State = review.DraftDismissed
		m.setStatus("comment dismissed (x restores, dd deletes)")
	}
	m.saveDraft()
}

// exportMarkdown writes the draft review to path — Markdown, or the review
// exchange document for .json destinations — echoing the absolute path in the
// status bar so the user can find a file created from a relative :export
// argument.
func (m *Model) exportMarkdown(path string) {
	out, err := review.RenderExport(path, m.draft, m.rawPatch)
	if err != nil {
		m.setError(fmt.Errorf("export: %w", err))
		return
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		m.setError(fmt.Errorf("export: %w", err))
		return
	}
	abs, _ := filepath.Abs(path)
	m.setStatus("exported %d comment(s) to %s", len(m.draft.Comments), abs)
}

// --- overlays ---

// openFiles opens the file picker with the list cursor preselecting the file
// currently under review, so enter without movement is a no-op jump.
func (m *Model) openFiles() {
	m.mode = ModeFiles
	m.listCursor = m.fileIdx
}

// handleFilesKey drives the file picker: j/k or arrows move, enter jumps to
// the selected file (gotoFile resets to normal mode, closing the overlay),
// and esc/q/f dismiss it — f matching the opening key so it behaves as a
// toggle.
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

// openComments opens the comment list at the top; the list is not stable
// across additions and deletions, so no previous position is restored.
func (m *Model) openComments() {
	m.mode = ModeComments
	m.listCursor = 0
}

// handleCommentsKey drives the comment list: enter jumps to the selected
// comment's diff location, e reopens it in the editor (dropping back to
// normal mode first so the editor returns to the diff), d deletes it, and
// esc/q/C close — C matching the opening key.
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

// handleConfirmKey drives the submission confirmation: y/enter submits,
// c/a/R re-pick the review event without leaving the screen (mirrored into
// the draft so the choice survives a cancel), and esc/n/q abort.
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
	case "g":
		return m.startSummaryEdit()
	case "esc", "n", "q":
		m.mode = ModeNormal
		m.setStatus("submission cancelled")
	}
	return nil
}

// jumpToComment closes the overlay and moves the cursor to the comment's
// anchor. It must switch files before reanchoring because row indices only
// have meaning within the current file's projection.
func (m *Model) jumpToComment(idx int) {
	if idx < 0 || idx >= len(m.draft.Comments) {
		return
	}
	c := m.draft.Comments[idx]
	m.mode = ModeNormal
	// Find the file and row for the comment's location. A missing file (an
	// orphaned draft after a head change) must not fall through to reanchor,
	// which would silently land on an unrelated row of the current file.
	for fi := range m.files {
		if m.files[fi].Path() == c.Location.Path {
			m.gotoFile(fi)
			m.reanchor(c.Location.Side, c.Location.StartLine)
			m.clampCursor()
			return
		}
	}
	m.setStatus("%s is not in this diff — comment kept as a draft", c.Location.Path)
}

// deleteCommentFromList removes the comment selected in the overlay and
// persists the draft, pulling the list cursor back when it would otherwise
// point past the shortened list.
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
