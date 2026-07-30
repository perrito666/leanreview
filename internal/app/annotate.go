package app

import (
	"fmt"

	"github.com/perrito666/leanreview/internal/diff"
)

// rows returns the visible display rows for the current file: the fold-filtered
// projection, with inline comment previews injected under annotated lines when
// enabled (i toggles). Annotation rows are display-only: they carry no Source,
// so selection and navigation skip them, and one row still equals one screen
// line for scrolling.
func (m *Model) rows() []diff.DisplayRow {
	visible := m.foldedRows()
	if !m.inlineComments || m.draft == nil {
		return visible
	}
	out := make([]diff.DisplayRow, 0, len(visible))
	for i := range visible {
		out = append(out, visible[i])
		out = append(out, m.annotationRows(&visible[i])...)
	}
	return out
}

// annotationRows builds the inline preview rows for one diff row: one per draft
// comment anchored on it (on either side of a split row) and one per existing
// review thread.
func (m *Model) annotationRows(r *diff.DisplayRow) []diff.DisplayRow {
	var rows []diff.DisplayRow
	add := func(text string) {
		rows = append(rows, diff.DisplayRow{
			Left:       &diff.DisplayCell{Kind: diff.LineMetadata, Text: text},
			Annotation: true,
		})
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
				add(fmt.Sprintf("● %s%s", firstLine(c.Body), state))
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
				add(fmt.Sprintf("◆ @%s: %s%s", th.Root.Author, firstLine(th.Root.Body), extra))
			}
		}
	}
	return rows
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
