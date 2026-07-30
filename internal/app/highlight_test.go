package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/perrito666/leanreview/internal/ui"
)

func TestSyntaxHighlightingApplied(t *testing.T) {
	// Force highlighting on regardless of the ambient environment.
	t.Setenv("NO_COLOR", "")
	t.Setenv("LEANREVIEW_SYNTAX", "")

	m := New(Config{
		Files: loadAppFixture(t), // internal/api/handler.go — a .go file
		Title: "t",
		Theme: ui.DefaultTheme(),
	})
	m.width, m.height = 100, 20

	if !m.highlighter.Enabled() {
		t.Skip("highlighter unavailable in this environment")
	}

	out := m.View()
	// Highlighted output carries ANSI SGR sequences...
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected ANSI color sequences in highlighted output")
	}
	// ...and stripping them recovers the source text.
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "calculate(input)") {
		t.Errorf("source text not recoverable after stripping ANSI:\n%s", plain)
	}
}

func TestHighlightDisabledIsPlain(t *testing.T) {
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	m := New(Config{Files: loadAppFixture(t), Title: "t", Theme: ui.DefaultTheme()})
	m.width, m.height = 100, 20
	if m.highlighter.Enabled() {
		t.Fatalf("LEANREVIEW_SYNTAX=0 should disable highlighting")
	}
	if got := m.highlight("x.go", "func main() {}"); got != "func main() {}" {
		t.Errorf("disabled highlighter should pass text through, got %q", got)
	}
}
