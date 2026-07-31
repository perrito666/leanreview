package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/ui"
)

// commentedModel returns a model with one draft comment on new-side line 72.
func commentedModel(t *testing.T) *Model {
	t.Helper()
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	m.draft.Add(loc, "needs a check", snip)
	return m
}

func TestCommentPreviewIsBoxed(t *testing.T) {
	m := commentedModel(t)
	out := m.View()
	for _, want := range []string{"╭─", "╮", "│ ", "╰─", "╯", "needs a check"} {
		if !strings.Contains(out, want) {
			t.Errorf("boxed preview missing %q:\n%s", want, out)
		}
	}
	// Every line still spans the full terminal width.
	lines := strings.Split(out, "\n")
	for i := 2; i < len(lines)-1; i++ {
		if w := lipgloss.Width(lines[i]); w != m.width {
			t.Fatalf("line %d width = %d, want %d", i, w, m.width)
		}
	}
}

func TestCommentBoxSitsOnRightPanelInSplit(t *testing.T) {
	m := commentedModel(t)
	m = key(m, "t") // split layout
	indent, _ := m.annotationLayout()
	rightPanelStart := 2 + m.numWidth() + 1 + m.splitPanelWidth() + 3
	if indent != rightPanelStart {
		t.Errorf("split box indent = %d, want right panel start %d", indent, rightPanelStart)
	}
	out := m.View()
	if !strings.Contains(out, "╭─") || !strings.Contains(out, "needs a check") {
		t.Fatalf("boxed preview missing in split view:\n%s", out)
	}
	// The box's border starts at the indent column, not at the left margin.
	for _, ln := range strings.Split(out, "\n") {
		if i := strings.Index(ln, "╭"); i >= 0 {
			plain := ln[:i]
			if got := lipgloss.Width(stripANSI(plain)); got < rightPanelStart {
				t.Errorf("box top border at column %d, want >= %d:\n%q", got, rightPanelStart, ln)
			}
		}
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if r == 'm' {
				inEsc = false
			}
		case r == 0x1b:
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestTabCyclesFiles(t *testing.T) {
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	data, err := os.ReadFile(filepath.Join("..", "diff", "testdata", "binary.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	files, err := diff.ParsePatchBytes(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := New(Config{Files: files, Title: "test", Theme: ui.DefaultTheme()})
	m.width, m.height = 100, 30
	if len(m.files) < 2 {
		t.Fatalf("fixture should carry two files, got %d", len(m.files))
	}
	m = key(m, "tab")
	if m.fileIdx != 1 {
		t.Errorf("tab should advance to the next file, fileIdx=%d", m.fileIdx)
	}
	m = key(m, "shift+tab")
	if m.fileIdx != 0 {
		t.Errorf("shift+tab should go back, fileIdx=%d", m.fileIdx)
	}
}

func TestSplitRowsStylePerSide(t *testing.T) {
	m := testModel(t)
	m = key(m, "t") // split
	rows := m.rows()
	for i := range rows {
		r := &rows[i]
		if r.Left == nil || r.Right == nil || r.Left.Kind != diff.LineDeletion || r.Right.Kind != diff.LineAddition {
			continue
		}
		// A paired row: each side must be rendered with its own kind's style,
		// which renderSplitStyled derives from the cells (not kindOf).
		line := m.renderSplitStyled(r, m.numWidth(), m.contentWidth()-2)
		if !strings.Contains(stripANSI(line), "│") {
			t.Fatalf("split row lost its separator: %q", line)
		}
		return
	}
	t.Skip("fixture has no paired deletion/addition row")
}

// TestThreadShareOneBoxOldestFirst: multiple comments on one line share one
// containing box, ordered by timestamp with inner dividers between items.
func TestThreadShareOneBoxOldestFirst(t *testing.T) {
	m := testModel(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	newer := m.draft.Add(loc, "newer note", snip)
	older := m.draft.Add(loc, "older note", snip)
	m.draft.Get(newer).At = "2026-07-30T10:00:00Z"
	m.draft.Get(older).At = "2026-07-01T10:00:00Z"

	rows := m.rows()
	tops, divs := 0, 0
	var texts []string
	for i := range rows {
		if !rows[i].Annotation {
			continue
		}
		switch rows[i].Edge {
		case diff.EdgeTop:
			tops++
		case diff.EdgeDivider:
			divs++
		case diff.EdgeNone:
			texts = append(texts, rows[i].Left.Text)
		}
	}
	if tops != 1 {
		t.Errorf("comments should share ONE box, got %d boxes", tops)
	}
	if divs != 1 {
		t.Errorf("two items should have one inner divider, got %d", divs)
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "older note") || !strings.Contains(joined, "newer note") {
		t.Fatalf("box missing items:\n%s", joined)
	}
	if strings.Index(joined, "older note") > strings.Index(joined, "newer note") {
		t.Errorf("thread not oldest-first:\n%s", joined)
	}
	if !strings.Contains(m.View(), "├") {
		t.Errorf("inner divider not rendered")
	}
}

// TestSplitDividerContinuesThroughBox: the split panel divider must not be
// interrupted by annotation rows — the box floats over the right panel while
// the two-pane geometry stays continuous.
func TestSplitDividerContinuesThroughBox(t *testing.T) {
	m := commentedModel(t)
	m = key(m, "t") // split
	div := 2 + m.numWidth() + 1 + m.splitPanelWidth() + 1
	for _, ln := range strings.Split(m.View(), "\n") {
		if !strings.Contains(ln, "╭") && !strings.Contains(ln, "╰") {
			continue
		}
		plain := stripANSI(ln)
		if len([]rune(plain)) <= div || []rune(plain)[div] != '│' {
			t.Errorf("split divider missing at col %d of box row: %q", div, plain)
		}
	}
}

// TestCommentImageFallbackTag: with images off, an image reference degrades
// to a visible tag — never to silence.
func TestCommentImageFallbackTag(t *testing.T) {
	m := testModel(t)
	m.images = ui.NewImageRenderer("off")
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	m.draft.Add(loc, "see ![screenshot](docs/shot.png) for the glitch", snip)
	// The tag names the image AND why it is not rendered — a bare tag never
	// teaches the user that installing chafa fixes it.
	if !strings.Contains(m.View(), "[image: docs/shot.png") || !strings.Contains(m.View(), "no image renderer") {
		t.Errorf("image tag with renderer hint missing from the box:\n%s", m.View())
	}
}

// TestCommentImageRendersPreRows: with a graphical backend, image rows enter
// the box as preformatted rows and remote URLs stay tags.
func TestCommentImageRendersPreRows(t *testing.T) {
	m := testModel(t)
	m.images = ui.NewImageRenderer("kitty")
	path := writeTestPNG(t)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, _ := m.buildLocation()
	m.draft.Add(loc, "local ![a]("+path+") and remote ![b](https://x.test/i.png)", snip)

	pre := 0
	for _, r := range m.rows() {
		if r.Annotation && r.Pre {
			pre++
		}
	}
	if pre == 0 {
		t.Fatalf("no preformatted image rows in the box")
	}
	if !strings.Contains(m.View(), "[image: https://x.test/i.png]") {
		t.Errorf("remote image must stay a tag (no network fetches)")
	}
}

func writeTestPNG(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	path := filepath.Join(t.TempDir(), "a.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}
