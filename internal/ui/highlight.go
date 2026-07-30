package ui

import (
	"os"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
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
// LEANREVIEW_SYNTAX with the default style. Used where no config is available
// (e.g. tests).
func NewHighlighterFromEnv() *Highlighter {
	enabled := os.Getenv("NO_COLOR") == "" && os.Getenv("LEANREVIEW_SYNTAX") != "0"
	return NewHighlighter(enabled, "github")
}

// NewHighlighter builds a highlighter with an explicit enabled flag and Chroma
// style name. An unknown or empty style falls back to a sensible default.
func NewHighlighter(enabled bool, style string) *Highlighter {
	h := &Highlighter{}
	if !enabled {
		return h
	}
	f := formatters.Get("terminal256")
	if f == nil {
		return h
	}
	s := styles.Get(style)
	if s == nil {
		s = styles.Get("github")
	}
	if s == nil {
		s = styles.Fallback
	}
	h.enabled = true
	h.formatter = f
	h.style = s
	return h
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
