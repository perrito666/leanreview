package app

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
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
	base := m.foldedRows()
	if m.contextActive() {
		// Folding is a diff-view concept. The context projection now carries
		// header rows too, and letting the fold filter see them would let a
		// hunk folded in the diff view silently swallow context content.
		base = m.rawRows()
	}
	visible := m.wrapRows(base)
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
		// Image markup renders as its own rows; showing the raw tag text too
		// (GitHub's <img width=... src=...> blobs especially) is pure noise.
		s = stripImageMarkup(s)
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
		at     string // RFC 3339 sorts lexically = chronologically
		lines  []annLine
		images []string // image references found in the bodies
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
				lines := splitBodyLines("● "+author, body(c.Body))
				if state != "" && len(lines) > 0 {
					lines[0].text += state
				}
				images := imageRefs(c.Body)
				for _, rp := range c.Replies {
					who := rp.Author
					if who == "" {
						who = "reply"
					}
					lines = append(lines, splitBodyLines(fmt.Sprintf("  ↳ @%s: ", who), body(rp.Body))...)
					images = append(images, imageRefs(rp.Body)...)
				}
				items = append(items, item{at: c.At, lines: lines, images: images})
			}
		}
		// Existing review threads (PR mode), with their replies inline.
		if m.pr != nil {
			for _, ti := range m.threadIndex[locKey(src.Path, src.Side, src.StartLine)] {
				th := m.pr.Threads[ti]
				lines := splitBodyLines(fmt.Sprintf("◆ @%s: ", th.Root.Author), body(th.Root.Body))
				images := imageRefs(th.Root.Body)
				for _, rp := range th.Replies {
					lines = append(lines, splitBodyLines(fmt.Sprintf("  ↳ @%s: ", rp.Author), body(rp.Body))...)
					images = append(images, imageRefs(rp.Body)...)
				}
				at := ""
				if !th.Root.CreatedAt.IsZero() {
					at = th.Root.CreatedAt.UTC().Format(time.RFC3339)
				}
				items = append(items, item{at: at, lines: lines, images: images})
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
		for _, ln := range it.lines {
			switch ln.kind {
			case annSuggestLabel:
				rows = append(rows, diff.DisplayRow{
					Left:       &diff.DisplayCell{Kind: diff.LineMetadata, Text: m.theme.Faint.Render("╌╌ " + ln.text + " ╌╌")},
					Annotation: true,
					Pre:        true,
				})
			case annSuggestCode:
				// Suggested code renders like an addition — it IS the
				// proposed new content — and never wraps (it is code).
				rows = append(rows, diff.DisplayRow{
					Left:       &diff.DisplayCell{Kind: diff.LineMetadata, Text: m.theme.Addition.Render("+ " + ln.text)},
					Annotation: true,
					Pre:        true,
				})
			default:
				for _, line := range m.wrapAnnotation(ln.text) {
					rows = append(rows, diff.DisplayRow{
						Left:       &diff.DisplayCell{Kind: diff.LineMetadata, Text: line},
						Annotation: true,
					})
				}
			}
		}
		rows = append(rows, m.imageRows(it.images)...)
	}
	rows = append(rows, diff.DisplayRow{
		Left:       &diff.DisplayCell{Kind: diff.LineMetadata},
		Annotation: true,
		Edge:       diff.EdgeBottom,
	})
	return rows
}

// imageRefTargets caps how many images render per thread item — a wall of
// screenshots would drown the diff the comment is about.
const maxImagesPerItem = 3

// imageRefs extracts image targets from a comment body — both Markdown
// (![alt](src)) and raw HTML <img src="..."> tags, which is how GitHub
// embeds pasted attachments in review comments. Local paths resolve
// relative to the working directory; remote URLs render only when a forge
// attachment fetcher exists (PR mode), and as tags otherwise.
func imageRefs(body string) []string {
	var out []string
	for _, m := range imageRefRe.FindAllStringSubmatch(body, maxImagesPerItem) {
		out = append(out, m[1])
	}
	for _, m := range htmlImgRe.FindAllStringSubmatch(body, maxImagesPerItem-len(out)) {
		out = append(out, m[1])
	}
	if len(out) > maxImagesPerItem {
		out = out[:maxImagesPerItem]
	}
	return out
}

// stripImageMarkup removes image syntax from display text — the image (or
// its tag) renders as its own rows, and a raw <img width=... src=...> blob
// wrapped across box lines is exactly the noise this replaces. The alt text
// survives when present.
func stripImageMarkup(body string) string {
	body = imageRefAltRe.ReplaceAllString(body, "$1")
	body = htmlImgRe.ReplaceAllStringFunc(body, func(tag string) string {
		// GitHub stamps alt="Image" on every paste; repeating that above the
		// image row (or its tag) is pure duplication — keep only alts that
		// say something.
		if m := htmlAltRe.FindStringSubmatch(tag); m != nil && m[1] != "" && !strings.EqualFold(m[1], "image") {
			return "[" + m[1] + "]"
		}
		return ""
	})
	return strings.TrimSpace(body)
}

var (
	imageRefRe    = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)\)`)
	imageRefAltRe = regexp.MustCompile(`!(\[[^\]]*\])\(([^)\s]+)\)`)
	htmlImgRe     = regexp.MustCompile(`<img[^>]*\bsrc="([^"]+)"[^>]*/?>`)
	htmlAltRe     = regexp.MustCompile(`\balt="([^"]*)"`)
)

// imageRows renders each referenced image into preformatted box rows, or a
// textual tag when the image cannot (or must not) be rendered: remote URLs,
// missing files, and terminals with images off all degrade to the tag rather
// than to silence — the reader should always learn the image exists.
func (m *Model) imageRows(refs []string) []diff.DisplayRow {
	var rows []diff.DisplayRow
	_, inner := m.annotationLayout()
	for _, ref := range refs {
		suffix := ""
		tag := func() {
			rows = append(rows, diff.DisplayRow{
				Left:       &diff.DisplayCell{Kind: diff.LineMetadata, Text: "  [image: " + ref + suffix + "]"},
				Annotation: true,
			})
		}
		if !m.images.Enabled() {
			// Say WHY it is a tag — "install chafa" is the fix most users
			// need and would otherwise never learn from a bare tag.
			suffix = " — no image renderer; install chafa or use kitty/ghostty"
			tag()
			continue
		}
		path := ref
		if isRemoteRef(ref) {
			// Forge attachments are fetched (authenticated, cached) into
			// local files; anything not yet — or not fetchable — stays a tag.
			local, state := m.imageFiles[ref], ""
			switch {
			case local != "":
				path = local
			case m.imagePending[ref]:
				state = " (fetching…)"
			}
			if path == ref {
				suffix = state
				tag()
				continue
			}
		}
		lines, ok := m.images.Render(path, inner-2, 12)
		if !ok {
			tag()
			continue
		}
		for _, ln := range lines {
			rows = append(rows, diff.DisplayRow{
				Left:       &diff.DisplayCell{Kind: diff.LineMetadata, Text: "  " + ln},
				Annotation: true,
				Pre:        true,
			})
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
		if r.Pre {
			// Preformatted (image) rows carry their own escapes: clip
			// ANSI-aware, pad to the frame, restyle nothing.
			text := pad(ansi.Truncate(r.Left.Text, inner, ""), inner)
			return pad(pre+side+" "+text+" "+side, cw)
		}
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
