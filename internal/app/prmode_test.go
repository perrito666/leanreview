package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/ui"
)

func loadAppFixture(t *testing.T) []diff.FileDiff {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "diff", "testdata", "simple.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	files, err := diff.ParsePatchBytes(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return files
}

func prTestModel(t *testing.T, threads []forge.Thread) *Model {
	t.Helper()
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	files := loadAppFixture(t)
	m := New(Config{
		Files: files,
		Title: "o/r#7: test",
		Theme: ui.DefaultTheme(),
		PR: &PRContext{
			Ref:     forge.PullRequestRef{Owner: "o", Repo: "r", Number: 7},
			PR:      &forge.PullRequest{Title: "test"},
			Threads: threads,
		},
	})
	m.width, m.height = 100, 24
	return m
}

func TestThreadMarkerRenders(t *testing.T) {
	// Anchor a thread on the new-side line 72 of the fixture.
	threads := []forge.Thread{{
		Root:     forge.Comment{Author: "reviewer", Body: "please handle the error"},
		Location: &diff.Location{Path: "internal/api/handler.go", Side: diff.SideRight, StartLine: 72, EndLine: 72},
	}}
	m := prTestModel(t, threads)

	// The index should place the thread on the matching row.
	found := false
	for i := range m.rows() {
		if len(m.threadsAt(i)) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("thread not indexed to any row")
	}

	out := m.View()
	if !strings.Contains(out, "◆") {
		t.Errorf("thread gutter marker not rendered:\n%s", out)
	}
	// The thread root is previewed inline under the line.
	if !strings.Contains(out, "@reviewer") || !strings.Contains(out, "please handle the error") {
		t.Errorf("inline thread preview missing:\n%s", out)
	}
}

func TestThreadsListedInCommentsOverlay(t *testing.T) {
	threads := []forge.Thread{{
		Root:     forge.Comment{Author: "alice", Body: "nit: rename this"},
		Replies:  []forge.Comment{{Author: "bob", Body: "agreed"}},
		Location: &diff.Location{Path: "internal/api/handler.go", Side: diff.SideRight, StartLine: 72, EndLine: 72},
		Outdated: true,
	}}
	m := prTestModel(t, threads)
	m.openComments()
	out := m.View()
	if !strings.Contains(out, "Existing threads") || !strings.Contains(out, "@alice") {
		t.Errorf("threads not shown in overlay:\n%s", out)
	}
	if !strings.Contains(out, "@bob") {
		t.Errorf("reply not shown:\n%s", out)
	}
	if !strings.Contains(out, "outdated") {
		t.Errorf("outdated flag not shown:\n%s", out)
	}
}
