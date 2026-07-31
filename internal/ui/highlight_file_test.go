package ui

import (
	"strings"
	"testing"
)

func fileHighlighter(t *testing.T) *Highlighter {
	t.Helper()
	h := NewHighlighter(true, "monokai")
	if !h.Enabled() {
		t.Skip("highlighting unavailable in this environment")
	}
	return h
}

func TestContentLinesLineCountMatchesSource(t *testing.T) {
	h := fileHighlighter(t)
	src := "package main\n\nfunc main() {\n\tprintln(1)\n}\n"
	lines := h.ContentLines("main.go", []byte(src))
	want := strings.Split(src, "\n")
	if len(lines) != len(want) {
		t.Fatalf("lines = %d, want %d (split semantics must match strings.Split)", len(lines), len(want))
	}
}

// TestContentLinesCrossLineState is the reason this API exists: a construct
// spanning lines must stay colored on every line it covers — the per-line
// highlighter gets this wrong by design.
func TestContentLinesCrossLineState(t *testing.T) {
	h := fileHighlighter(t)
	src := "/* first\nsecond */\nvar x = 1\n"
	lines := h.ContentLines("f.go", []byte(src))
	if len(lines) < 3 {
		t.Fatalf("unexpected line count %d", len(lines))
	}
	// Both comment lines carry styling; and crucially the SECOND line (which
	// per-line lexing would mis-lex as code) is styled as one comment token.
	if !strings.Contains(lines[0], "\x1b[") || !strings.Contains(lines[1], "\x1b[") {
		t.Errorf("comment lines not styled: %q / %q", lines[0], lines[1])
	}
	first := lines[0][:strings.Index(lines[0], "m")+1]
	if !strings.HasPrefix(lines[1], first) {
		t.Errorf("second comment line lost the comment style:\nline0 %q\nline1 %q", lines[0], lines[1])
	}
}

// TestContentLinesFgOnly: no full resets and no background codes, so a row
// background tint survives the embedded styling.
func TestContentLinesFgOnly(t *testing.T) {
	h := fileHighlighter(t)
	lines := h.ContentLines("main.go", []byte("package main\nfunc f() int { return 1 }\n"))
	joined := strings.Join(lines, "\n")
	for _, banned := range []string{"\x1b[0m", "\x1b[m", "\x1b[48;", "\x1b[4" + "9m"} {
		if strings.Contains(joined, banned) {
			t.Errorf("output contains %q — background tints would be destroyed", banned)
		}
	}
	if !strings.Contains(joined, sgrOff) {
		t.Errorf("styled runs must terminate with the fg-only clear")
	}
}

func TestRGBTo256(t *testing.T) {
	if got := rgbTo256(0, 0, 0); got != 16 && got != 232 {
		t.Errorf("black = %d", got)
	}
	if got := rgbTo256(255, 255, 255); got != 231 && got != 255 {
		t.Errorf("white = %d", got)
	}
	if got := rgbTo256(128, 128, 128); got < 232 {
		t.Errorf("mid gray should map to the gray ramp, got %d", got)
	}
	if got := rgbTo256(255, 0, 0); got != 196 {
		t.Errorf("pure red = %d, want 196", got)
	}
}
