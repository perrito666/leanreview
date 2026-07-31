package diff

import (
	"fmt"
	"strings"
)

// RenderUnifiedContext projects a FileDiff over the full new-side file
// content: every file line becomes a context row, with the hunks' own rows
// (including deletions) spliced in at their positions — the diff as it reads
// inside the file, like an unbounded -U. Each hunk is bracketed by boundary
// rows (a rule plus its @@ header on entry, a rule on exit) so the reader
// can tell exactly where the reviewed excerpt begins and ends inside the
// surrounding file. Gap rows carry both line numbers but no Source: hosts
// anchor review comments to diff positions, so lines outside the hunks are
// deliberately not commentable. Hunk rows keep their exact Sources, so
// comments, threads, and navigation behave identically in both views.
//
// Every context/addition line is verified against the file content; a
// mismatch aborts with an error rather than rendering a lie — it means the
// content is from the wrong revision.
func RenderUnifiedContext(f *FileDiff, content []byte) ([]DisplayRow, error) {
	if f.IsBinary {
		return nil, fmt.Errorf("binary file has no textual context")
	}
	// Normalize to the parser's display form (tab expansion) — hunk Text was
	// normalized at parse time, and comparing raw bytes against it would
	// misfire on every tab-indented line.
	lines := strings.Split(strings.TrimRight(string(NormalizeContent(content)), "\n"), "\n")

	var rows []DisplayRow
	newPos := 1 // next new-side line not yet emitted
	delta := 0  // new - old, accumulated over emitted hunks

	// gap emits full-context rows for new lines [newPos, upto].
	gap := func(upto int) {
		for ; newPos <= upto && newPos <= len(lines); newPos++ {
			n := newPos
			o := n - delta
			rows = append(rows, DisplayRow{
				Left:  &DisplayCell{LineNumber: &o, Kind: LineContext, Text: lines[n-1]},
				Right: &DisplayCell{LineNumber: &n, Kind: LineContext, Text: lines[n-1]},
			})
		}
	}

	for hi := range f.Hunks {
		h := &f.Hunks[hi]
		anchor, adds, dels := hunkGeometry(h, delta)
		gap(anchor - 1)
		// Hunk boundary in: a rule plus the hunk header, so the excerpt's
		// extent and identity stay visible inside the full file. Adjacent
		// hunks share the single rule the previous hunk's exit left behind,
		// and a hunk at the very top of the file needs no rule above its
		// header.
		if n := len(rows); n > 0 && !rows[n-1].Separator {
			rows = append(rows, DisplayRow{Separator: true})
		}
		rows = append(rows, DisplayRow{Left: &DisplayCell{Kind: LineMetadata, Text: h.Header}})
		for li := range h.Lines {
			l := &h.Lines[li]
			if l.NewLine != nil {
				if n := *l.NewLine; n < 1 || n > len(lines) || lines[n-1] != l.Text {
					return nil, fmt.Errorf("%s: line %d does not match the diff — the fetched content is from a different revision", f.Path(), n)
				}
				if n := *l.NewLine; n >= newPos {
					newPos = n + 1
				}
			}
			rows = append(rows, DisplayRow{
				Left:   &DisplayCell{LineNumber: l.OldLine, Kind: l.Kind, Text: l.Text},
				Right:  &DisplayCell{LineNumber: l.NewLine, Kind: l.Kind, Text: l.Text},
				Source: rowSource(f, l.Kind.Side(), l, hi, li),
			})
		}
		// Hunk boundary out.
		rows = append(rows, DisplayRow{Separator: true})
		delta += adds - dels
	}
	gap(len(lines))
	return rows, nil
}

// hunkGeometry derives where a hunk begins on the new side and its line-count
// change. A hunk that only deletes has no new-side lines of its own; its
// anchor is where the deleted block would sit — the old start shifted by the
// running delta.
func hunkGeometry(h *Hunk, delta int) (anchorNew, adds, dels int) {
	anchorNew = -1
	for i := range h.Lines {
		l := &h.Lines[i]
		switch l.Kind {
		case LineAddition:
			adds++
		case LineDeletion:
			dels++
		}
		if anchorNew == -1 && l.NewLine != nil {
			anchorNew = *l.NewLine
		}
	}
	if anchorNew == -1 {
		for i := range h.Lines {
			if l := &h.Lines[i]; l.OldLine != nil {
				anchorNew = *l.OldLine + delta
				break
			}
		}
	}
	if anchorNew < 1 {
		anchorNew = 1
	}
	return anchorNew, adds, dels
}
