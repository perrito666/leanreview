package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func typeSearch(m *Model, text string) *Model {
	m = key(m, "/")
	for _, r := range text {
		m = key(m, string(r))
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return next.(*Model)
}

func TestSearchJumpsToMatch(t *testing.T) {
	m := testModel(t)
	m.cursor = 0
	m = typeSearch(m, "calculate")
	r := m.rowAt(m.cursor)
	if r == nil || !m.rowMatches(r) {
		t.Fatalf("cursor did not land on a match; row=%v", r)
	}
	if !strings.Contains(m.status, "match") {
		t.Errorf("status = %q", m.status)
	}
}

func TestSearchNextPrevWrap(t *testing.T) {
	m := testModel(t)
	m = typeSearch(m, "result") // appears on multiple lines
	first := m.cursor
	m = key(m, "n")
	if m.cursor == first {
		t.Errorf("n did not advance to the next match")
	}
	m = key(m, "N")
	if m.cursor != first {
		t.Errorf("N did not return to the previous match (got %d, want %d)", m.cursor, first)
	}
}

func TestSearchNoMatch(t *testing.T) {
	m := testModel(t)
	start := m.cursor
	m = typeSearch(m, "zzz-not-present")
	if m.cursor != start {
		t.Errorf("cursor moved on a no-match search")
	}
	if !strings.Contains(m.status, "no matches") {
		t.Errorf("status = %q", m.status)
	}
}

func TestEscapeClearsSearch(t *testing.T) {
	m := testModel(t)
	m = typeSearch(m, "result")
	if m.search == "" {
		t.Fatalf("search should be set")
	}
	m = key(m, "esc")
	if m.search != "" {
		t.Errorf("esc did not clear the search")
	}
}

// TestSearchAcceptsNonASCII guards the rune-aware input path: multi-byte
// characters must be typable and backspace must remove whole runes.
func TestSearchAcceptsNonASCII(t *testing.T) {
	m := testModel(t)
	m.searchActive = true
	m.searchInput = "/"
	m.handleSearchKey("é")
	m.handleSearchKey("ü")
	if m.searchInput != "/éü" {
		t.Fatalf("searchInput = %q, want /éü", m.searchInput)
	}
	m.handleSearchKey("backspace")
	if m.searchInput != "/é" {
		t.Errorf("backspace should remove one rune, got %q", m.searchInput)
	}
	m.handleSearchKey("up") // named key: must not be appended
	if m.searchInput != "/é" {
		t.Errorf("named keys must not enter the query, got %q", m.searchInput)
	}
}

// TestCmdlineAcceptsNonASCII mirrors the search guard for ":" input.
func TestCmdlineAcceptsNonASCII(t *testing.T) {
	m := testModel(t)
	m.cmdlineActive = true
	m.cmdline = ":"
	m.handleCmdlineKey("ñ")
	if m.cmdline != ":ñ" {
		t.Fatalf("cmdline = %q, want :ñ", m.cmdline)
	}
	m.handleCmdlineKey("backspace")
	if m.cmdline != ":" {
		t.Errorf("backspace should remove the rune, got %q", m.cmdline)
	}
}
