package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wrap"

	"github.com/perrito666/leanreview/internal/diff"
)

// unifiedTextWidth is the number of columns available to line text in the
// unified layout (after the comment gutter and the number gutter), capped by
// the configured wrap width — the cap the user reads code at, independent of
// how wide the terminal happens to be.
func (m *Model) unifiedTextWidth() int {
	w := m.contentWidth() - 2 - (2*m.numWidth() + 4)
	if m.wrapWidth > 0 && m.wrapWidth < w {
		w = m.wrapWidth
	}
	if w < 1 {
		w = 1
	}
	return w
}

// splitPanelWidth is the text width of one side panel in the split layout —
// the wrap point for both diff lines and comments in that layout.
func (m *Model) splitPanelWidth() int {
	nw := m.numWidth()
	half := (m.contentWidth() - 2 - (nw * 2) - 6) / 2
	if half < 4 {
		half = 4
	}
	return half
}

// hardWrap splits s into display-width chunks of at most w columns. Code is
// wrapped hard at the column edge (not at word boundaries) so indentation and
// alignment of the visible part stay untouched.
func hardWrap(s string, w int) []string {
	if s == "" || lipgloss.Width(s) <= w {
		return []string{s}
	}
	return strings.Split(wrap.String(s, w), "\n")
}

// wrapRows expands rows whose text overflows the layout's wrap point into
// continuation rows. The first row keeps the line numbers and Source; the
// continuations render in the line's style but are invisible to navigation.
func (m *Model) wrapRows(rows []diff.DisplayRow) []diff.DisplayRow {
	if !m.wrapText {
		return rows
	}
	var width int
	if m.layout == LayoutSplit {
		width = m.splitPanelWidth()
	} else {
		width = m.unifiedTextWidth()
	}

	out := make([]diff.DisplayRow, 0, len(rows))
	for i := range rows {
		r := rows[i]
		if r.Annotation || isHeader(&r) {
			out = append(out, r)
			continue
		}
		var lParts, rParts []string
		if r.Left != nil {
			lParts = hardWrap(r.Left.Text, width)
		}
		if r.Right != nil {
			rParts = hardWrap(r.Right.Text, width)
		}
		n := len(lParts)
		if len(rParts) > n {
			n = len(rParts)
		}
		if n <= 1 {
			out = append(out, r)
			continue
		}

		for k := 0; k < n; k++ {
			row := diff.DisplayRow{}
			if k == 0 {
				row = r // numbers, Source, AltSource on the first row only
			} else {
				row.Continuation = true
			}
			if r.Left != nil && k < len(lParts) {
				cell := diff.DisplayCell{Kind: r.Left.Kind, Text: lParts[k]}
				if k == 0 {
					cell.LineNumber = r.Left.LineNumber
				}
				row.Left = &cell
			} else if r.Left != nil && k > 0 {
				row.Left = &diff.DisplayCell{Kind: r.Left.Kind}
			}
			if r.Right != nil && k < len(rParts) {
				cell := diff.DisplayCell{Kind: r.Right.Kind, Text: rParts[k]}
				if k == 0 {
					cell.LineNumber = r.Right.LineNumber
				}
				row.Right = &cell
			} else if r.Right != nil && k > 0 {
				row.Right = &diff.DisplayCell{Kind: r.Right.Kind}
			}
			out = append(out, row)
		}
	}
	return out
}

// toggleWrap switches wrapping for diff lines and comment previews.
func (m *Model) toggleWrap() {
	m.wrapText = !m.wrapText
	m.hscroll = 0
	m.clampCursor()
	if m.wrapText {
		m.setStatus("wrapping on (w to disable)")
	} else {
		m.setStatus("wrapping off — long lines clip; h/l scroll")
	}
}
