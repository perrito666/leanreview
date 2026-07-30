package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/ui"
)

// View implements tea.Model.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}

	switch m.mode {
	case ModeHelp:
		return m.frame(ui.HelpText())
	case ModeFiles:
		return m.frame(m.filesView())
	case ModeComments:
		return m.frame(m.commentsView())
	case ModeConfirm:
		return m.frame(m.confirmView())
	case ModeThread:
		return m.frame(m.threadReaderView())
	}

	if len(m.files) == 0 {
		return m.frame("No changes to review.")
	}

	body := m.diffLines()
	if m.effSidebar() {
		side := m.sidebarLines(len(body))
		sep := m.theme.Faint.Render("│")
		for i := range body {
			s := ""
			if i < len(side) {
				s = side[i]
			}
			body[i] = s + sep + body[i]
		}
	}
	return m.titleBar() + "\n" + strings.Join(body, "\n") + "\n" + m.statusBar()
}

// frame wraps overlay content between the title and status bars.
func (m *Model) frame(body string) string {
	lines := strings.Split(body, "\n")
	h := m.contentHeight()
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, ln := range lines {
		lines[i] = pad(clip(ln, m.width), m.width)
	}
	return m.titleBar() + "\n" + strings.Join(lines, "\n") + "\n" + m.statusBar()
}

func (m *Model) titleBar() string {
	f := m.currentFile()
	path := ""
	fileInfo := ""
	if f != nil {
		path = f.Path()
		if f.Status != diff.StatusModified {
			path += " (" + f.Status.String() + ")"
		}
		fileInfo = fmt.Sprintf(" [%d/%d]", m.fileIdx+1, len(m.files))
	}
	left := fmt.Sprintf("leanreview  %s", m.title)
	right := fmt.Sprintf("%s%s  %s", path, fileInfo, m.layout)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return m.theme.Title.Width(m.width).Render(clip(line, m.width))
}

func (m *Model) statusBar() string {
	if m.cmdlineActive {
		return m.theme.Status.Width(m.width).Render(clip(m.cmdline, m.width))
	}
	if m.searchActive {
		return m.theme.Status.Width(m.width).Render(clip(m.searchInput, m.width))
	}
	if m.err != nil {
		return m.theme.Error.Width(m.width).Render(clip("error: "+m.err.Error(), m.width))
	}
	mode := m.mode.String()
	if m.layout == LayoutSplit {
		mode += fmt.Sprintf(" [%s]", m.activeSide)
	}
	msg := m.status
	if msg == "" {
		msg = fmt.Sprintf("%d comment(s)   ? help   : command", len(m.draft.Comments))
	}
	line := fmt.Sprintf("%s  %s", mode, msg)
	return m.theme.Status.Width(m.width).Render(clip(line, m.width))
}

// diffLines renders exactly contentHeight rows of the diff at the content width
// (which excludes the sidebar when it is shown).
func (m *Model) diffLines() []string {
	rows := m.rows()
	h := m.contentHeight()
	nw := m.numWidth()
	lo, hi := m.selectionRange()

	out := make([]string, 0, h)
	for i := m.top; i < m.top+h; i++ {
		if i >= len(rows) {
			out = append(out, pad("", m.contentWidth()))
			continue
		}
		out = append(out, m.renderRow(i, &rows[i], nw, i >= lo && i <= hi))
	}
	return out
}

// numWidth returns the gutter width needed for line numbers in the current file.
func (m *Model) numWidth() int {
	max := 0
	f := m.currentFile()
	if f != nil {
		for hi := range f.Hunks {
			for _, l := range f.Hunks[hi].Lines {
				if l.OldLine != nil && *l.OldLine > max {
					max = *l.OldLine
				}
				if l.NewLine != nil && *l.NewLine > max {
					max = *l.NewLine
				}
			}
		}
	}
	w := len(strconv.Itoa(max))
	if w < 3 {
		w = 3
	}
	return w
}

