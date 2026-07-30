package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSidebarTogglesContentWidth(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 100, 20
	if m.contentWidth() != 100 {
		t.Fatalf("content width without sidebar = %d, want 100", m.contentWidth())
	}
	m.toggleSidebar()
	if !m.effSidebar() {
		t.Fatalf("sidebar should be effective at width 100")
	}
	if m.contentWidth() != 100-sidebarWidth-1 {
		t.Errorf("content width with sidebar = %d, want %d", m.contentWidth(), 100-sidebarWidth-1)
	}
}

func TestSidebarSuppressedWhenNarrow(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 50, 20
	m.toggleSidebar()
	if m.effSidebar() {
		t.Errorf("sidebar should be suppressed on a narrow terminal")
	}
	if m.contentWidth() != 50 {
		t.Errorf("content width = %d, want full 50 when suppressed", m.contentWidth())
	}
}

func TestSidebarRendersAndPreservesWidth(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 100, 20
	m.toggleSidebar()
	out := m.View()
	if !strings.Contains(out, "Changed files") {
		t.Errorf("sidebar header missing:\n%s", out)
	}
	if !strings.Contains(out, "handler.go") {
		t.Errorf("file entry missing from sidebar")
	}
	// Every body line must still be exactly the full terminal width.
	lines := strings.Split(out, "\n")
	for i := 1; i < len(lines)-1; i++ { // skip title (0) and status (last)
		if w := lipgloss.Width(lines[i]); w != 100 {
			t.Fatalf("body line %d width = %d, want 100\nline: %q", i, w, lines[i])
		}
	}
}
