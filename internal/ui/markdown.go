package ui

import (
	"regexp"
	"strings"

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
)

// RenderMarkdown renders a small Markdown subset for terminal display: ATX
// headings, fenced code blocks, block quotes, thematic breaks, bullet lists,
// and inline code/link/bold spans. It wraps prose at width (fenced code is
// left verbatim) and returns the styled lines. It is deliberately not a full
// Markdown implementation — unknown constructs pass through as plain text.
func RenderMarkdown(src string, width int, th Theme) []string {
	if width < 10 {
		width = 10
	}
	var out []string
	inFence := false
	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
			inFence = !inFence
			out = append(out, th.Faint.Render(trimmed))
		case inFence:
			out = append(out, "  "+line)
		case headingRe.MatchString(trimmed):
			out = append(out, th.Metadata.Render(trimmed))
		case hrRe.MatchString(trimmed):
			out = append(out, th.Faint.Render(strings.Repeat("─", width)))
		case strings.HasPrefix(trimmed, ">"):
			text := strings.TrimPrefix(strings.TrimPrefix(trimmed, ">"), " ")
			for _, ln := range wrapText(text, width-2) {
				out = append(out, th.Faint.Render("▏ "+ln))
			}
		default:
			if m := bulletRe.FindStringSubmatch(line); m != nil {
				line = m[1] + "• " + line[len(m[0]):]
			}
			for _, ln := range wrapText(line, width) {
				out = append(out, styleInline(ln, th))
			}
		}
	}
	return out
}

var (
	headingRe = regexp.MustCompile(`^#{1,6}\s+\S`)
	hrRe      = regexp.MustCompile(`^(-{3,}|\*{3,}|_{3,})$`)
	bulletRe  = regexp.MustCompile(`^(\s*)[-*+]\s+`)
	codeRe    = regexp.MustCompile("`([^`]+)`")
	linkRe    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	boldRe    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
)

// wrapText word-wraps s at width with a hard-wrap fallback for long tokens,
// preserving blank lines as empty entries.
func wrapText(s string, w int) []string {
	if w < 1 {
		w = 1
	}
	return strings.Split(wrap.String(wordwrap.String(s, w), w), "\n")
}

// styleInline colorizes inline code, links, and bold spans within one line.
// Spans split across wrapped lines are left as-is.
func styleInline(s string, th Theme) string {
	s = codeRe.ReplaceAllStringFunc(s, func(m string) string {
		return th.Key.Render(strings.Trim(m, "`"))
	})
	s = linkRe.ReplaceAllStringFunc(s, func(m string) string {
		g := linkRe.FindStringSubmatch(m)
		return th.Key.Render(g[1]) + th.Faint.Render(" ("+g[2]+")")
	})
	s = boldRe.ReplaceAllStringFunc(s, func(m string) string {
		return th.Metadata.Render(strings.Trim(m, "*"))
	})
	return s
}
