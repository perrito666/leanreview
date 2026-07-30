package diff

// RenderUnified projects a FileDiff into unified-layout rows: each logical diff
// line becomes exactly one row. The Left cell carries the old-side number, the
// Right cell the new-side number, and the text is shared; callers style by
// Kind. Every content row gets a Source so it can be commented on.
func RenderUnified(f *FileDiff) []DisplayRow {
	var rows []DisplayRow
	for hi := range f.Hunks {
		h := &f.Hunks[hi]
		rows = append(rows, DisplayRow{
			Left: &DisplayCell{Kind: LineMetadata, Text: h.Header},
		})
		for li := range h.Lines {
			l := &h.Lines[li]
			row := DisplayRow{
				Left:  &DisplayCell{LineNumber: l.OldLine, Kind: l.Kind, Text: l.Text},
				Right: &DisplayCell{LineNumber: l.NewLine, Kind: l.Kind, Text: l.Text},
			}
			row.Source = rowSource(f, l.Kind.Side(), l, hi, li)
			rows = append(rows, row)
		}
	}
	return rows
}
