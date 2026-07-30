package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/ui"
)

func pickerFixture() *pickerModel {
	entries := []forge.ListedRequest{
		{Ref: forge.PullRequestRef{Host: "github.com", Owner: "o", Repo: "r", Number: 1}, Title: "First", Author: "a", UpdatedAt: time.Unix(0, 0)},
		{Ref: forge.PullRequestRef{Host: "github.com", Owner: "o", Repo: "r", Number: 2}, Title: "Second", Author: "b", UpdatedAt: time.Unix(0, 0)},
		{Ref: forge.PullRequestRef{Host: "gitlab.com", Owner: "g", Repo: "p", Number: 3}, Title: "Third", Author: "c", UpdatedAt: time.Unix(0, 0)},
	}
	m := &pickerModel{entries: entries, theme: ui.DefaultTheme(), choice: -1}
	m.width, m.height = 90, 10
	return m
}

func pkey(m *pickerModel, k string) (*pickerModel, tea.Cmd) {
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	next, cmd := m.Update(msg)
	return next.(*pickerModel), cmd
}

func TestPickerSelect(t *testing.T) {
	m := pickerFixture()
	m, _ = pkey(m, "j")
	m, _ = pkey(m, "down")
	m, cmd := pkey(m, "enter")
	if m.choice != 2 {
		t.Errorf("choice = %d, want 2", m.choice)
	}
	if cmd == nil {
		t.Errorf("enter should quit the picker")
	}
}

func TestPickerDismiss(t *testing.T) {
	m := pickerFixture()
	m, _ = pkey(m, "j")
	m, cmd := pkey(m, "q")
	if m.choice != -1 {
		t.Errorf("q should leave choice at -1, got %d", m.choice)
	}
	if cmd == nil {
		t.Errorf("q should quit the picker")
	}
}

func TestPickerViewShowsEntries(t *testing.T) {
	m := pickerFixture()
	out := m.View()
	for _, want := range []string{"github.com/o/r#1", "gitlab.com/g/p!3", "First", "@b", "3 found"} {
		if !strings.Contains(out, want) {
			t.Errorf("picker view missing %q:\n%s", want, out)
		}
	}
}
