package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/ui"
)

// pickerModel is a minimal list UI for choosing a discovered pull/merge
// request. It runs as its own Bubble Tea program before the review opens.
type pickerModel struct {
	entries []forge.ListedRequest
	theme   ui.Theme

	cursor int
	top    int
	width  int
	height int

	// choice is the selected index, or -1 when the picker was dismissed.
	choice int
}

// PickRequest shows an interactive picker over the discovered requests and
// returns the chosen index, or -1 if the user dismissed it.
func PickRequest(entries []forge.ListedRequest, theme ui.Theme) (int, error) {
	m := &pickerModel{entries: entries, theme: theme, choice: -1}
	prog := tea.NewProgram(m, tea.WithAltScreen())
	res, err := prog.Run()
	if err != nil {
		return -1, err
	}
	return res.(*pickerModel).choice, nil
}

// Init implements tea.Model; the picker needs no startup command and simply
// waits for the initial WindowSizeMsg before rendering anything real.
func (m *pickerModel) Init() tea.Cmd { return nil }

// Update handles resize and vim-style list navigation: enter records the
// selection and quits, q/esc quit leaving choice at -1 (dismissed).
func (m *pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "G":
			m.cursor = len(m.entries) - 1
		case "enter":
			m.choice = m.cursor
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.choice = -1
			return m, tea.Quit
		}
		m.scroll()
	}
	return m, nil
}

// scroll keeps the cursor inside the visible window after every key, the same
// clamp the main model's clampCursor applies to the diff view.
func (m *pickerModel) scroll() {
	h := m.listHeight()
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+h {
		m.top = m.cursor - h + 1
	}
}

// listHeight is the number of rows available for entries once the title and
// status lines are subtracted, floored at 1 so the render loop always
// makes progress.
func (m *pickerModel) listHeight() int {
	h := m.height - 2
	if h < 1 {
		return 1
	}
	return h
}

// View renders the title bar, the windowed entry list padded with blank rows
// to a stable height, and a key-hint footer. Until the first WindowSizeMsg
// arrives (width 0) it shows a placeholder instead of guessing a layout.
func (m *pickerModel) View() string {
	if m.width == 0 {
		return "loading…"
	}
	var b strings.Builder
	title := fmt.Sprintf("Select a request to review — %d found  (enter to open, q to quit)", len(m.entries))
	b.WriteString(m.theme.Title.Width(m.width).Render(clip(title, m.width)))
	b.WriteByte('\n')

	h := m.listHeight()
	for i := m.top; i < m.top+h; i++ {
		if i >= len(m.entries) {
			b.WriteString(pad("", m.width))
		} else {
			e := m.entries[i]
			line := fmt.Sprintf("%-32s %s", e.Ref.String(), e.Title)
			meta := fmt.Sprintf("  @%s  %s", e.Author, e.UpdatedAt.Format("2006-01-02"))
			row := pad(clip(line+meta, m.width), m.width)
			if i == m.cursor {
				row = m.theme.Cursor.Render(row)
			}
			b.WriteString(row)
		}
		b.WriteByte('\n')
	}
	b.WriteString(m.theme.Status.Width(m.width).Render(clip("j/k move   enter review   q quit", m.width)))
	return b.String()
}
