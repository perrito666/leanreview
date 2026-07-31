package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
)

// contextContentMsg delivers a fetched full file (or the failure) back to the
// update loop; fetching runs as a tea.Cmd so the TUI never blocks on git or
// the network.
type contextContentMsg struct {
	fileIdx    int
	side       diff.Side
	forContext bool // true: the context view requested it; false: highlighting
	data       []byte
	err        error
}

// contextActive reports whether the current file is being shown with its full
// surrounding context. The toggle is a session preference (m.contextView),
// but it only takes effect for files whose content has actually arrived —
// switching files never triggers an implicit fetch.
func (m *Model) contextActive() bool {
	if !m.contextView {
		return false
	}
	_, ok := m.contextRows[m.fileIdx]
	return ok
}

// toggleContext flips between the diff-only view and the full-file context
// view. The first request for a file fetches its content lazily (through the
// cmd-injected fetcher, which consults the on-disk cache); subsequent toggles
// reuse the session's rows. Either way the cursor keeps its semantic line and
// the viewport re-centers on it — the reader is asking for context *around*
// the line they are on.
func (m *Model) toggleContext() tea.Cmd {
	f := m.currentFile()
	if f == nil {
		return nil
	}
	if m.contextView {
		m.switchProjection(false)
		m.setStatus("diff view (T for full context)")
		return nil
	}
	if _, ok := m.contextRows[m.fileIdx]; ok {
		m.switchProjection(true)
		m.setStatus("full file context (T for diff only)")
		return nil
	}
	if m.fetchContext == nil {
		m.setStatus("full context is not available for this source")
		return nil
	}
	if m.layout == LayoutSplit {
		// Context is a unified-projection feature; switching implicitly would
		// be surprising, asking the user costs one keypress.
		m.setStatus("full context needs the unified layout (t to switch)")
		return nil
	}
	if data, ok := m.contentCache[contentKey(m.fileIdx, diff.SideRight)]; ok {
		// The highlight fetch already brought the content; project directly.
		m.onContextContent(contextContentMsg{fileIdx: m.fileIdx, side: diff.SideRight, forContext: true, data: data})
		return nil
	}
	m.setStatus("fetching %s…", f.Path())
	fi, path := m.fileIdx, f.Path()
	return func() tea.Msg {
		data, err := m.fetchContext(m.ctx, path, diff.SideRight)
		return contextContentMsg{fileIdx: fi, side: diff.SideRight, forContext: true, data: data, err: err}
	}
}

// onContextContent projects the fetched content and enables the context view.
// A projection failure (content from the wrong revision) is surfaced and the
// view stays on the diff — rendering a file that disagrees with the hunks
// would be worse than no context at all.
func (m *Model) onContextContent(msg contextContentMsg) {
	if msg.err != nil {
		// Highlight fetches fail silently (stitched fallback covers them);
		// an explicit context request deserves the error.
		if msg.forContext {
			m.setError(msg.err)
		}
		return
	}
	if msg.fileIdx < 0 || msg.fileIdx >= len(m.files) {
		return
	}
	// Either purpose feeds the shared content cache (and thereby the
	// whole-file highlight passes).
	m.contentCache[contentKey(msg.fileIdx, msg.side)] = msg.data
	if !msg.forContext {
		return
	}
	rows, err := diff.RenderUnifiedContext(&m.files[msg.fileIdx], msg.data)
	if err != nil {
		m.setError(err)
		return
	}
	m.contextRows[msg.fileIdx] = rows
	if msg.fileIdx == m.fileIdx {
		m.switchProjection(true)
		m.setStatus("full file context (T for diff only)")
	}
}

// switchProjection flips the context flag while re-anchoring the cursor to
// the semantic line it was on and centering the viewport there, so toggling
// reads as the file growing (or shrinking) around the line, not as a jump.
// The anchor must be captured from the OUTGOING projection — row indices mean
// nothing across the switch.
func (m *Model) switchProjection(enable bool) {
	anchor := m.rowAt(m.cursor)
	m.contextView = enable
	m.invalidateRows()
	if anchor != nil && anchor.Source != nil {
		m.reanchor(anchor.Source.Side, anchor.Source.StartLine)
	}
	m.centerOnCursor()
}

// centerOnCursor scrolls so the cursor row sits mid-viewport (clamped).
func (m *Model) centerOnCursor() {
	m.top = m.cursor - m.contentHeight()/2
	if m.top < 0 {
		m.top = 0
	}
	m.clampCursor()
}

// hunkAt returns the hunk index owning the nearest sourced row at or before i,
// or -1 before the first hunk — the context view has no header rows to count,
// so hunk identity comes from the rows' Sources.
func (m *Model) hunkAt(i int) int {
	rows := m.rows()
	for ; i >= 0; i-- {
		if i < len(rows) && rows[i].Source != nil {
			return rows[i].Source.HunkIndex
		}
	}
	return -1
}

// contextNextHunk moves to the first row of the n-th following hunk in the
// context view, keeping ]c/[c meaningful when the whole file is visible.
func (m *Model) contextNextHunk(n int) {
	rows := m.rows()
	cur := m.hunkAt(m.cursor)
	for i := m.cursor + 1; i < len(rows) && n > 0; i++ {
		if rows[i].Source != nil && rows[i].Source.HunkIndex > cur {
			cur = rows[i].Source.HunkIndex
			m.cursor = i
			n--
		}
	}
	m.centerOnCursor()
}

// contextPrevHunk mirrors contextNextHunk backwards, landing on the first row
// of the target hunk rather than its tail so repeated [c walks hunk starts.
func (m *Model) contextPrevHunk(n int) {
	rows := m.rows()
	for ; n > 0; n-- {
		cur := m.hunkAt(m.cursor)
		target := cur - 1
		if m.cursor < len(rows) && m.cursor >= 0 && rows[m.cursor].Source != nil && firstRowOfHunk(rows, cur) < m.cursor {
			// Mid-hunk: [c first goes to this hunk's start.
			target = cur
		}
		if target < 0 {
			break
		}
		if i := firstRowOfHunk(rows, target); i >= 0 {
			m.cursor = i
		}
	}
	m.centerOnCursor()
}

// firstRowOfHunk returns the index of the first row sourced from hunk h.
func firstRowOfHunk(rows []diff.DisplayRow, h int) int {
	for i := range rows {
		if rows[i].Source != nil && rows[i].Source.HunkIndex == h {
			return i
		}
	}
	return -1
}
