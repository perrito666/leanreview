package app

import (
	"fmt"

	"github.com/perrito666/leanreview/internal/diff"
)

// foldKey builds the m.folded map key for one hunk. Folds are keyed by file
// and hunk index so a hunk stays folded while the user moves between files.
func foldKey(fileIdx, hunkIdx int) string {
	return fmt.Sprintf("%d/%d", fileIdx, hunkIdx)
}

// isFolded reports whether the given hunk of the current file is folded;
// absent keys read as false, so hunks start out expanded.
func (m *Model) isFolded(hunkIdx int) bool {
	return m.folded[foldKey(m.fileIdx, hunkIdx)]
}

// foldedRows returns the fold-filtered display rows for the current file: a
// folded hunk collapses to just its header row (annotated with a count of
// hidden lines). Headers are prefixed with a fold indicator either way.
func (m *Model) foldedRows() []diff.DisplayRow {
	raw := m.rawRows()
	if len(raw) == 0 {
		return raw
	}
	var out []diff.DisplayRow
	hunk := -1
	for i := 0; i < len(raw); i++ {
		r := raw[i]
		if !isHeader(&r) {
			out = append(out, r)
			continue
		}
		hunk++
		// Count the content rows belonging to this hunk.
		cnt := 0
		for j := i + 1; j < len(raw) && !isHeader(&raw[j]); j++ {
			cnt++
		}
		out = append(out, foldedHeader(r, m.isFolded(hunk), cnt))
		if m.isFolded(hunk) {
			i += cnt // skip the hidden content rows
		}
	}
	return out
}

// foldedHeader returns a copy of a header row annotated with a fold indicator.
func foldedHeader(r diff.DisplayRow, folded bool, hidden int) diff.DisplayRow {
	cell := *r.Left
	if folded {
		cell.Text = fmt.Sprintf("▸ %s  (%d lines)", r.Left.Text, hidden)
	} else {
		cell.Text = "▾ " + r.Left.Text
	}
	r.Left = &cell
	return r
}

// cursorHunkIndex returns the hunk index the cursor is currently within.
func (m *Model) cursorHunkIndex() int {
	rows := m.rows()
	hunk := -1
	for i := 0; i < len(rows) && i <= m.cursor; i++ {
		if isHeader(&rows[i]) {
			hunk++
		}
	}
	if hunk < 0 {
		hunk = 0
	}
	return hunk
}

// headerRowIndex returns the visible row index of the given hunk's header.
func (m *Model) headerRowIndex(hunk int) int {
	rows := m.rows()
	h := -1
	for i := range rows {
		if isHeader(&rows[i]) {
			h++
			if h == hunk {
				return i
			}
		}
	}
	return 0
}

// toggleFold folds or unfolds the hunk under the cursor, keeping the cursor on
// that hunk's header.
func (m *Model) toggleFold() {
	f := m.currentFile()
	if f == nil {
		return
	}
	if m.contextActive() {
		m.setStatus("folding applies to the diff view (T to return)")
		return
	}
	h := m.cursorHunkIndex()
	k := foldKey(m.fileIdx, h)
	m.folded[k] = !m.folded[k]
	m.cursor = m.headerRowIndex(h)
	m.clampCursor()
	if m.folded[k] {
		m.setStatus("hunk folded (za to unfold, zR to expand all)")
	} else {
		m.setStatus("hunk unfolded")
	}
}

// expandAll unfolds every hunk in the current file.
func (m *Model) expandAll() {
	f := m.currentFile()
	if f == nil {
		return
	}
	for i := range f.Hunks {
		delete(m.folded, foldKey(m.fileIdx, i))
	}
	m.clampCursor()
	m.setStatus("all hunks expanded")
}

// collapseAll folds every hunk in the current file.
func (m *Model) collapseAll() {
	f := m.currentFile()
	if f == nil {
		return
	}
	for i := range f.Hunks {
		m.folded[foldKey(m.fileIdx, i)] = true
	}
	m.cursor = 0
	m.clampCursor()
	m.setStatus("all hunks folded")
}
