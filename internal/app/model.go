package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/forge"
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

	// PR is set in pull-request mode; nil for local/patch review.
	PR *PRContext

	// Highlighter, when set, overrides the environment-derived default.
	Highlighter *ui.Highlighter

	// Keys overrides individual normal-mode key bindings (key -> action).
	Keys map[string]string

	// Wrap enables line/comment wrapping at start (default true when built via
	// the CLI); WrapWidth caps the unified wrap point (0 means the default).
	Wrap      bool
	WrapWidth int

	// RawPatch is the literal diff text, when the source can provide it. It
	// is what makes a review-exchange export (:export x.json) self-contained;
	// nil disables that export form.
	RawPatch []byte

	// FetchContext returns the full new-side content of a file, enabling the
	// full-file context view (T). nil disables the toggle for this source.
	FetchContext func(ctx context.Context, path string) ([]byte, error)

	// Images selects comment-image rendering: auto, kitty, chafa, or off.
	Images string

	// Author is the reviewer's name for attribution in review-exchange
	// conversations (comment replies); empty falls back to "reviewer".
	Author string
}

// Model is the root Bubble Tea model.
type Model struct {
	files   []diff.FileDiff
	fileIdx int
	layout  Layout

	rowCache map[string][]diff.DisplayRow

	// folded records collapsed hunks, keyed by "fileIdx/hunkIdx".
	folded map[string]bool

	// sidebar toggles the persistent changed-files list.
	sidebar bool

	// inlineComments shows comment previews under annotated lines (default on).
	inlineComments bool

	// wrapText wraps long diff lines and comment previews (w toggles);
	// wrapWidth caps the unified wrap point (split wraps at the panel width).
	wrapText  bool
	wrapWidth int

	// highlighter renders source syntax; hlCache memoizes per (path,text).
	highlighter *ui.Highlighter
	hlCache     map[string]string

	// images renders comment-thread image references (kitty/chafa/off).
	images *ui.ImageRenderer

	cursor  int // index into the current file's rows
	top     int // first visible row (vertical scroll offset)
	hscroll int // horizontal scroll offset in columns

	width  int
	height int

	mode      Mode
	selAnchor int // -1 when no selection is active

	// activeSide selects which side of a both-sided split row the cursor
	// targets (h/l). Meaningful only in split layout; unified rows derive
	// their side from the line kind.
	activeSide diff.Side
	pending    pendingCommand
	keymap     Keymap

	// listCursor is the selection index for the files / comments overlays.
	listCursor int

	// cmdlineActive is true while the user is typing a ":" command.
	cmdlineActive bool
	cmdline       string

	// search input state ("/") and the committed query used for highlighting
	// and n/N navigation.
	searchActive bool
	searchInput  string
	search       string

	draft  *review.DraftReview
	store  *review.Store
	editor editor.Editor
	theme  ui.Theme

	title   string
	headOID string
	status  string
	err     error

	// pr and threadIndex are populated in pull-request mode.
	pr          *PRContext
	threadIndex map[string][]int
	// threadView holds the thread indices shown in the thread reader.
	threadView []int
	// convoID/convoSel drive the draft-conversation reader: the comment shown
	// and the selected item (0 = the comment, 1.. = its replies).
	convoID  string
	convoSel int
	// prScroll is the vertical scroll offset of the PR details overlay.
	prScroll int

	// pendingEvent is the review event awaiting confirmation; submitting is set
	// while a submission is in flight.
	pendingEvent forge.ReviewEvent
	submitting   bool

	// inflight carries the location/snippet while the external editor is open.
	inflight *pendingEdit

	// contextView is the full-file context toggle (T); contextRows holds the
	// per-file context projections, built lazily from fetched content;
	// fetchContext is the cmd-injected fetcher (nil: unsupported source).
	contextView  bool
	contextRows  map[int][]diff.DisplayRow
	fetchContext func(ctx context.Context, path string) ([]byte, error)

	// rawPatch and exchangePath support review-exchange conversations: the
	// literal diff for self-contained exports, and (when the session was
	// opened from an exchange file) the writeback target every draft save
	// also refreshes.
	rawPatch     []byte
	exchangePath string
	// author attributes this reviewer's exchange replies.
	author string

	ctx      context.Context
	quitting bool
}

