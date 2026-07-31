package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
)

// startSuggestion opens the editor pre-filled with a ```suggestion fence
// containing the selected lines' verbatim code, GitHub-style: edit the block
// into the proposed replacement, save, done. The fence is what the hosts
// render as an applyable suggestion, so submission needs no extra machinery.
// The pre-fill uses RawText — the tab-expanded display form would corrupt
// the patch the host builds from the suggestion.
func (m *Model) startSuggestion() tea.Cmd {
	loc, snippet, err := m.buildLocation()
	if err != nil {
		m.setError(err)
		return nil
	}
	if loc.Side != diff.SideRight {
		m.setStatus("suggestions replace new-side lines — select on the RIGHT side")
		return nil
	}
	f := m.currentFile()
	if f == nil {
		return nil
	}

	// The selected rows' verbatim code, in order.
	rows := m.rows()
	lo, hi := m.selectionRange()
	if m.selAnchor < 0 {
		lo, hi = m.cursor, m.cursor
	}
	var lines []string
	for i := lo; i <= hi && i < len(rows); i++ {
		r := &rows[i]
		if r.Source == nil || r.Source.Side != loc.Side {
			continue
		}
		if r.Source.HunkIndex < len(f.Hunks) && r.Source.LineIndex < len(f.Hunks[r.Source.HunkIndex].Lines) {
			lines = append(lines, f.Hunks[r.Source.HunkIndex].Lines[r.Source.LineIndex].RawText())
		}
	}
	if len(lines) == 0 {
		m.setStatus("no code lines selected to suggest on")
		return nil
	}

	body := "```suggestion\n" + strings.Join(lines, "\n") + "\n```\n"
	tctx := editor.TemplateContext{
		File:  loc.Path,
		Lines: lineRefString(loc),
		Side:  loc.Side.String(),
	}
	sess, err := editor.NewSession(editor.BuildTemplate(tctx, body), fmt.Sprintf("suggest-%s-%s", filepath.Base(loc.Path), lineRefString(loc)))
	if err != nil {
		m.setError(err)
		return nil
	}
	m.inflight = &pendingEdit{loc: loc, snippet: snippet, session: sess}
	m.mode = ModeExternalEditor
	c := m.editor.Cmd(m.ctx, sess.Path)
	return tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
}

// annLine is one display line of a comment body: plain prose, a line of
// suggested code, or the label introducing a suggestion block.
type annLine struct {
	text string
	kind annLineKind
}

type annLineKind uint8

const (
	annProse annLineKind = iota
	annSuggestLabel
	annSuggestCode
)

// splitBodyLines renders a comment body into display lines, replacing
// ```suggestion fences (GitHub's, and GitLab's ranged ```suggestion:-N+M
// form) with a label plus styled code lines — a suggestion shown as plain
// quoted text would be indistinguishable from prose, which is exactly what
// it is not. prefix decorates the first prose line (the "● @author:"
// lead-in); when the body opens with a suggestion, attribution still gets
// its own line.
func splitBodyLines(prefix, body string) []annLine {
	var out []annLine
	prefixed := prefix == ""
	lead := func() string {
		if prefixed {
			return "  "
		}
		prefixed = true
		return prefix
	}
	inFence := false
	for _, ln := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		trimmed := strings.TrimSpace(ln)
		switch {
		case !inFence && strings.HasPrefix(trimmed, "```suggestion"):
			inFence = true
			if !prefixed {
				out = append(out, annLine{text: strings.TrimRight(prefix, ": ")})
				prefixed = true
			}
			out = append(out, annLine{text: "suggested change:", kind: annSuggestLabel})
		case inFence && trimmed == "```":
			inFence = false
		case inFence:
			out = append(out, annLine{text: ln, kind: annSuggestCode})
		default:
			out = append(out, annLine{text: lead() + ln})
		}
	}
	if !prefixed {
		out = append(out, annLine{text: strings.TrimRight(prefix, ": ")})
	}
	return out
}
