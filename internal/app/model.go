package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/review"
	"github.com/perrito666/leanreview/internal/ui"
)

// Config wires the model's dependencies together.
type Config struct {
	Files   []diff.FileDiff
	Title   string
	HeadOID string
	Draft   *review.DraftReview
	Store   *review.Store
	Editor  editor.Editor
	Theme   ui.Theme
}

// Model is the root Bubble Tea model.
type Model struct {
	files   []diff.FileDiff
	fileIdx int
	layout  Layout

	rowCache map[string][]diff.DisplayRow

	cursor int // index into the current file's rows
	top    int // first visible row (scroll offset)

	width  int
	height int

	mode      Mode
	selAnchor int // -1 when no selection is active
	pending   pendingCommand

	// listCursor is the selection index for the files / comments overlays.
	listCursor int

	// cmdlineActive is true while the user is typing a ":" command.
	cmdlineActive bool
	cmdline       string

	draft  *review.DraftReview
	store  *review.Store
	editor editor.Editor
	theme  ui.Theme

	title   string
	headOID string
	status  string
	err     error

	// inflight carries the location/snippet while the external editor is open.
	inflight *pendingEdit

	ctx      context.Context
	quitting bool
}

type pendingEdit struct {
	loc     diff.Location
	snippet string
	replyTo *int64
	editing string // local id when editing an existing draft; "" when new
	session *editor.Session
}

// New builds the initial model.
func New(cfg Config) *Model {
	m := &Model{
		files:     cfg.Files,
		layout:    LayoutUnified,
		rowCache:  map[string][]diff.DisplayRow{},
		selAnchor: -1,
		draft:     cfg.Draft,
		store:     cfg.Store,
		editor:    cfg.Editor,
		theme:     cfg.Theme,
		title:     cfg.Title,
		headOID:   cfg.HeadOID,
		mode:      ModeNormal,
		ctx:       context.Background(),
	}
	if m.draft == nil {
		m.draft = review.NewDraftReview("", cfg.Title, cfg.HeadOID)
	}
	m.cursor = m.firstContentRow()
	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// rows returns the display rows for the current file and layout, cached.
func (m *Model) rows() []diff.DisplayRow {
	if len(m.files) == 0 {
		return nil
	}
	key := fmt.Sprintf("%d/%d", m.fileIdx, m.layout)
	if r, ok := m.rowCache[key]; ok {
		return r
	}
	f := &m.files[m.fileIdx]
	var r []diff.DisplayRow
	if m.layout == LayoutSplit {
		r = diff.RenderSplit(f)
	} else {
		r = diff.RenderUnified(f)
	}
	m.rowCache[key] = r
	return r
}

// currentFile returns the file under review, or nil when there are none.
func (m *Model) currentFile() *diff.FileDiff {
	if len(m.files) == 0 {
		return nil
	}
	return &m.files[m.fileIdx]
}

// rowAt returns the row at index i, or nil if out of range.
func (m *Model) rowAt(i int) *diff.DisplayRow {
	rows := m.rows()
	if i < 0 || i >= len(rows) {
		return nil
	}
	return &rows[i]
}

// firstContentRow returns the index of the first commentable row (skipping the
// leading hunk header), or 0.
func (m *Model) firstContentRow() int {
	for i, r := range m.rows() {
		if r.Source != nil {
			return i
		}
	}
	return 0
}

// contentHeight is the number of diff rows visible between header and status.
func (m *Model) contentHeight() int {
	// Reserve two lines: the top title bar and the bottom status bar.
	h := m.height - 2
	if h < 1 {
		return 1
	}
	return h
}

// setStatus records a transient status-bar message and clears any error.
func (m *Model) setStatus(format string, args ...any) {
	m.status = fmt.Sprintf(format, args...)
	m.err = nil
}

// setError records an error to surface in the status bar.
func (m *Model) setError(err error) {
	m.err = err
}

// invalidateRows drops the row cache (after a layout or file mutation).
func (m *Model) invalidateRows() {
	m.rowCache = map[string][]diff.DisplayRow{}
}
