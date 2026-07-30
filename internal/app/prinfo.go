package app

import (
	"fmt"
	"strings"

	"github.com/muesli/reflow/wordwrap"

	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/ui"
)

// openPRInfo opens the pull-request details overlay (p toggles it).
func (m *Model) openPRInfo() {
	if !m.prActive() || m.pr.PR == nil {
		m.setStatus("no pull request (local review)")
		return
	}
	m.prScroll = 0
	m.mode = ModePR
}

// handlePRKey handles keys while the PR details overlay is open.
func (m *Model) handlePRKey(key string) {
	switch key {
	case "esc", "q", "p":
		m.mode = ModeNormal
	case "j", "down":
		m.prScroll++
	case "k", "up":
		m.prScroll--
	case "ctrl+d", "pgdown":
		m.prScroll += m.contentHeight() / 2
	case "ctrl+u", "pgup":
		m.prScroll -= m.contentHeight() / 2
	case "g":
		m.prScroll = 0
	case "G":
		m.prScroll = 1 << 30 // clamped against the content in prInfoView
	}
	if m.prScroll < 0 {
		m.prScroll = 0
	}
}

// prInfoView renders the PR header (title, author, branches, URL) followed by
// the description as styled Markdown, applying the scroll offset.
func (m *Model) prInfoView() string {
	pr := m.pr.PR
	w := m.width - 2
	if w < 10 {
		w = 10
	}

	url := pr.URL
	if url == "" {
		url = m.pr.Ref.WebURL()
	}
	branches := ""
	if pr.BaseRef != "" || pr.HeadRef != "" {
		branches = fmt.Sprintf("   %s ← %s", pr.BaseRef, pr.HeadRef)
	}

	lines := []string{
		m.theme.Faint.Render(fmt.Sprintf("Pull request %s  (esc to close, j/k to scroll)", m.pr.Ref)),
		"",
	}
	for _, ln := range strings.Split(wordwrap.String(pr.Title, w), "\n") {
		lines = append(lines, m.theme.Metadata.Render(ln))
	}
	if pr.Author != "" || branches != "" {
		lines = append(lines, m.theme.Faint.Render("@"+pr.Author+branches))
	}
	lines = append(lines,
		m.theme.Key.Render(url),
		m.theme.Faint.Render(strings.Repeat("─", w)),
	)
	if strings.TrimSpace(pr.Body) == "" {
		lines = append(lines, m.theme.Faint.Render("(no description)"))
	} else {
		lines = append(lines, ui.RenderMarkdown(pr.Body, w, m.theme)...)
	}

	// Clamp the scroll offset so the last page stays full.
	h := m.contentHeight()
	max := len(lines) - h
	if max < 0 {
		max = 0
	}
	if m.prScroll > max {
		m.prScroll = max
	}
	lines = lines[m.prScroll:]
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

// forgeName is the short badge for the forge hosting the current PR ("gh",
// "glab"); it is empty in local mode.
func (m *Model) forgeName() string {
	if !m.prActive() {
		return ""
	}
	return forge.KindForHost(m.pr.Ref.Host).String()
}
