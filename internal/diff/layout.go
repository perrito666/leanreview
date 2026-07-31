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

	// Annotation marks a display-only row injected under a diff line (e.g. an
	// inline comment preview). Annotation rows carry no Source, are skipped by
	// navigation, and are not hunk headers.
	Annotation bool

	// Edge marks the horizontal border rows of a boxed annotation; text rows
	// keep EdgeNone. Meaningful only when Annotation is set.
	Edge AnnotationEdge

	// Continuation marks the overflow of a wrapped diff line: it renders with
	// the line's own styling but carries no line numbers or Source, so
	// navigation and selection treat the logical line as one unit.
	Continuation bool

	// Separator marks the boundary row drawn between hunks, so the reader can
	// see where one excerpt ends and the next begins when the surrounding
	// file context is not shown. Display-only: no Source, skipped by
	// navigation.
	Separator bool

	// Pre marks an annotation text row whose content is preformatted terminal
	// output (an image rendered to cells): renderers must clip it ANSI-aware
	// and never wrap or restyle it. Meaningful only with Annotation set.
	Pre bool
}

// AnnotationEdge distinguishes the border rows of a boxed annotation from its
// text rows. EdgeDivider separates items of a thread that share one box.
type AnnotationEdge uint8

const (
	EdgeNone AnnotationEdge = iota
	EdgeTop
	EdgeBottom
	EdgeDivider
)

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
