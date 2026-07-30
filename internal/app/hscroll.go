package app

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// hscrollStep is how many columns the left/right motions shift the view.
const hscrollStep = 8

// gutterCols is the fixed number of columns the line-number gutter and sign
// occupy in the unified layout (two numbers of width nw, three spaces, a sign).
func (m *Model) gutterCols() int {
	return 2*m.numWidth() + 4
}

// textAvail is the number of columns available for line text after the gutter.
func (m *Model) textAvail() int {
	a := m.contentWidth() - m.gutterCols()
	if a < 1 {
		a = 1
	}
	return a
}

// maxHScroll is the furthest useful horizontal offset: the longest visible line
// beyond the available text width.
func (m *Model) maxHScroll() int {
	longest := 0
	for _, r := range m.rows() {
		if r.Right != nil {
			if w := lipgloss.Width(r.Right.Text); w > longest {
				longest = w
			}
		}
		if r.Left != nil {
			if w := lipgloss.Width(r.Left.Text); w > longest {
				longest = w
			}
		}
	}
	if max := longest - m.textAvail(); max > 0 {
		return max
	}
	return 0
}

func (m *Model) scrollLeft() {
	m.hscroll -= hscrollStep
	if m.hscroll < 0 {
		m.hscroll = 0
	}
}

func (m *Model) scrollRight() {
	m.hscroll += hscrollStep
	if max := m.maxHScroll(); m.hscroll > max {
		m.hscroll = max
	}
}

func (m *Model) lineStart() { m.hscroll = 0 }
func (m *Model) lineEnd()   { m.hscroll = m.maxHScroll() }

// hcut drops the first m.hscroll display columns of s, ANSI-aware so it works on
// both plain and syntax-highlighted text. The number gutter is added separately
// and so stays fixed while the text scrolls under it.
func (m *Model) hcut(s string) string {
	if m.hscroll <= 0 {
		return s
	}
	return ansi.TruncateLeft(s, m.hscroll, "")
}
