package app

import (
	"fmt"
	"strings"

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"

	"github.com/perrito666/leanreview/internal/diff"
)

// rows returns the visible display rows for the current file: the fold-filtered
// projection, wrapped to the layout's wrap point when wrapping is on (w), with
// inline comment previews injected under annotated lines when enabled (i).
// Annotation and continuation rows are display-only: they carry no Source, so
// selection and navigation skip them, and one row still equals one screen line
// for scrolling.
func (m *Model) rows() []diff.DisplayRow {
	visible := m.wrapRows(m.foldedRows())
	if !m.inlineComments || m.draft == nil {
		return visible
	}
	out := make([]diff.DisplayRow, 0, len(visible))
	for i := 0; i < len(visible); i++ {
		out = append(out, visible[i])
		ann := m.annotationRows(&visible[i])
		// A wrapped line's annotations belong after its continuation rows.
		for i+1 < len(visible) && visible[i+1].Continuation {
			i++
			out = append(out, visible[i])
		}
		out = append(out, ann...)
	}
	return out
}

// annotationRows builds the inline preview rows for one diff row: one preview
// per draft comment anchored on it (on either side of a split row) and one per
// existing review thread. With wrapping on, the full body is word-wrapped —
// at the side panel's width in split layout, at the configured wrap width in
// unified; with wrapping off, only the first line is shown (clipped).
func (m *Model) annotationRows(r *diff.DisplayRow) []diff.DisplayRow {
	var rows []diff.DisplayRow
	add := func(text string) {
		for _, line := range m.wrapAnnotation(text) {
			rows = append(rows, diff.DisplayRow{
				Left:       &diff.DisplayCell{Kind: diff.LineMetadata, Text: line},
				Annotation: true,
			})
		}
	}
	body := func(s string) string {
		if m.wrapText {
			return strings.TrimRight(s, "\n")
		}
		return firstLine(s)
	}

	for _, src := range []*diff.Location{r.Source, r.AltSource} {
		if src == nil {
			continue
		}
		// Drafts: anchor the preview at the comment's end line so a multi-line
		// comment appears once, under its last covered row.
		for i := range m.draft.Comments {
			c := &m.draft.Comments[i]
			if c.Location.Path == src.Path && c.Location.Side == src.Side && c.Location.EndLine == src.StartLine {
				state := ""
				if c.State != 0 {
					state = " [" + c.State.String() + "]"
				}
				add(fmt.Sprintf("● %s%s", body(c.Body), state))
			}
		}
		// Existing review threads (PR mode).
		if m.pr != nil {
			for _, ti := range m.threadIndex[locKey(src.Path, src.Side, src.StartLine)] {
				th := m.pr.Threads[ti]
				extra := ""
				if n := len(th.Replies); n > 0 {
					extra = fmt.Sprintf("  (+%d repl%s)", n, plural(n, "y", "ies"))
				}
				add(fmt.Sprintf("◆ @%s: %s%s", th.Root.Author, body(th.Root.Body), extra))
			}
		}
	}
	return rows
}

// wrapAnnotation word-wraps a comment preview to the layout's wrap point,
// indenting continuation lines under the text of the first. Prose wraps at
// word boundaries (unlike code, which wraps hard at the column).
func (m *Model) wrapAnnotation(text string) []string {
	if !m.wrapText {
		return []string{text}
	}
	var width int
	if m.layout == LayoutSplit {
		width = m.splitPanelWidth()
	} else {
		width = m.unifiedTextWidth()
	}
	if width <= 2 {
		return []string{text}
	}
	wrapped := wordwrap.String(text, width)
	// Guard against unbroken tokens longer than the width.
	wrapped = wrap.String(wrapped, width)
	lines := strings.Split(wrapped, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = "  " + lines[i]
	}
	return lines
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// toggleInlineComments shows or hides the inline previews.
func (m *Model) toggleInlineComments() {
	m.inlineComments = !m.inlineComments
	m.clampCursor()
	if m.inlineComments {
		m.setStatus("inline comments shown")
	} else {
		m.setStatus("inline comments hidden (i to show)")
	}
}
