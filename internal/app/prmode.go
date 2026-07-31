package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

// PRContext carries everything the TUI needs to operate in pull-request mode:
// the forge to reply/submit through, the PR reference and metadata, and the
// existing review threads. It is nil in local/patch mode.
type PRContext struct {
	Forge   forge.Forge
	Ref     forge.PullRequestRef
	PR      *forge.PullRequest
	Threads []forge.Thread
	// General is the PR's conversation (non-inline) discussion, oldest
	// first, shown in the PR details overlay below the description.
	General []forge.Comment
}

// buildThreadIndex maps a location key to the indices of threads anchored there,
// so the diff view can mark lines that already carry review discussion.
func (m *Model) buildThreadIndex() {
	m.threadIndex = map[string][]int{}
	if m.pr == nil {
		return
	}
	for i, th := range m.pr.Threads {
		if th.Location == nil {
			continue
		}
		key := locKey(th.Location.Path, th.Location.Side, th.Location.StartLine)
		m.threadIndex[key] = append(m.threadIndex[key], i)
	}
}

// locKey canonicalises a comment anchor (path, side, start line) into the
// string key shared by everything that writes or reads threadIndex, so lookups
// from diff rows and thread locations always agree.
func locKey(path string, side diff.Side, line int) string {
	return fmt.Sprintf("%s|%d|%d", path, side, line)
}

// threadsAt returns the thread indices anchored to the row at index i.
func (m *Model) threadsAt(i int) []int {
	if m.pr == nil {
		return nil
	}
	r := m.rowAt(i)
	if r == nil || r.Source == nil {
		return nil
	}
	return m.threadIndex[locKey(r.Source.Path, r.Source.Side, r.Source.StartLine)]
}

// prActive reports whether the model is in pull-request mode.
func (m *Model) prActive() bool { return m.pr != nil }

// openThreadReader opens the focused reader for threads at the cursor line.
func (m *Model) openThreadReader() {
	idxs := m.threadsAt(m.cursor)
	if len(idxs) == 0 {
		return
	}
	m.threadView = idxs
	m.mode = ModeThread
}

// handleThreadKey handles keys while the thread reader is open.
func (m *Model) handleThreadKey(key string) tea.Cmd {
	switch key {
	case "esc", "q", "enter":
		m.mode = ModeNormal
	case "r":
		m.mode = ModeNormal
		return m.startReplyUnderCursor()
	}
	return nil
}

// threadReaderView renders the full root + replies of the viewed threads.
func (m *Model) threadReaderView() string {
	var b strings.Builder
	b.WriteString("Thread (r to reply, esc to close)\n\n")
	for _, ti := range m.threadView {
		if ti < 0 || ti >= len(m.pr.Threads) {
			continue
		}
		th := m.pr.Threads[ti]
		loc := "general"
		if th.Location != nil {
			loc = fmt.Sprintf("%s %s", th.Location.Path, lineRefString(*th.Location))
		}
		flags := ""
		if th.Outdated {
			flags += " (outdated)"
		}
		if th.Resolved {
			flags += " (resolved)"
		}
		b.WriteString(fmt.Sprintf("── %s%s\n", loc, flags))
		writeThreadComment(&b, th.Root)
		for _, rep := range th.Replies {
			writeThreadComment(&b, rep)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// writeThreadComment appends one comment to the thread reader: an author line
// (with timestamp when known) and the body indented beneath it. Roots and
// replies go through the same helper so they format identically.
func writeThreadComment(b *strings.Builder, c forge.Comment) {
	when := ""
	if !c.CreatedAt.IsZero() {
		when = "  " + c.CreatedAt.Format("2006-01-02 15:04")
	}
	fmt.Fprintf(b, "  @%s%s\n", c.Author, when)
	for _, line := range strings.Split(strings.TrimRight(c.Body, "\n"), "\n") {
		b.WriteString("    " + line + "\n")
	}
}
