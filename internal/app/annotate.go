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
	// Each preview renders as one bordered box: an edge row above and below
	// the wrapped text rows (see renderAnnotation for the drawing). A comment
	// and its conversation replies share a box — they are one exchange.
	add := func(texts ...string) {
		rows = append(rows, diff.DisplayRow{
			Left:       &diff.DisplayCell{Kind: diff.LineMetadata},
			Annotation: true,
			Edge:       diff.EdgeTop,
		})
		for _, text := range texts {
			for _, line := range m.wrapAnnotation(text) {
				rows = append(rows, diff.DisplayRow{
					Left:       &diff.DisplayCell{Kind: diff.LineMetadata, Text: line},
					Annotation: true,
				})
			}
		}
		rows = append(rows, diff.DisplayRow{
			Left:       &diff.DisplayCell{Kind: diff.LineMetadata},
			Annotation: true,
			Edge:       diff.EdgeBottom,
		})
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
				// Imported (exchange) comments carry an author worth showing;
				// the reviewer's own drafts do not.
				author := ""
				if c.Author != "" {
					author = "@" + c.Author + ": "
				}
				texts := []string{fmt.Sprintf("● %s%s%s", author, body(c.Body), state)}
				for _, r := range c.Replies {
					who := r.Author
					if who == "" {
						who = "reply"
					}
					texts = append(texts, fmt.Sprintf("  ↳ @%s: %s", who, body(r.Body)))
				}
				add(texts...)
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

// annotationLayout returns the left indent and inner text width of the comment
// box: under the text column in unified layout, over the right panel in split.
func (m *Model) annotationLayout() (indent, inner int) {
	cw := m.contentWidth()
	if m.layout == LayoutSplit {
		indent = 2 + m.numWidth() + 1 + m.splitPanelWidth() + 3
		inner = m.splitPanelWidth()
	} else {
		indent = 3
		inner = m.unifiedTextWidth()
	}
	// The box frame ("│ " + text + " │") must fit the content width.
	if indent+inner+4 > cw {
		inner = cw - indent - 4
	}
	if inner < 4 {
		inner = 4
		if indent > cw-inner-4 {
			indent = cw - inner - 4
		}
		if indent < 0 {
			indent = 0
		}
	}
	return indent, inner
}

// wrapAnnotation word-wraps a comment preview to the box's inner width,
// indenting continuation lines under the text of the first. Prose wraps at
// word boundaries (unlike code, which wraps hard at the column). With wrapping
// off, the single line is clipped by the box instead.
func (m *Model) wrapAnnotation(text string) []string {
	if !m.wrapText {
		return []string{text}
	}
	_, width := m.annotationLayout()
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

// renderAnnotation draws one row of a boxed comment preview: a border edge or
// a text row framed by the box sides.
func (m *Model) renderAnnotation(r *diff.DisplayRow, isCursor bool) string {
	indent, inner := m.annotationLayout()
	cw := m.contentWidth()
	pre := strings.Repeat(" ", indent)

	if isCursor {
		// The cursor never rests here, but stay defensive and legible.
		return m.theme.Cursor.Render(pad(pre+clip(r.Left.Text, cw-indent), cw))
	}
	switch r.Edge {
	case diff.EdgeTop:
		return pad(pre+m.theme.Faint.Render("╭"+strings.Repeat("─", inner+2)+"╮"), cw)
	case diff.EdgeBottom:
		return pad(pre+m.theme.Faint.Render("╰"+strings.Repeat("─", inner+2)+"╯"), cw)
	default:
		side := m.theme.Faint.Render("│")
		text := m.theme.Comment.Render(pad(clip(r.Left.Text, inner), inner))
		return pad(pre+side+" "+text+" "+side, cw)
	}
}

// plural picks the singular or plural form for a count, keeping preview text
// like "+2 replies" grammatical without pulling in a pluralisation library.
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
