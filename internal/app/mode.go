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
	// ModeThread shows a focused reader for the review thread under the cursor.
	ModeThread
	// ModePR shows the pull-request details (title, description, URL).
	ModePR
	// ModeConvo shows a focused reader for a draft comment's conversation,
	// where individual replies can be edited and deleted.
	ModeConvo
	// ModeExternalEditor is a placeholder state while the editor is open.
	ModeExternalEditor
)

// String returns the upper-case label shown in the status bar. ModeNormal —
// and any unknown value — reads NORMAL, so the bar always shows a valid mode.
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
	case ModeThread:
		return "THREAD"
	case ModePR:
		return "PR"
	case ModeConvo:
		return "CONVERSATION"
	case ModeExternalEditor:
		return "EDITOR"
	default:
		return "NORMAL"
	}
}

// Layout selects unified vs split rendering.
type Layout uint8

const (
	// LayoutUnified interleaves old and new lines in one column.
	LayoutUnified Layout = iota
	// LayoutSplit shows old and new side by side.
	LayoutSplit
)

// String returns the lower-case layout name shown in the title bar and in the
// toggle-layout status message.
func (l Layout) String() string {
	if l == LayoutSplit {
		return "split"
	}
	return "unified"
}
