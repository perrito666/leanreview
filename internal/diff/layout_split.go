package diff

// idxLine pairs a diff line with its index inside the hunk, so paired rows can
// still resolve back to a stable Location.
type idxLine struct {
	line *DiffLine
	idx  int
}

// RenderSplit projects a FileDiff into split-layout rows: a left (old) column
// and a right (new) column. Context lines occupy both columns. Change blocks
// are paired with a simple rule — gather consecutive deletions, then the
// immediately following additions, pair them by index, and render any leftover
// against a blank cell on the opposite side. Similarity/intraline matching is
// intentionally out of scope for v1.
func RenderSplit(f *FileDiff) []DisplayRow {
	if r := binaryPlaceholder(f); r != nil {
		return r
	}
	var rows []DisplayRow
	for hi := range f.Hunks {
		h := &f.Hunks[hi]
		if hi > 0 {
			// Same boundary rule as the unified projection: disjoint excerpts
			// get a visible seam.
			rows = append(rows, DisplayRow{Separator: true})
		}
		rows = append(rows, DisplayRow{Left: &DisplayCell{Kind: LineMetadata, Text: h.Header}})

		lines := h.Lines
		i := 0
		for i < len(lines) {
			if lines[i].Kind == LineContext {
				l := &lines[i]
				rows = append(rows, DisplayRow{
					Left:      &DisplayCell{LineNumber: l.OldLine, Kind: LineContext, Text: l.Text},
					Right:     &DisplayCell{LineNumber: l.NewLine, Kind: LineContext, Text: l.Text},
					Source:    rowSource(f, SideRight, l, hi, i),
					AltSource: rowSource(f, SideLeft, l, hi, i),
				})
				i++
				continue
			}

			// Gather a maximal run of changed lines, split by side.
			var dels, adds []idxLine
			for i < len(lines) && lines[i].Kind != LineContext {
				switch lines[i].Kind {
				case LineDeletion:
					dels = append(dels, idxLine{&lines[i], i})
				case LineAddition:
					adds = append(adds, idxLine{&lines[i], i})
				default:
					// Metadata line inside a change block: render on the left.
					rows = append(rows, DisplayRow{Left: &DisplayCell{Kind: LineMetadata, Text: lines[i].Text}})
				}
				i++
			}

			n := len(dels)
			if len(adds) > n {
				n = len(adds)
			}
			for k := 0; k < n; k++ {
				row := DisplayRow{}
				if k < len(dels) {
					d := dels[k]
					row.Left = &DisplayCell{LineNumber: d.line.OldLine, Kind: LineDeletion, Text: d.line.Text}
					row.Source = rowSource(f, SideLeft, d.line, hi, d.idx)
				}
				if k < len(adds) {
					a := adds[k]
					row.Right = &DisplayCell{LineNumber: a.line.NewLine, Kind: LineAddition, Text: a.line.Text}
					// An addition anchors the right side; when the row also
					// carries a deletion, keep it reachable via AltSource.
					row.AltSource = row.Source
					row.Source = rowSource(f, SideRight, a.line, hi, a.idx)
				}
				rows = append(rows, row)
			}
		}
	}
	return rows
}