func (m *Model) renderRow(idx int, r *diff.DisplayRow, nw int, inSel bool) string {
	isCursor := idx == m.cursor
	selected := inSel && m.mode == ModeVisual && m.selAnchor >= 0
	cw := m.contentWidth()

	// Hunk header rows span the full content width.
	if isHeader(r) {
		text := clip(r.Left.Text, cw)
		if isCursor {
			return m.theme.Cursor.Width(cw).Render(text)
		}
		return m.theme.Metadata.Width(cw).Render(pad(text, cw))
	}

	marker := ""
	if n := len(m.commentIDsAt(idx)); n > 0 {
		marker = fmt.Sprintf(" ●%d", n)
	}
	if t := len(m.threadsAt(idx)); t > 0 {
		marker += fmt.Sprintf(" ◆%d", t)
	}

	var plain string
	if m.layout == LayoutSplit {
		plain = m.plainSplit(r, nw, marker)
	} else {
		plain = m.plainUnified(r, nw, marker)
	}
	plain = pad(clip(plain, cw), cw)

	switch {
	case isCursor:
		return m.theme.Cursor.Render(plain)
	case selected:
		return m.theme.Select.Render(plain)
	case m.search != "" && m.rowMatches(r):
		return m.theme.Search.Render(plain)
	case m.layout == LayoutUnified && m.highlighter.Enabled():
		return m.renderHighlighted(r, nw, marker)
	default:
		return m.styleFor(kindOf(r)).Render(plain)
	}
}

// renderHighlighted draws a unified content row with a dim number gutter, a
// diff-colored sign, and syntax-highlighted source text. This is the order the
// design calls for: source text → syntax spans → horizontal clip → assembly.
func (m *Model) renderHighlighted(r *diff.DisplayRow, nw int, marker string) string {
	path := ""
	if f := m.currentFile(); f != nil {
		path = f.Path()
	}
	gutter := m.theme.Gutter.Render(fmt.Sprintf("%s %s ", numStr(r.Left.LineNumber, nw), numStr(r.Right.LineNumber, nw)))
	sign := m.styleFor(r.Left.Kind).Render(signFor(r.Left.Kind) + " ")

	cw := m.contentWidth()
	avail := cw - lipgloss.Width(gutter) - lipgloss.Width(sign) - lipgloss.Width(marker)
	if avail < 1 {
		avail = 1
	}
	text := ansi.Truncate(m.hcut(m.highlight(path, r.Left.Text)), avail, "")
	line := gutter + sign + text
	if marker != "" {
		line += m.theme.Marker.Render(marker)
	}
	return pad(line, cw)
}

func (m *Model) plainUnified(r *diff.DisplayRow, nw int, marker string) string {
	oldN := numStr(r.Left.LineNumber, nw)
	newN := numStr(r.Right.LineNumber, nw)
	sign := signFor(r.Left.Kind)
	return fmt.Sprintf("%s %s %s %s%s", oldN, newN, sign, m.hcut(r.Left.Text), marker)
}

func (m *Model) plainSplit(r *diff.DisplayRow, nw int, marker string) string {
	half := (m.contentWidth() - (nw * 2) - 6) / 2
	if half < 4 {
		half = 4
	}
	lNum, lText := "", ""
	if r.Left != nil {
		lNum = numStr(r.Left.LineNumber, nw)
		lText = m.hcut(r.Left.Text)
	} else {
		lNum = strings.Repeat(" ", nw)
	}
	rNum, rText := "", ""
	if r.Right != nil {
		rNum = numStr(r.Right.LineNumber, nw)
		rText = m.hcut(r.Right.Text)
	} else {
		rNum = strings.Repeat(" ", nw)
	}
	left := fmt.Sprintf("%s %s", lNum, pad(clip(lText, half), half))
	right := fmt.Sprintf("%s %s%s", rNum, clip(rText, half), marker)
	return left + " │ " + right
}

func (m *Model) styleFor(kind diff.LineKind) lipgloss.Style {
	switch kind {
	case diff.LineAddition:
		return m.theme.Addition
	case diff.LineDeletion:
		return m.theme.Deletion
	case diff.LineMetadata:
		return m.theme.Metadata
	default:
		return m.theme.Context
	}
}

