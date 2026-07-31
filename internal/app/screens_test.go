package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/ui"
)

// TestGenerateScreens renders the documentation screenshots as ANSI dumps.
// It is a test (not a cmd) so it can drive the unexported model directly, and
// it only runs when LEANREVIEW_SCREENS_DIR is set — `make screens` sets it and
// converts the dumps to SVG with freeze. Keeping the renders here means the
// screenshots regenerate from the real UI, so docs cannot drift silently.
func TestGenerateScreens(t *testing.T) {
	dir := os.Getenv("LEANREVIEW_SCREENS_DIR")
	if dir == "" {
		t.Skip("set LEANREVIEW_SCREENS_DIR to render screenshot sources")
	}
	// Headless runs collapse to no color; force a truecolor dark profile so
	// the dumps carry the styling a real terminal would show.
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	t.Setenv("LEANREVIEW_SYNTAX", "0")

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	pr := &forge.PullRequest{
		Title:   "Add named discovery filters",
		Body:    "# Summary\n\nAdds `engine:name` filters to `--list` so a review queue can be\nnarrowed per forge.\n\n## Details\n\n- supports **gh** and **glab** engines\n- see [the docs](https://perrito666.github.io/leanreview/) for syntax\n\n> Falls back to all engines when unfiltered.",
		Author:  "perrito666",
		URL:     "https://github.com/perrito666/leanreview/pull/12",
		BaseRef: "main",
		HeadRef: "filters",
	}
	threads := []forge.Thread{{
		Root:     forge.Comment{ID: 1, Author: "reviewer", Body: "should this propagate the error instead?"},
		Location: &diff.Location{Path: "internal/api/handler.go", Side: diff.SideRight, StartLine: 74, EndLine: 74},
	}}
	newModel := func() *Model {
		m := New(Config{
			Files: loadAppFixture(t),
			Title: "perrito666/leanreview#12: Add named discovery filters",
			Theme: ui.DefaultTheme(),
			Wrap:  true,
			PR: &PRContext{
				Ref:     forge.PullRequestRef{Owner: "perrito666", Repo: "leanreview", Number: 12},
				PR:      pr,
				Threads: threads,
			},
		})
		m.width, m.height = 100, 28
		return m
	}

	// Main view: unified diff, sidebar, a draft comment in its box, a thread.
	m := newModel()
	m.sidebar = true
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	m.draft.Add(loc, "This still ignores the error from calculate().", snip)
	write("main", m.View())

	// Split layout: per-side styling with the comment box over the right panel.
	m.sidebar = false
	m.toggleLayout()
	write("split", m.View())

	// PR details overlay (p).
	m = newModel()
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ = m.buildLocation()
	m.draft.Add(loc, "This still ignores the error from calculate().", snip)
	m.openPRInfo()
	write("pr-overlay", m.View())

	// Comment list overlay (C).
	m.mode = ModeNormal
	m.openComments()
	write("comments", m.View())

	// Submission confirmation (s).
	m.mode = ModeNormal
	m.beginSubmit(forge.EventRequestChanges)
	write("submit", m.View())

	// Help overlay (?).
	m.mode = ModeHelp
	write("help", m.View())

	// Discovery picker (--list).
	p := &pickerModel{choice: -1, theme: ui.DefaultTheme(), entries: []forge.ListedRequest{
		{Ref: forge.PullRequestRef{Owner: "perrito666", Repo: "leanreview", Number: 12}, Title: "Add named discovery filters", Author: "perrito666"},
		{Ref: forge.PullRequestRef{Owner: "perrito666", Repo: "leanreview", Number: 11}, Title: "Fix wrap width off-by-one", Author: "alice"},
		{Ref: forge.PullRequestRef{Owner: "acme", Repo: "tools", Number: 7}, Title: "Retry transient gh failures", Author: "bob"},
	}}
	p.width, p.height = 100, 12
	write("picker", p.View())
}
