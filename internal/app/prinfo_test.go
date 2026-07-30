package app

import (
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/ui"
)

func prInfoModel(t *testing.T, pr *forge.PullRequest) *Model {
	t.Helper()
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	m := New(Config{
		Files: loadAppFixture(t),
		Title: "o/r#7: " + pr.Title,
		Theme: ui.DefaultTheme(),
		PR: &PRContext{
			Ref: forge.PullRequestRef{Owner: "o", Repo: "r", Number: 7},
			PR:  pr,
		},
	})
	m.width, m.height = 100, 24
	return m
}

func TestTitleBarShowsForgeAndFileOnSeparateLines(t *testing.T) {
	m := prInfoModel(t, &forge.PullRequest{Title: "Fix the frobnicator"})
	lines := strings.Split(m.View(), "\n")
	if len(lines) < 3 {
		t.Fatalf("view too short: %d lines", len(lines))
	}
	if strings.Contains(lines[0], "leanreview") {
		t.Errorf("title bar still carries the leanreview prefix: %q", lines[0])
	}
	if !strings.Contains(lines[0], "gh") || !strings.Contains(lines[0], "Fix the frobnicator") {
		t.Errorf("first line should show forge badge and PR title: %q", lines[0])
	}
	if !strings.Contains(lines[1], "handler.go") || !strings.Contains(lines[1], "[1/") {
		t.Errorf("second line should show the file path and position: %q", lines[1])
	}
	if strings.Contains(lines[0], "handler.go") {
		t.Errorf("file path leaked onto the title line: %q", lines[0])
	}
}

func TestTitleBarLocalModeHasNoForgeBadge(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 100, 24
	first := strings.Split(m.View(), "\n")[0]
	if strings.Contains(first, "leanreview") || strings.Contains(first, "gh ") {
		t.Errorf("local mode should show neither prefix nor forge badge: %q", first)
	}
}

func TestPRInfoOverlayTogglesAndRenders(t *testing.T) {
	m := prInfoModel(t, &forge.PullRequest{
		Title:   "Fix the frobnicator",
		Body:    "# Why\n\nThe `frob` call was **wrong**.\n\n- fixes [#5](https://example.com/5)",
		Author:  "alice",
		URL:     "https://github.com/o/r/pull/7",
		BaseRef: "main",
		HeadRef: "fix",
	})

	m = key(m, "p")
	if m.mode != ModePR {
		t.Fatalf("p did not open the PR overlay, mode=%v", m.mode)
	}
	out := m.View()
	for _, want := range []string{"Fix the frobnicator", "@alice", "main ← fix", "https://github.com/o/r/pull/7", "Why", "frob", "https://example.com/5"} {
		if !strings.Contains(out, want) {
			t.Errorf("PR overlay missing %q:\n%s", want, out)
		}
	}

	m = key(m, "p")
	if m.mode != ModeNormal {
		t.Errorf("p did not close the PR overlay, mode=%v", m.mode)
	}
}

func TestPRInfoOverlayScrollClamps(t *testing.T) {
	body := strings.Repeat("paragraph line\n\n", 40)
	m := prInfoModel(t, &forge.PullRequest{Title: "long", Body: body})
	m = key(m, "p")
	first := m.View()

	m = key(m, "j")
	if m.prScroll != 1 {
		t.Errorf("j should scroll, prScroll=%d", m.prScroll)
	}
	if m.View() == first {
		t.Errorf("scrolling did not change the overlay")
	}
	m = key(m, "G")
	m.View() // clamps
	if m.prScroll <= 1 || m.prScroll > 200 {
		t.Errorf("G should clamp to the bottom, prScroll=%d", m.prScroll)
	}
	m = key(m, "g")
	if m.prScroll != 0 {
		t.Errorf("g should return to the top, prScroll=%d", m.prScroll)
	}
	m = key(m, "k") // at top: stays clamped at 0
	if m.prScroll != 0 {
		t.Errorf("k at top should clamp to 0, prScroll=%d", m.prScroll)
	}
}

func TestPRInfoUnavailableInLocalMode(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 100, 24
	m = key(m, "p")
	if m.mode != ModeNormal {
		t.Errorf("p in local mode should not open an overlay, mode=%v", m.mode)
	}
	if !strings.Contains(m.status, "no pull request") {
		t.Errorf("expected a status hint, got %q", m.status)
	}
}

func TestPRInfoFallsBackToRefURL(t *testing.T) {
	m := prInfoModel(t, &forge.PullRequest{Title: "no url"})
	m.openPRInfo()
	if out := m.View(); !strings.Contains(out, "https://github.com/o/r/pull/7") {
		t.Errorf("overlay should derive a URL from the ref:\n%s", out)
	}
}
