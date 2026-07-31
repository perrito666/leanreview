package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

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

// annotationRows builds the inline preview rows for one diff row: every draft
// comment and review thread anchored on it (either side of a split row),
// merged into one containing box and ordered oldest first so the line's
// discussion reads as a single thread. With wrapping on, bodies word-wrap —
// at the side panel's width in split layout, at the configured wrap width in
// unified; with wrapping off, only each item's first line is shown (clipped).
func (m *Model) annotationRows(r *diff.DisplayRow) []diff.DisplayRow {
	body := func(s string) string {
		if m.wrapText {
			return strings.TrimRight(s, "\n")
		}
		return firstLine(s)
	}

	// Gather every item anchored to this row — draft comments and existing
	// review threads alike — as (sort key, rendered lines) pairs. They all
	// share ONE containing box, ordered oldest first, so the line's whole
	// discussion reads as a single thread instead of stacked fragments.
	type item struct {
		at    string // RFC 3339 sorts lexically = chronologically
		texts []string
	}
	var items []item

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
				for _, rp := range c.Replies {
					who := rp.Author
					if who == "" {
						who = "reply"
					}
					texts = append(texts, fmt.Sprintf("  ↳ @%s: %s", who, body(rp.Body)))
				}
				items = append(items, item{at: c.At, texts: texts})
			}
		}
		// Existing review threads (PR mode), with their replies inline.
		if m.pr != nil {
			for _, ti := range m.threadIndex[locKey(src.Path, src.Side, src.StartLine)] {
				th := m.pr.Threads[ti]
				texts := []string{fmt.Sprintf("◆ @%s: %s", th.Root.Author, body(th.Root.Body))}
				for _, rp := range th.Replies {
					texts = append(texts, fmt.Sprintf("  ↳ @%s: %s", rp.Author, body(rp.Body)))
				}
				at := ""
				if !th.Root.CreatedAt.IsZero() {
					at = th.Root.CreatedAt.UTC().Format(time.RFC3339)
				}
				items = append(items, item{at: at, texts: texts})
			}
		}
	}
	if len(items) == 0 {
		return nil
	}
	// Oldest first; stable so undated items keep their anchor order.
	sort.SliceStable(items, func(i, j int) bool { return items[i].at < items[j].at })

	// One box: top edge, items separated by an inner divider, bottom edge.
	rows := []diff.DisplayRow{{
		Left:       &diff.DisplayCell{Kind: diff.LineMetadata},
		Annotation: true,
		Edge:       diff.EdgeTop,
	}}
	for i, it := range items {
		if i > 0 {
			rows = append(rows, diff.DisplayRow{
				Left:       &diff.DisplayCell{Kind: diff.LineMetadata},
				Annotation: true,
				Edge:       diff.EdgeDivider,
			})
		}
		for _, text := range it.texts {
			for _, line := range m.wrapAnnotation(text) {
				rows = append(rows, diff.DisplayRow{
					Left:       &diff.DisplayCell{Kind: diff.LineMetadata, Text: line},
					Annotation: true,
				})
			}
		}
	}
	rows = append(rows, diff.DisplayRow{
		Left:       &diff.DisplayCell{Kind: diff.LineMetadata},
		Annotation: true,
		Edge:       diff.EdgeBottom,
	})
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

// renderAnnotation draws one row of a boxed comment preview: a border edge,
// an inner thread divider, or a text row framed by the box sides. In split
// layout the panel divider is drawn through the box's indent so the two-pane
// geometry stays visually continuous instead of being interrupted by the
// thread.
func (m *Model) renderAnnotation(r *diff.DisplayRow, isCursor bool) string {
	indent, inner := m.annotationLayout()
	cw := m.contentWidth()
	pre := strings.Repeat(" ", indent)
	if m.layout == LayoutSplit {
		// The split divider column sits two cells before the box; keep the
		// vertical line flowing through annotation rows.
		div := 2 + m.numWidth() + 1 + m.splitPanelWidth() + 1
		if div+2 <= indent {
			pre = strings.Repeat(" ", div) + m.theme.Faint.Render("│") + strings.Repeat(" ", indent-div-1)
		}
	}

	if isCursor {
		// The cursor never rests here, but stay defensive and legible.
		return m.theme.Cursor.Render(pad(strings.Repeat(" ", indent)+clip(r.Left.Text, cw-indent), cw))
	}
	switch r.Edge {
	case diff.EdgeTop:
		return pad(pre+m.theme.Faint.Render("╭"+strings.Repeat("─", inner+2)+"╮"), cw)
	case diff.EdgeBottom:
		return pad(pre+m.theme.Faint.Render("╰"+strings.Repeat("─", inner+2)+"╯"), cw)
	case diff.EdgeDivider:
		return pad(pre+m.theme.Faint.Render("├"+strings.Repeat("┄", inner+2)+"┤"), cw)
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
