package ui

import (
	"os"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// Highlighter renders single source lines to ANSI-colored strings using Chroma.
// It highlights per line (no cross-line lexer state), which is the right
// trade-off for a diff viewer where lines are shown out of block context. It is
// disabled when NO_COLOR is set or LEANREVIEW_SYNTAX=0, in which case Line is a
// passthrough.
type Highlighter struct {
	enabled   bool
	formatter chroma.Formatter
	style     *chroma.Style
}

// NewHighlighterFromEnv builds a highlighter honoring NO_COLOR and
// LEANREVIEW_SYNTAX with the auto-detected style. Used where no config is
// available (e.g. tests).
func NewHighlighterFromEnv() *Highlighter {
	enabled := os.Getenv("NO_COLOR") == "" && os.Getenv("LEANREVIEW_SYNTAX") != "0"
	return NewHighlighter(enabled, "auto")
}

// NewHighlighter builds a highlighter with an explicit enabled flag and Chroma
// style name. The style "auto" (or "") picks one suited to the terminal
// background — a dark-background style on dark terminals, a light one
// otherwise — because Chroma styles are background-specific: a light style's
// dark foreground tokens are unreadable on a dark terminal. An unknown style
// falls back to the auto choice.
func NewHighlighter(enabled bool, style string) *Highlighter {
	h := &Highlighter{}
	if !enabled {
		return h
	}
	f := formatters.Get("terminal256")
	if f == nil {
		return h
	}
	if style == "" || style == "auto" {
		style = autoStyle()
	}
	s := styles.Get(style)
	if s == nil {
		s = styles.Get(autoStyle())
	}
	if s == nil {
		s = styles.Fallback
	}
	h.enabled = true
	h.formatter = f
	h.style = s
	return h
}

// autoStyle queries the terminal background (before Bubble Tea owns it) and
// returns a matching Chroma style name.
func autoStyle() string {
	if lipgloss.HasDarkBackground() {
		return "monokai"
	}
	return "github"
}

// Enabled reports whether highlighting is active.
func (h *Highlighter) Enabled() bool { return h != nil && h.enabled }

// Line highlights one line of source from the file at path. On any failure it
// returns the input unchanged, so callers can use the result directly.
func (h *Highlighter) Line(path, text string) string {
	if !h.Enabled() || text == "" {
		return text
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	it, err := lexer.Tokenise(nil, text)
	if err != nil {
		return text
	}
	var b strings.Builder
	if err := h.formatter.Format(&b, h.style, it); err != nil {
		return text
	}
	// Chroma appends a trailing newline from the source line; trim it so the
	// caller controls layout.
	return strings.TrimRight(b.String(), "\n")
}
