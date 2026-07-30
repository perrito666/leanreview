package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

func threadModelAtLine(t *testing.T) *Model {
	t.Helper()
	threads := []forge.Thread{{
		Root: forge.Comment{Author: "alice", Body: "why not handle the error?", CreatedAt: time.Unix(1_700_000_000, 0)},
		Replies: []forge.Comment{
			{Author: "bob", Body: "good point"},
			{Author: "alice", Body: "will fix"},
		},
		Location: &diff.Location{Path: "internal/api/handler.go", Side: diff.SideRight, StartLine: 72, EndLine: 72},
	}}
	m := prTestModel(t, threads)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	return m
}

func TestEnterOpensThreadReader(t *testing.T) {
	m := threadModelAtLine(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.mode != ModeThread {
		t.Fatalf("Enter on a threaded line should open the thread reader, mode=%v", m.mode)
	}
	out := m.View()
	for _, want := range []string{"@alice", "@bob", "why not handle the error?", "good point", "will fix"} {
		if !strings.Contains(out, want) {
			t.Errorf("thread reader missing %q:\n%s", want, out)
		}
	}
}

func TestThreadReaderEscCloses(t *testing.T) {
	m := threadModelAtLine(t)
	m.openThreadReader()
	if m.mode != ModeThread {
		t.Fatal("reader did not open")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if next.(*Model).mode != ModeNormal {
		t.Errorf("esc did not close the thread reader")
	}
}

func TestEnterWithoutThreadOpensCommentList(t *testing.T) {
	m := threadModelAtLine(t)
	// Move to a line with no thread.
	m.cursor = mustFindRow(t, m, diff.SideRight, 74)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := next.(*Model).mode; got != ModeComments {
		t.Errorf("Enter off a thread should open the comment list, got %v", got)
	}
}
