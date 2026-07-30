package app

import "github.com/perrito666/leanreview/internal/diff"

// clampCursor keeps the cursor within the current file's rows and adjusts the
// scroll offset so the cursor stays visible.
func (m *Model) clampCursor() {
	rows := m.rows()
	if len(rows) == 0 {
		m.cursor, m.top = 0, 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(rows) {
		m.cursor = len(rows) - 1
	}
	h := m.contentHeight()
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+h {
		m.top = m.cursor - h + 1
	}
	if m.top < 0 {
		m.top = 0
	}
}

// moveLine moves the cursor by delta rows, gliding over display-only
// annotation rows in the direction of travel.
func (m *Model) moveLine(delta int) {
	m.cursor += delta
	dir := 1
	if delta < 0 {
		dir = -1
	}
	rows := m.rows()
	skip := func(i int) bool { return rows[i].Annotation || rows[i].Continuation }
	for m.cursor >= 0 && m.cursor < len(rows) && skip(m.cursor) {
		m.cursor += dir
	}
	m.clampCursor()
	// If clamping landed on a display-only row (file edge), back off the other way.
	for m.cursor > 0 && m.cursor < len(rows) && skip(m.cursor) {
		m.cursor -= dir
	}
}

// halfPage moves the cursor by half the visible height.
func (m *Model) halfPage(sign int) {
	m.moveLine(sign * (m.contentHeight() / 2))
}

// fullPage moves the cursor by a full visible page.
func (m *Model) fullPage(sign int) {
	m.moveLine(sign * m.contentHeight())
}

// firstLine jumps the cursor to the first row of the file.
func (m *Model) firstLine() { m.cursor = 0; m.clampCursor() }

// lastLine jumps the cursor to the last row of the file, mirroring firstLine;
// the clampCursor call is what actually scrolls the viewport to the tail.
func (m *Model) lastLine() { m.cursor = len(m.rows()) - 1; m.clampCursor() }

// nextChange moves to the next row that is an addition or deletion.
func (m *Model) nextChange(n int) {
	rows := m.rows()
	for ; n > 0; n-- {
		for i := m.cursor + 1; i < len(rows); i++ {
			if isChange(&rows[i]) {
				m.cursor = i
				break
			}
		}
	}
	m.clampCursor()
}

// prevChange moves to the previous added/deleted row, mirroring nextChange;
// n repeats the motion for count-prefixed keys. When no earlier change exists
// the cursor simply stays put.
func (m *Model) prevChange(n int) {
	rows := m.rows()
	for ; n > 0; n-- {
		for i := m.cursor - 1; i >= 0; i-- {
			if isChange(&rows[i]) {
				m.cursor = i
				break
			}
		}
	}
	m.clampCursor()
}

// nextHunk moves to the next hunk-header row, then settles on the first
// content row after it (see skipToContent).
func (m *Model) nextHunk(n int) {
	rows := m.rows()
	for ; n > 0; n-- {
		for i := m.cursor + 1; i < len(rows); i++ {
			if isHeader(&rows[i]) {
				m.cursor = i
				break
			}
		}
	}
	// Prefer the first content row after the header.
	m.skipToContent(1)
	m.clampCursor()
}

// prevHunk mirrors nextHunk backwards: it lands on the previous hunk header,
// then settles on the first content row after it so the cursor rests on a
// commentable line rather than the header itself.
func (m *Model) prevHunk(n int) {
	rows := m.rows()
	for ; n > 0; n-- {
		for i := m.cursor - 1; i >= 0; i-- {
			if isHeader(&rows[i]) {
				m.cursor = i
				break
			}
		}
	}
	m.skipToContent(1)
	m.clampCursor()
}

// skipToContent nudges the cursor off a header row in the given direction.
func (m *Model) skipToContent(dir int) {
	rows := m.rows()
	for m.cursor >= 0 && m.cursor < len(rows) && rows[m.cursor].Source == nil {
		next := m.cursor + dir
		if next < 0 || next >= len(rows) {
			return
		}
		m.cursor = next
	}
}

// nextFile advances n files, clamping at the last one; onFileChange resets
// the per-file view state.
func (m *Model) nextFile(n int) {
	if len(m.files) == 0 {
		return
	}
	m.fileIdx += n
	if m.fileIdx >= len(m.files) {
		m.fileIdx = len(m.files) - 1
	}
	m.onFileChange()
}

// prevFile mirrors nextFile in the other direction, clamping at the first
// file rather than wrapping.
func (m *Model) prevFile(n int) {
	if len(m.files) == 0 {
		return
	}
	m.fileIdx -= n
	if m.fileIdx < 0 {
		m.fileIdx = 0
	}
	m.onFileChange()
}

// gotoFile jumps straight to file idx (used by the file list and comment
// jumps). Out-of-range indices are ignored so stale list cursors are harmless.
func (m *Model) gotoFile(idx int) {
	if idx < 0 || idx >= len(m.files) {
		return
	}
	m.fileIdx = idx
	m.onFileChange()
}

// onFileChange resets all per-file view state (scroll, selection, mode) after
// any file switch, so the new file always opens at its first commentable row
// with nothing carried over from the previous one.
func (m *Model) onFileChange() {
	m.top = 0
	m.hscroll = 0
	m.selAnchor = -1
	m.mode = ModeNormal
	m.cursor = m.firstContentRow()
	m.clampCursor()
}

// isChange reports whether a row is actual changed content (an addition or
// deletion). Headers and display-only rows carry no Source and never count,
// which is what lets the change motions skip them.
func isChange(r *diff.DisplayRow) bool {
	if r.Source == nil {
		return false
	}
	k := kindOf(r)
	return k == diff.LineAddition || k == diff.LineDeletion
}

// isHeader identifies hunk-header rows: metadata carried in the Left cell with
// no Source. The Annotation check matters because inline comment previews also
// lack a Source and must not be mistaken for headers.
func isHeader(r *diff.DisplayRow) bool {
	return !r.Annotation && r.Source == nil && r.Left != nil && r.Left.Kind == diff.LineMetadata
}

// kindOf reports the diff kind a row represents (preferring the populated side).
func kindOf(r *diff.DisplayRow) diff.LineKind {
	if r.Right != nil && r.Right.LineNumber != nil {
		return r.Right.Kind
	}
	if r.Left != nil {
		return r.Left.Kind
	}
	if r.Right != nil {
		return r.Right.Kind
	}
	return diff.LineContext
}
