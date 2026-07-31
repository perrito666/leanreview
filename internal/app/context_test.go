package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
)

// contextModel builds a model whose fetcher serves a synthesized full file
// matching the fixture's hunks, counting fetches so laziness is observable.
func contextModel(t *testing.T) (*Model, *int) {
	t.Helper()
	m := testModel(t)
	m.width, m.height = 100, 30

	// Reconstruct the new-side file the same way the diff fixture implies it.
	f := m.currentFile()
	max := 0
	byLine := map[int]string{}
	for hi := range f.Hunks {
		for _, l := range f.Hunks[hi].Lines {
			if l.NewLine != nil {
				byLine[*l.NewLine] = l.Text
				if *l.NewLine > max {
					max = *l.NewLine
				}
			}
		}
	}
	var b strings.Builder
	for i := 1; i <= max+5; i++ {
		if text, ok := byLine[i]; ok {
			b.WriteString(text)
		} else {
			fmt.Fprintf(&b, "filler %d", i)
		}
		b.WriteByte('\n')
	}
	content := b.String()

	fetches := 0
	m.fetchContext = func(context.Context, string, diff.Side) ([]byte, error) {
		fetches++
		return []byte(content), nil
	}
	return m, &fetches
}

// toggleAndDeliver presses T and, when a fetch command results, runs it and
// delivers the message — the synchronous equivalent of the async flow.
func toggleAndDeliver(t *testing.T, m *Model) {
	t.Helper()
	cmd := m.toggleContext()
	if cmd != nil {
		msg, ok := cmd().(contextContentMsg)
		if !ok {
			t.Fatalf("toggle produced an unexpected message")
		}
		m.onContextContent(msg)
	}
}

func TestContextToggleLazyFetchAndCenter(t *testing.T) {
	m, fetches := contextModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)

	if *fetches != 0 {
		t.Fatalf("content fetched before anyone asked for context")
	}
	toggleAndDeliver(t, m)
	if *fetches != 1 {
		t.Fatalf("fetches = %d, want exactly 1 on first toggle", *fetches)
	}
	if !m.contextActive() {
		t.Fatalf("context not active after toggle")
	}
	// The cursor still points at the same semantic line…
	r := m.rowAt(m.cursor)
	if r == nil || r.Source == nil || r.Source.StartLine != 72 || r.Source.Side != diff.SideRight {
		t.Errorf("cursor lost its line across the toggle: %+v", r.Source)
	}
	// …and the viewport is centered on it.
	wantTop := m.cursor - m.contentHeight()/2
	if wantTop < 0 {
		wantTop = 0
	}
	if m.top != wantTop {
		t.Errorf("top = %d, want centered %d", m.top, wantTop)
	}
	// Gap rows exist and are not commentable.
	found := false
	for _, row := range m.rows() {
		if row.Source == nil && !row.Annotation && row.Left != nil && strings.HasPrefix(row.Left.Text, "filler") {
			found = true
		}
	}
	if !found {
		t.Errorf("no gap rows in the context view")
	}

	// Toggling back reuses the projection (no refetch) and recenters.
	toggleAndDeliver(t, m)
	if m.contextActive() {
		t.Fatalf("context still active after toggling off")
	}
	toggleAndDeliver(t, m)
	if *fetches != 1 {
		t.Errorf("fetches = %d after re-toggle; projection must be reused", *fetches)
	}
}

func TestContextHunkJumping(t *testing.T) {
	m, _ := contextModel(t)
	m.cursor = m.firstContentRow()
	toggleAndDeliver(t, m)

	start := m.hunkAt(m.cursor)
	m = key(m, "]")
	m = key(m, "c")
	if got := m.hunkAt(m.cursor); got != start+1 {
		t.Errorf("]c moved to hunk %d, want %d", got, start+1)
	}
	// And the view recentered on the new cursor.
	wantTop := m.cursor - m.contentHeight()/2
	if wantTop < 0 {
		wantTop = 0
	}
	if m.top != wantTop {
		t.Errorf("hunk jump did not recenter: top=%d want %d", m.top, wantTop)
	}
	m = key(m, "[")
	m = key(m, "c")
	if got := m.hunkAt(m.cursor); got != start {
		t.Errorf("[c returned to hunk %d, want %d", got, start)
	}
}

func TestContextUnavailableSources(t *testing.T) {
	m := testModel(t)
	m.fetchContext = nil
	if cmd := m.toggleContext(); cmd != nil {
		t.Errorf("toggle without a fetcher should not produce a command")
	}
	if !strings.Contains(m.status, "not available") {
		t.Errorf("status = %q", m.status)
	}
}

func TestContextRejectsMismatchedContent(t *testing.T) {
	m, _ := contextModel(t)
	m.fetchContext = func(context.Context, string, diff.Side) ([]byte, error) {
		return []byte("totally different file\n"), nil
	}
	toggleAndDeliver(t, m)
	if m.contextActive() {
		t.Errorf("mismatched content must not enable the context view")
	}
	if m.err == nil {
		t.Errorf("mismatch should surface an error")
	}
}