type pendingEdit struct {
	loc     diff.Location
	snippet string
	replyTo *int64
	editing string // local id when editing an existing draft; "" when new
	// replyToLocal is the draft comment a conversation reply attaches to
	// (review-exchange flow); distinct from replyTo, which targets a host
	// thread comment in PR mode. editReplyAt, when set, is the index of an
	// existing reply being edited rather than a new one being appended.
	replyToLocal string
	editReplyAt  *int
	session      *editor.Session
}

// New builds the initial model, filling in whatever cfg leaves unset: an
// empty draft, the environment-derived highlighter, the default wrap width,
// and the default keymap overlaid with the user's overrides. The cursor
// starts on the first commentable row rather than row 0, which is a hunk
// header.
func New(cfg Config) *Model {
	m := &Model{
		files:          cfg.Files,
		layout:         LayoutUnified,
		rowCache:       map[string][]diff.DisplayRow{},
		folded:         map[string]bool{},
		highlighter:    cfg.Highlighter,
		hlCache:        map[string]string{},
		selAnchor:      -1,
		draft:          cfg.Draft,
		store:          cfg.Store,
		editor:         cfg.Editor,
		theme:          cfg.Theme,
		title:          cfg.Title,
		headOID:        cfg.HeadOID,
		mode:           ModeNormal,
		activeSide:     diff.SideRight,
		inlineComments: true,
		wrapText:       cfg.Wrap,
		wrapWidth:      cfg.WrapWidth,
		ctx:            context.Background(),
		pr:             cfg.PR,
		rawPatch:       cfg.RawPatch,
		author:         cfg.Author,
		contextRows:    map[int][]diff.DisplayRow{},
		fetchContext:   cfg.FetchContext,
		images:         ui.NewImageRenderer(cfg.Images),
	}
	if m.draft == nil {
		m.draft = review.NewDraftReview("", cfg.Title, cfg.HeadOID)
	}
	if m.highlighter == nil {
		m.highlighter = ui.NewHighlighterFromEnv()
	}
	if m.wrapWidth == 0 {
		m.wrapWidth = 120
	}
	m.keymap = DefaultKeymap()
	m.keymap.apply(cfg.Keys)
	m.buildThreadIndex()
	m.cursor = m.firstContentRow()
	return m
}

// Init implements tea.Model. All data arrives fully loaded through Config, so
// there is no startup command to run.
func (m *Model) Init() tea.Cmd { return nil }

// SetExchangeWriteback makes every draft save also rewrite the review-exchange
// file at path, keeping the on-disk conversation current without an explicit
// export step — the file is the contract with the other side of the review.
func (m *Model) SetExchangeWriteback(path string) { m.exchangePath = path }

// rawRows returns the unfolded display rows for the current file and layout,
// cached. Folding is applied on top by rows(). With the context view active,
// the pre-built full-file projection replaces the diff-only one.
func (m *Model) rawRows() []diff.DisplayRow {
	if len(m.files) == 0 {
		return nil
	}
	if m.contextActive() {
		return m.contextRows[m.fileIdx]
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
	// Reserve three lines: the two-line title bar and the bottom status bar.
	h := m.height - 3
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

// highlight returns the syntax-highlighted form of a source line, memoized.
func (m *Model) highlight(path, text string) string {
	if !m.highlighter.Enabled() || text == "" {
		return text
	}
	key := path + "\x00" + text
	if v, ok := m.hlCache[key]; ok {
		return v
	}
	v := m.highlighter.Line(path, text)
	m.hlCache[key] = v
	return v
}
