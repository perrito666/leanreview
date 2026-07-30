package diff

// DisplayCell is one rendered cell: a line number (optional), the kind of line
// it represents (for styling), and the raw text. A nil cell renders as blank
// (used for the empty side of an unpaired addition/deletion in split view).
type DisplayCell struct {
	LineNumber *int
	Kind       LineKind
	Text       string
}

// DisplayRow is one rendered row of a diff view. In unified layout only Left is
// used to carry the number gutter is folded into a single cell; in split layout
// Left and Right are the two sides. Source points back at the semantic location
// this row anchors to, so a comment made on a screen row resolves to a stable
// Location rather than a terminal coordinate.
type DisplayRow struct {
	Left  *DisplayCell
	Right *DisplayCell

	// Source is the semantic location of this row (start line = end line here);
	// nil for rows that cannot be commented on (e.g. hunk headers). For split
	// rows carrying both sides it points at the right/new side.
	Source *Location

	// AltSource is the location of the opposite (left/old) side when a split
	// row carries both sides — a paired deletion/addition or a context line —
	// so the cursor can select either side. Nil elsewhere.
	AltSource *Location
}

// binaryPlaceholder returns a one-row informational rendering for binary files
// (which have no hunks and would otherwise draw a blank body), or nil for text
// files.
func binaryPlaceholder(f *FileDiff) []DisplayRow {
	if !f.IsBinary {
		return nil
	}
	return []DisplayRow{{Left: &DisplayCell{Kind: LineMetadata, Text: "(binary file — no textual diff)"}}}
}

// rowSource builds a single-line Location for the line at (hunkIndex, lineIndex).
func rowSource(f *FileDiff, side Side, line *DiffLine, hunkIndex, lineIndex int) *Location {
	n, ok := line.LineNumber(side)
	if !ok {
		return nil
	}
	loc := &Location{
		Path:      f.Path(),
		Side:      side,
		StartLine: n,
		EndLine:   n,
		HunkIndex: hunkIndex,
		LineIndex: lineIndex,
		Anchor:    NewContextAnchor(f, hunkIndex, lineIndex, 3),
	}
	return loc
}
