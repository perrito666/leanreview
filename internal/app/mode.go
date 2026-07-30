// Package app is the Bubble Tea model/update/view for the review client. It is
// deliberately independent of Git and any forge: it consumes a parsed diff and
// a draft review, renders them, and turns key events into navigation, selection,
// and comment actions. Rendered rows are projections; comments anchor to
// semantic diff locations.
package app

// Mode is the explicit interaction mode. Using an enum (rather than scattered
// booleans) keeps the update logic legible and testable.
type Mode uint8

const (
	// ModeNormal is navigation.
	ModeNormal Mode = iota
	// ModeVisual is an active line selection.
	ModeVisual
	// ModeFiles is the file picker.
	ModeFiles
	// ModeComments is the comment/thread list.
	ModeComments
	// ModeConfirm is a yes/no confirmation prompt.
	ModeConfirm
	// ModeHelp shows the key reference.
	ModeHelp
	// ModeExternalEditor is a placeholder state while the editor is open.
	ModeExternalEditor
)

func (m Mode) String() string {
	switch m {
	case ModeVisual:
		return "VISUAL"
	case ModeFiles:
		return "FILES"
	case ModeComments:
		return "COMMENTS"
	case ModeConfirm:
		return "CONFIRM"
	case ModeHelp:
		return "HELP"
	case ModeExternalEditor:
		return "EDITOR"
	default:
		return "NORMAL"
	}
}

// Layout selects unified vs split rendering.
type Layout uint8

const (
	LayoutUnified Layout = iota
	LayoutSplit
)

func (l Layout) String() string {
	if l == LayoutSplit {
		return "split"
	}
	return "unified"
}
