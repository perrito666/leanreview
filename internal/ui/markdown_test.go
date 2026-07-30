package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// mono keeps assertions free of ANSI noise where possible.
func mdLines(t *testing.T, src string, width int) []string {
	t.Helper()
	return RenderMarkdown(src, width, monoTheme())
}

func TestRenderMarkdownHeadingAndParagraph(t *testing.T) {
	out := mdLines(t, "# Title\n\nSome body text.", 80)
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "# Title") {
		t.Errorf("heading missing:\n%s", joined)
	}
	if !strings.Contains(joined, "Some body text.") {
		t.Errorf("paragraph missing:\n%s", joined)
	}
}

func TestRenderMarkdownWrapsProse(t *testing.T) {
	out := mdLines(t, strings.Repeat("word ", 30), 20)
	if len(out) < 2 {
		t.Fatalf("long paragraph should wrap, got %d line(s)", len(out))
	}
	for i, ln := range out {
		if w := lipgloss.Width(ln); w > 20 {
			t.Errorf("line %d width %d exceeds 20: %q", i, w, ln)
		}
	}
}

func TestRenderMarkdownFencePreservedVerbatim(t *testing.T) {
	src := "```go\nfunc main() { fmt.Println(strings.Repeat(\"x\", 99)) }\n```"
	out := mdLines(t, src, 20)
	joined := strings.Join(out, "\n")
	// Code is not word-wrapped: the long line survives intact (indented).
	if !strings.Contains(joined, `func main() { fmt.Println(strings.Repeat("x", 99)) }`) {
		t.Errorf("fenced code was mangled:\n%s", joined)
	}
}

func TestRenderMarkdownBulletsAndRule(t *testing.T) {
	out := mdLines(t, "- one\n* two\n\n---", 40)
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "• one") || !strings.Contains(joined, "• two") {
		t.Errorf("bullets not normalized:\n%s", joined)
	}
	if !strings.Contains(joined, "────") {
		t.Errorf("thematic break missing:\n%s", joined)
	}
}

func TestRenderMarkdownInlineSpans(t *testing.T) {
	out := mdLines(t, "use `frob()` and [docs](https://x.dev) with **care**", 80)
	joined := strings.Join(out, "\n")
	// Markers are stripped; the content and link target survive.
	for _, want := range []string{"frob()", "docs", "(https://x.dev)", "care"} {
		if !strings.Contains(joined, want) {
			t.Errorf("inline span %q missing:\n%s", want, joined)
		}
	}
	for _, gone := range []string{"`", "**", "[docs]"} {
		if strings.Contains(joined, gone) {
			t.Errorf("marker %q should be stripped:\n%s", gone, joined)
		}
	}
}

func TestRenderMarkdownBlockquote(t *testing.T) {
	out := mdLines(t, "> quoted wisdom", 40)
	if !strings.Contains(strings.Join(out, "\n"), "▏ quoted wisdom") {
		t.Errorf("blockquote missing prefix:\n%s", strings.Join(out, "\n"))
	}
}
