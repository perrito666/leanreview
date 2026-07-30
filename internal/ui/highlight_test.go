package ui

import (
	"strings"
	"testing"
)

func TestAutoStyleResolves(t *testing.T) {
	h := NewHighlighter(true, "auto")
	if !h.Enabled() {
		t.Fatal("auto style should yield an enabled highlighter")
	}
	if out := h.Line("x.go", "func main() {}"); !strings.Contains(out, "\x1b[") {
		t.Errorf("auto-styled output should carry ANSI codes, got %q", out)
	}
}

func TestUnknownStyleFallsBack(t *testing.T) {
	h := NewHighlighter(true, "no-such-style")
	if !h.Enabled() {
		t.Fatal("unknown style should fall back, not disable")
	}
	if out := h.Line("x.go", "func main() {}"); !strings.Contains(out, "\x1b[") {
		t.Errorf("fallback style should still highlight, got %q", out)
	}
}

func TestDisabledPassthrough(t *testing.T) {
	h := NewHighlighter(false, "auto")
	if h.Enabled() {
		t.Fatal("disabled highlighter reports enabled")
	}
	if got := h.Line("x.go", "plain"); got != "plain" {
		t.Errorf("disabled Line = %q, want passthrough", got)
	}
}
