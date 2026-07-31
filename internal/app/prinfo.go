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
		lines = append(lines, ui.RenderMarkdown(stripImageMarkup(pr.Body), w, m.theme)...)
		lines = append(lines, m.imageLines(pr.Body, w)...)
	}

	// The PR's conversation-level discussion, oldest first — the part of a
	// review that anchors to the request rather than to a line, previously
	// invisible in the TUI.
	if len(m.pr.General) > 0 {
		lines = append(lines, "", m.theme.Faint.Render(strings.Repeat("─", w)),
			m.theme.Metadata.Render(fmt.Sprintf("Conversation (%d)", len(m.pr.General))))
		for _, c := range m.pr.General {
			when := ""
			if !c.CreatedAt.IsZero() {
				when = "  " + c.CreatedAt.Format("2006-01-02 15:04")
			}
			lines = append(lines, "", m.theme.Key.Render("@"+c.Author)+m.theme.Faint.Render(when))
			lines = append(lines, ui.RenderMarkdown(stripImageMarkup(c.Body), w, m.theme)...)
			lines = append(lines, m.imageLines(c.Body, w)...)
		}
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

// imageLines renders a body's image references for the overlay — real cell
// art when the attachment is local (or already fetched) and a renderer is
// active, a textual tag otherwise. It mirrors imageRows but yields plain
// lines: overlays are line-oriented, not diff-row-oriented. The body text
// itself goes through stripImageMarkup, so these lines are the only place
// the image appears.
func (m *Model) imageLines(body string, w int) []string {
	var out []string
	for _, ref := range imageRefs(body) {
		tag := func(suffix string) {
			out = append(out, m.theme.Faint.Render("[image: "+ref+suffix+"]"))
		}
		if !m.images.Enabled() {
			tag(" — no image renderer; install chafa or use kitty/ghostty")
			continue
		}
		path := ref
		if isRemoteRef(ref) {
			local := m.imageFiles[ref]
			if local == "" {
				if m.imagePending[ref] {
					tag(" (fetching…)")
				} else {
					tag("")
				}
				continue
			}
			path = local
		}
		if lines, ok := m.images.Render(path, w-2, 12); ok {
			out = append(out, lines...)
		} else {
			tag("")
		}
	}
	return out
}

// forgeName is the short badge for the forge hosting the current PR ("gh",
// "glab"); it is empty in local mode.
func (m *Model) forgeName() string {
	if !m.prActive() {
		return ""
	}
	return forge.KindForHost(m.pr.Ref.Host).String()
}
