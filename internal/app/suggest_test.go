package app

import (
	"os"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/ui"
)

// TestSuggestionPrefillsSelectedCode: R opens the editor with the selected
// lines inside a ```suggestion fence — verbatim, so a raw tab in the source
// survives (the tab-expanded display form would corrupt the host's patch).
func TestSuggestionPrefillsSelectedCode(t *testing.T) {
	patch := "diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n@@ -1,2 +1,4 @@\n func f() {\n+\tfirst()\n+\tsecond()\n }\n"
	files, err := diff.ParsePatchBytes([]byte(patch))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := newTestModelWithFiles(t, files)

	// Select the two added lines.
	rows := m.rows()
	var lo, hi int
	for i := range rows {
		if rows[i].Source != nil && rows[i].Right != nil && rows[i].Right.Kind == diff.LineAddition {
			if lo == 0 {
				lo = i
			}
			hi = i
		}
	}
	m.cursor = lo
	m = key(m, "v")
	m.cursor = hi

	cmd := m.startSuggestion()
	if cmd == nil {
		t.Fatalf("no editor command: status=%q err=%v", m.status, m.err)
	}
	buf, err := os.ReadFile(m.inflight.session.Path)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	want := "```suggestion\n\tfirst()\n\tsecond()\n```"
	if !strings.Contains(string(buf), want) {
		t.Errorf("editor buffer missing verbatim fence:\n%s", buf)
	}
	m.inflight.session.Close()
}

// TestSuggestionRefusesLeftSide: suggestions replace new-side lines; a
// deletion selection cannot host one.
func TestSuggestionRefusesLeftSide(t *testing.T) {
	m := testModel(t)
	for i, r := range m.rows() {
		if r.Source != nil && r.Source.Side == diff.SideLeft {
			m.cursor = i
			break
		}
	}
	if cmd := m.startSuggestion(); cmd != nil {
		t.Fatalf("left-side suggestion should be refused")
	}
	if !strings.Contains(m.status, "RIGHT") {
		t.Errorf("status = %q", m.status)
	}
}

// TestSuggestionRendersStyled: a comment carrying a suggestion fence shows a
// label plus addition-styled, unwrapped code rows — not the raw fence text.
func TestSuggestionRendersStyled(t *testing.T) {
	m := testModel(t)
	m.wrapText = true // multi-line bodies collapse to one line with wrap off
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	m.draft.Add(loc, "tighten this:\n```suggestion\nresult, err := calculate(input)\nreturn result, err\n```", snip)

	out := m.View()
	if strings.Contains(out, "```") {
		t.Errorf("raw fence leaked into the box:\n%s", out)
	}
	if !strings.Contains(out, "suggested change:") {
		t.Errorf("suggestion label missing")
	}
	if !strings.Contains(out, "+ result, err := calculate(input)") || !strings.Contains(out, "+ return result, err") {
		t.Errorf("suggested code rows missing:\n%s", out)
	}
	// The code rows are preformatted (never wrapped/restyled).
	pre := 0
	for _, r := range m.rows() {
		if r.Annotation && r.Pre {
			pre++
		}
	}
	if pre < 3 { // label + 2 code lines
		t.Errorf("pre rows = %d, want label+code", pre)
	}
}

// newTestModelWithFiles mirrors testModel with explicit files.
func newTestModelWithFiles(t *testing.T, files []diff.FileDiff) *Model {
	t.Helper()
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	m := New(Config{Files: files, Title: "test", Theme: ui.DefaultTheme()})
	m.width, m.height = 100, 30
	return m
}
