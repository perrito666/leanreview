package app

import (
	"fmt"

	"github.com/perrito666/leanreview/internal/diff"
)

// sidebarWidth is the fixed column width of the changed-files sidebar.
const sidebarWidth = 28

// effSidebar reports whether the sidebar is actually shown: it is suppressed on
// narrow terminals so the diff keeps usable width.
func (m *Model) effSidebar() bool {
	return m.sidebar && m.width >= 70
}

// contentWidth is the width available to the diff body (excludes the sidebar and
// its separator when shown).
func (m *Model) contentWidth() int {
	if m.effSidebar() {
		w := m.width - sidebarWidth - 1
		if w < 1 {
			w = 1
		}
		return w
	}
	return m.width
}

// toggleSidebar shows or hides the changed-files sidebar.
func (m *Model) toggleSidebar() {
	m.sidebar = !m.sidebar
	m.clampCursor()
	if m.sidebar {
		m.setStatus("file sidebar shown")
	} else {
		m.setStatus("file sidebar hidden")
	}
}

// sidebarLines renders exactly h lines of the changed-files list at sidebarWidth,
// windowed so the current file stays visible.
func (m *Model) sidebarLines(h int) []string {
	w := sidebarWidth
	lines := make([]string, 0, h)
	lines = append(lines, m.theme.Metadata.Render(pad(clip("Changed files", w), w)))

	avail := h - 1
	if avail < 1 {
		return padLines(lines, h, w)
	}
	start := 0
	if len(m.files) > avail {
		start = m.fileIdx - avail/2
		if start < 0 {
			start = 0
		}
		if start > len(m.files)-avail {
			start = len(m.files) - avail
		}
	}
	for i := start; i < len(m.files) && len(lines) < h; i++ {
		f := &m.files[i]
		badge := ""
		if n := m.commentCountForPath(f.Path()); n > 0 {
			badge = fmt.Sprintf(" ●%d", n)
		}
		text := clip(fmt.Sprintf("%s %s%s", statusGlyph(f.Status), f.Path(), badge), w)
		if i == m.fileIdx {
			lines = append(lines, m.theme.Cursor.Render(pad(text, w)))
		} else {
			lines = append(lines, pad(text, w))
		}
	}
	return padLines(lines, h, w)
}

func padLines(lines []string, h, w int) []string {
	for len(lines) < h {
		lines = append(lines, pad("", w))
	}
	return lines[:h]
}

// statusGlyph returns a single-character marker for a file status.
func statusGlyph(s diff.FileStatus) string {
	switch s {
	case diff.StatusAdded:
		return "A"
	case diff.StatusDeleted:
		return "D"
	case diff.StatusRenamed:
		return "R"
	case diff.StatusCopied:
		return "C"
	case diff.StatusBinary:
		return "B"
	default:
		return "M"
	}
}
