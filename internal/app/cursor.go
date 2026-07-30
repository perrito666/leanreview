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
	for m.cursor >= 0 && m.cursor < len(rows) && rows[m.cursor].Annotation {
		m.cursor += dir
	}
	m.clampCursor()
	// If clamping landed on an annotation (file edge), back off the other way.
	for m.cursor > 0 && m.cursor < len(rows) && rows[m.cursor].Annotation {
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

// firstLine / lastLine jump to the extremes.
func (m *Model) firstLine() { m.cursor = 0; m.clampCursor() }
func (m *Model) lastLine()  { m.cursor = len(m.rows()) - 1; m.clampCursor() }

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

// nextHunk / prevHunk move to hunk-header rows.
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

// nextFile / prevFile switch files, resetting cursor and scroll.
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

func (m *Model) gotoFile(idx int) {
	if idx < 0 || idx >= len(m.files) {
		return
	}
	m.fileIdx = idx
	m.onFileChange()
}

func (m *Model) onFileChange() {
	m.top = 0
	m.hscroll = 0
	m.selAnchor = -1
	m.mode = ModeNormal
	m.cursor = m.firstContentRow()
	m.clampCursor()
}

func isChange(r *diff.DisplayRow) bool {
	if r.Source == nil {
		return false
	}
	k := kindOf(r)
	return k == diff.LineAddition || k == diff.LineDeletion
}

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