func (m *Model) filesView() string {
	var b strings.Builder
	b.WriteString("Files (enter to open, esc to close)\n\n")
	for i := range m.files {
		f := &m.files[i]
		cursor := "  "
		if i == m.listCursor {
			cursor = "▶ "
		}
		n := m.commentCountForPath(f.Path())
		marker := ""
		if n > 0 {
			marker = fmt.Sprintf("  ●%d", n)
		}
		b.WriteString(fmt.Sprintf("%s%-9s %s%s\n", cursor, f.Status.String(), f.Path(), marker))
	}
	return b.String()
}

func (m *Model) commentsView() string {
	var b strings.Builder
	b.WriteString("Comments (enter to jump, e to edit, d to delete, esc to close)\n\n")
	if len(m.draft.Comments) == 0 {
		b.WriteString("  (no drafts yet — press c on a line to add one)\n")
	}
	for i, c := range m.draft.Comments {
		cursor := "  "
		if i == m.listCursor {
			cursor = "▶ "
		}
		reply := ""
		if c.ReplyTo != nil {
			reply = "↳ "
		}
		state := ""
		if c.State != 0 { // non-active
			state = " [" + c.State.String() + "]"
		}
		first := firstLine(c.Body)
		b.WriteString(fmt.Sprintf("%s%s%s %s (%s)%s  %s\n", cursor, reply, c.Location.Path, lineRefString(c.Location), c.Location.Side, state, first))
	}

	// Existing review threads (read-only here; reply with r on the diff line).
	if m.prActive() && len(m.pr.Threads) > 0 {
		b.WriteString("\nExisting threads ◆\n")
		for _, th := range m.pr.Threads {
			loc := "general"
			if th.Location != nil {
				loc = fmt.Sprintf("%s %s", th.Location.Path, lineRefString(*th.Location))
			}
			flags := ""
			if th.Outdated {
				flags = " (outdated)"
			}
			b.WriteString(fmt.Sprintf("  %s — @%s: %s%s\n", loc, th.Root.Author, firstLine(th.Root.Body), flags))
			for _, rep := range th.Replies {
				b.WriteString(fmt.Sprintf("      ↳ @%s: %s\n", rep.Author, firstLine(rep.Body)))
			}
		}
	}
	return b.String()
}

func (m *Model) confirmView() string {
	c := m.submitCounts()
	var b strings.Builder
	b.WriteString("Submit review\n\n")
	if m.prActive() {
		b.WriteString(fmt.Sprintf("  Pull request: %s/%s#%d\n", m.pr.Ref.Owner, m.pr.Ref.Repo, m.pr.Ref.Number))
	}
	b.WriteString(fmt.Sprintf("  %d new comment(s), %d repl(y/ies)\n", c.NewComments, c.Replies))
	if c.Orphaned > 0 {
		b.WriteString(fmt.Sprintf("  ⚠ %d orphaned comment(s) (location lost after a head change) will NOT be\n    submitted — reposition them first (they remain saved as drafts)\n", c.Orphaned))
	}
	b.WriteString("\n  Event:\n")
	b.WriteString(eventLine("c", "Comment", m.pendingEvent == forge.EventComment))
	b.WriteString(eventLine("a", "Approve", m.pendingEvent == forge.EventApprove))
	b.WriteString(eventLine("R", "Request changes", m.pendingEvent == forge.EventRequestChanges))
	b.WriteString("\n  Press y to submit, c/a/R to change the event, esc to cancel.\n")
	return b.String()
}

func eventLine(key, label string, selected bool) string {
	mark := "[ ]"
	if selected {
		mark = "[x]"
	}
	return fmt.Sprintf("    %s %s  (%s)\n", mark, label, key)
}

func (m *Model) commentCountForPath(path string) int {
	n := 0
	for _, c := range m.draft.Comments {
		if c.Location.Path == path {
			n++
		}
	}
	return n
}

// --- small formatting helpers ---

func numStr(n *int, w int) string {
	if n == nil {
		return strings.Repeat(" ", w)
	}
	return fmt.Sprintf("%*d", w, *n)
}

func signFor(kind diff.LineKind) string {
	switch kind {
	case diff.LineAddition:
		return "+"
	case diff.LineDeletion:
		return "-"
	default:
		return " "
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// clip truncates s to at most w display cells (rune-approximate).
func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > w {
		r = r[:len(r)-1]
	}
	return string(r)
}

// pad right-pads s with spaces to width w.
func pad(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}
