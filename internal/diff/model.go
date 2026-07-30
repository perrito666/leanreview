// Package diff holds the canonical, view-independent representation of a set of
// changes under review, plus projections of that model into rows for display.
//
// The model here is deliberately decoupled from any terminal concern: rendered
// screen rows are never canonical comment locations. Comments point at semantic
// positions in this model (see the review package), which survive terminal
// resize, context folding, and switching between unified and split layouts.
package diff

// Side identifies which side of a diff a line or selection refers to. It maps
// directly onto GitHub's LEFT/RIGHT review-comment sides so that PR mode is a
// no-op switch over the same model.
type Side uint8

const (
	// SideLeft is the old (pre-image) side: deleted lines live here.
	SideLeft Side = iota
	// SideRight is the new (post-image) side: added and context lines live here.
	SideRight
)

// String returns the GitHub API spelling of the side ("LEFT"/"RIGHT").
func (s Side) String() string {
	if s == SideLeft {
		return "LEFT"
	}
	return "RIGHT"
}

// LineKind classifies a single logical diff line.
type LineKind uint8

const (
	// LineContext is an unchanged line present on both sides.
	LineContext LineKind = iota
	// LineAddition is a line added on the new side.
	LineAddition
	// LineDeletion is a line removed from the old side.
	LineDeletion
	// LineMetadata is a non-content marker line (e.g. "No newline at end of file").
	LineMetadata
)

// Side reports which diff side a line of this kind naturally belongs to.
// Additions and context default to the right; deletions to the left.
func (k LineKind) Side() Side {
	if k == LineDeletion {
		return SideLeft
	}
	return SideRight
}

// FileStatus describes what happened to a file across the diff.
type FileStatus uint8

const (
	// StatusModified is the default: content changed in place.
	StatusModified FileStatus = iota
	StatusAdded
	StatusDeleted
	StatusRenamed
	StatusCopied
	StatusBinary
)

// String renders a short human label for the status.
func (s FileStatus) String() string {
	switch s {
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusCopied:
		return "copied"
	case StatusBinary:
		return "binary"
	default:
		return "modified"
	}
}

// DiffLine is one logical line of a hunk. Exactly one of OldLine/NewLine is nil
// for pure additions/deletions; both are set for context lines.
type DiffLine struct {
	Kind LineKind
	Text string

	// OldLine is the 1-based line number on the old side, or nil for additions.
	OldLine *int
	// NewLine is the 1-based line number on the new side, or nil for deletions.
	NewLine *int

	// PatchPosition is the 1-based offset of this line within the file's patch
	// (counting from the first hunk header, as GitHub numbers review positions),
	// or nil for lines that have no patch position. Preserved to reconcile our
	// parsed model with comments returned by the GitHub API.
	PatchPosition *int
}

// Hunk is a contiguous block of changes with its "@@ ... @@" header.
type Hunk struct {
	Header string
	Lines  []DiffLine
}

// FileDiff is the full set of changes to a single file.
type FileDiff struct {
	OldPath string
	NewPath string

	Status FileStatus
	Hunks  []Hunk

	IsBinary  bool
	IsRenamed bool

	// RawPatch is the original unified-diff text for this file, preserved for
	// export snippets and (later) GitHub reconciliation.
	RawPatch string
}

// Path returns the most meaningful path for display: the new path unless the
// file was deleted, in which case the old path.
func (f *FileDiff) Path() string {
	if f.Status == StatusDeleted || f.NewPath == "" {
		return f.OldPath
	}
	return f.NewPath
}

// LineAt returns the line at (hunkIndex, lineIndex), or nil if out of range.
func (f *FileDiff) LineAt(hunkIndex, lineIndex int) *DiffLine {
	if hunkIndex < 0 || hunkIndex >= len(f.Hunks) {
		return nil
	}
	h := f.Hunks[hunkIndex]
	if lineIndex < 0 || lineIndex >= len(h.Lines) {
		return nil
	}
	return &h.Lines[lineIndex]
}
