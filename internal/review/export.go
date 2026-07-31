package review

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/perrito666/leanreview/internal/diff"
)

// ExportMarkdown renders a draft review as Markdown grouped by file, suitable
// for pasting back as prompt feedback. Files appear in first-seen order; within
// a file, comments are ordered by their start line. Each comment shows a line
// reference, the diff snippet it was made on (in a fenced code block), and the
// note as a block quote.
func ExportMarkdown(d *DraftReview) string {
	var b strings.Builder

	title := d.Title
	if title == "" {
		title = "review"
	}
	fmt.Fprintf(&b, "# Review: %s\n\n", title)
	if strings.TrimSpace(d.Summary) != "" {
		b.WriteString(strings.TrimRight(d.Summary, "\n"))
		b.WriteString("\n\n")
	}

	if len(d.Comments) == 0 {
		b.WriteString("_No comments._\n")
		return b.String()
	}

	// Group by path, preserving first-seen order.
	var order []string
	groups := map[string][]DraftComment{}
	for _, c := range d.Comments {
		p := c.Location.Path
		if _, ok := groups[p]; !ok {
			order = append(order, p)
		}
		groups[p] = append(groups[p], c)
	}

	for _, p := range order {
		fmt.Fprintf(&b, "## %s\n\n", p)
		cs := groups[p]
		sort.SliceStable(cs, func(i, j int) bool {
			return cs[i].Location.StartLine < cs[j].Location.StartLine
		})
		for _, c := range cs {
			fmt.Fprintf(&b, "### %s (%s)", lineRef(c.Location), c.Location.Side)
			if c.State != DraftActive {
				fmt.Fprintf(&b, " — %s", c.State)
			}
			b.WriteString("\n\n")
			if strings.TrimSpace(c.Snippet) != "" {
				lang := langOf(p)
				fmt.Fprintf(&b, "```%s\n%s\n```\n\n", lang, strings.TrimRight(c.Snippet, "\n"))
			}
			b.WriteString(blockquote(c.Body))
			// Replies continue the same quote block so the conversation reads
			// as one unit, each attributed on its own marker line.
			for _, r := range c.Replies {
				who := r.Author
				if who == "" {
					who = "reply"
				}
				b.WriteString("\n>\n")
				b.WriteString(blockquote(fmt.Sprintf("↳ @%s: %s", who, strings.TrimRight(r.Body, "\n"))))
			}
			b.WriteString("\n\n")
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// lineRef renders a location's line span for a comment heading, collapsing a
// single-line range to "L12" instead of the noisier "L12-12".
func lineRef(l diff.Location) string {
	if l.Single() {
		return fmt.Sprintf("L%d", l.StartLine)
	}
	return fmt.Sprintf("L%d-%d", l.StartLine, l.EndLine)
}

// blockquote prefixes every line of body with "> " so the note reads as a
// quote distinct from the surrounding structure. Empty lines become a bare ">"
// — a truly blank line would end the quote block in Markdown, splitting a
// multi-paragraph note in two.
func blockquote(body string) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for i, ln := range lines {
		if ln == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + ln
		}
	}
	return strings.Join(lines, "\n")
}

// langOf guesses a fenced-code-block language from a file extension. It is a
// best-effort hint only; unknown extensions produce an empty (plain) fence.
func langOf(path string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "go":
		return "go"
	case "js", "mjs", "cjs":
		return "javascript"
	case "ts", "tsx":
		return "typescript"
	case "py":
		return "python"
	case "rb":
		return "ruby"
	case "rs":
		return "rust"
	case "java":
		return "java"
	case "c", "h":
		return "c"
	case "cc", "cpp", "cxx", "hpp":
		return "cpp"
	case "sh", "bash":
		return "bash"
	case "md":
		return "markdown"
	case "json":
		return "json"
	case "yaml", "yml":
		return "yaml"
	default:
		return ""
	}
}
