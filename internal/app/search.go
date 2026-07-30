package app

import (
	"strings"
	"unicode/utf8"

	"github.com/perrito666/leanreview/internal/diff"
)

// handleSearchKey handles input while typing a "/" search.
func (m *Model) handleSearchKey(key string) {
	switch key {
	case "esc":
		m.searchActive = false
		m.searchInput = ""
	case "enter":
		q := strings.TrimPrefix(m.searchInput, "/")
		m.searchActive = false
		m.searchInput = ""
		m.runSearch(q)
	case "backspace":
		if r := []rune(m.searchInput); len(r) > 1 {
			m.searchInput = string(r[:len(r)-1])
		} else {
			m.searchActive = false
			m.searchInput = ""
		}
	default:
		// One rune = typed text (multi-rune strings are named keys like "up").
		// Counting runes rather than bytes keeps non-ASCII queries typable.
		if utf8.RuneCountInString(key) == 1 {
			m.searchInput += key
		}
	}
}

// runSearch commits a query and jumps to the first match at or after the cursor.
func (m *Model) runSearch(query string) {
	m.search = query
	if query == "" {
		return
	}
	total := m.matchCount()
	if total == 0 {
		m.setStatus("no matches for %q", query)
		return
	}
	// Jump to the first match at/after the current cursor (wrapping).
	if !m.rowMatches(m.rowAt(m.cursor)) {
		m.nextMatch(1)
	}
	m.setStatus("%d match(es) for %q  (n/N to navigate)", total, query)
}

// nextMatch moves the cursor to the next/previous matching row, wrapping.
func (m *Model) nextMatch(dir int) {
	if m.search == "" {
		return
	}
	rows := m.rows()
	n := len(rows)
	if n == 0 {
		return
	}
	for step := 1; step <= n; step++ {
		i := ((m.cursor+dir*step)%n + n) % n
		if m.rowMatches(&rows[i]) {
			m.cursor = i
			m.clampCursor()
			return
		}
	}
}

// matchCount counts the matching rows for the status line. It scans the
// current file's visible rows only, so matches in other files or inside folded
// hunks are not included in the reported total.
func (m *Model) matchCount() int {
	c := 0
	for _, r := range m.rows() {
		if m.rowMatches(&r) {
			c++
		}
	}
	return c
}

// rowMatches reports whether either side's text contains the query (case-insensitive).
func (m *Model) rowMatches(r *diff.DisplayRow) bool {
	if m.search == "" || r == nil {
		return false
	}
	q := strings.ToLower(m.search)
	if r.Left != nil && strings.Contains(strings.ToLower(r.Left.Text), q) {
		return true
	}
	if r.Right != nil && strings.Contains(strings.ToLower(r.Right.Text), q) {
		return true
	}
	return false
}

// clearSearch drops the active query and its highlighting.
func (m *Model) clearSearch() { m.search = "" }
