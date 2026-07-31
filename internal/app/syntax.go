package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
)

// Change-line coloring modes: classic red/green for +/- lines (syntax
// reserved for context), or syntax colors everywhere with an optional faint
// background tint carrying the diff identity.
const (
	changeColorsDiff   = "diff"
	changeColorsSyntax = "syntax"
)

// syntaxActive reports whether syntax coloring participates in rendering
// right now — the highlighter must exist AND the runtime toggle be on.
func (m *Model) syntaxActive() bool {
	return m.syntaxOn && m.highlighter.Enabled()
}

// cycleSyntax is the S key: syntax with red/green changes → syntax
// everywhere (tinted) → no syntax → back. One key, three useful states,
// because the two underlying settings are rarely toggled independently
// mid-review.
func (m *Model) cycleSyntax() tea.Cmd {
	switch {
	case m.syntaxActive() && m.changeColors == changeColorsDiff:
		m.changeColors = changeColorsSyntax
		m.setStatus("syntax colors everywhere (S: highlighting off)")
	case m.syntaxActive():
		m.syntaxOn = false
		m.setStatus("syntax highlighting off (S: red/green changes)")
	default:
		m.syntaxOn = true
		m.changeColors = changeColorsDiff
		m.setStatus("syntax on, red/green changes (S: syntax everywhere)")
	}
	return m.maybeFetchHighlight()
}

// contentKey addresses the per-side content cache.
func contentKey(fileIdx int, side diff.Side) string {
	return fmt.Sprintf("%d/%s", fileIdx, side)
}

// sidePath returns the path a side's content lives under — renames make the
// old side live at the old path.
func sidePath(f *diff.FileDiff, side diff.Side) string {
	if side == diff.SideLeft && f.OldPath != "" {
		return f.OldPath
	}
	return f.Path()
}

// maybeFetchHighlight requests both sides' full content for the current file,
// once per file, when whole-file highlighting could use it. Local git reads
// are effectively free; PR-mode fetches go through the on-disk cache and are
// the cost of asking for correct colors. Failures are silent: rendering
// falls back to per-hunk stitching, which is still lexically sound within a
// hunk.
func (m *Model) maybeFetchHighlight() tea.Cmd {
	if !m.syntaxActive() || m.fetchContext == nil {
		return nil
	}
	f := m.currentFile()
	if f == nil || m.hlFetched[m.fileIdx] {
		return nil
	}
	m.hlFetched[m.fileIdx] = true
	fi := m.fileIdx
	var cmds []tea.Cmd
	for _, side := range []diff.Side{diff.SideLeft, diff.SideRight} {
		if _, ok := m.contentCache[contentKey(fi, side)]; ok {
			continue
		}
		s := side
		path := sidePath(f, s)
		cmds = append(cmds, func() tea.Msg {
			data, err := m.fetchContext(m.ctx, path, s)
			return contextContentMsg{fileIdx: fi, side: s, data: data, err: err}
		})
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// fileLines returns the whole-file highlight pass for a side, building and
// memoizing it from cached content. ok=false when the content has not
// arrived (or the file has none on that side).
func (m *Model) fileLines(side diff.Side) ([]string, bool) {
	key := contentKey(m.fileIdx, side)
	if lines, ok := m.hlFileLines[key]; ok {
		return lines, lines != nil
	}
	content, ok := m.contentCache[key]
	if !ok {
		return nil, false
	}
	f := m.currentFile()
	lines := m.highlighter.ContentLines(sidePath(f, side), content)
	m.hlFileLines[key] = lines
	return lines, lines != nil
}

// rowSideLine picks the side and 1-based line number a row's text lives at:
// deletions on the old side, everything else on the new.
func rowSideLine(r *diff.DisplayRow) (diff.Side, int, bool) {
	if r.Left != nil && r.Left.Kind == diff.LineDeletion {
		if r.Left.LineNumber != nil {
			return diff.SideLeft, *r.Left.LineNumber, true
		}
		return diff.SideLeft, 0, false
	}
	if r.Right != nil && r.Right.LineNumber != nil {
		return diff.SideRight, *r.Right.LineNumber, true
	}
	if r.Left != nil && r.Left.LineNumber != nil {
		return diff.SideLeft, *r.Left.LineNumber, true
	}
	return diff.SideRight, 0, false
}

// syntaxLineFor returns the best highlighted rendering of a row's text, in
// declining fidelity: the whole-file pass (cross-line lexer state correct),
// the per-hunk stitched pass (correct within the hunk), and finally the
// legacy per-line highlight. The caller has already decided syntax applies.
func (m *Model) syntaxLineFor(r *diff.DisplayRow) string {
	if side, n, ok := rowSideLine(r); ok {
		if lines, ok2 := m.fileLines(side); ok2 && n >= 1 && n <= len(lines) {
			return lines[n-1]
		}
	}
	if line, ok := m.stitchedLine(r); ok {
		return line
	}
	f := m.currentFile()
	path := ""
	if f != nil {
		path = f.Path()
	}
	return m.highlight(path, r.Left.Text)
}

// stitchedLine reconstructs the row's side of its hunk as one block and
// highlights that: lexically sound within the hunk, which is everything the
// diff-only view shows anyway. Memoized per (file, hunk, side).
func (m *Model) stitchedLine(r *diff.DisplayRow) (string, bool) {
	if r.Source == nil {
		return "", false
	}
	f := m.currentFile()
	if f == nil || r.Source.HunkIndex >= len(f.Hunks) {
		return "", false
	}
	side, _, ok := rowSideLine(r)
	if !ok {
		return "", false
	}
	key := fmt.Sprintf("%d/%d/%s", m.fileIdx, r.Source.HunkIndex, side)
	byIdx, ok := m.hlHunkLines[key]
	if !ok {
		h := &f.Hunks[r.Source.HunkIndex]
		// The side's image of the hunk: its own lines plus shared context.
		var texts []string
		var idxs []int
		for li := range h.Lines {
			l := &h.Lines[li]
			onSide := (side == diff.SideLeft && l.OldLine != nil) || (side == diff.SideRight && l.NewLine != nil)
			if onSide {
				texts = append(texts, l.Text)
				idxs = append(idxs, li)
			}
		}
		byIdx = map[int]string{}
		if lines := m.highlighter.ContentLines(sidePath(f, side), []byte(strings.Join(texts, "\n"))); lines != nil {
			for pos, li := range idxs {
				if pos < len(lines) {
					byIdx[li] = lines[pos]
				}
			}
		}
		m.hlHunkLines[key] = byIdx
	}
	line, ok := byIdx[r.Source.LineIndex]
	return line, ok
}

// tintFor returns the faint background style carrying diff identity for a
// changed line in syntax mode, or ok=false when tinting does not apply.
func (m *Model) tintFor(kind diff.LineKind) (st func(string) string, ok bool) {
	if !m.changeTint || m.changeColors != changeColorsSyntax {
		return nil, false
	}
	switch kind {
	case diff.LineAddition:
		return func(s string) string { return m.theme.AdditionTint.Render(s) }, true
	case diff.LineDeletion:
		return func(s string) string { return m.theme.DeletionTint.Render(s) }, true
	default:
		return nil, false
	}
}
