package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/ui"
)

// appPNGFixture writes a small PNG and returns its path.
func appPNGFixture(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 10))
	for x := 0; x < 40; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 6), G: 128, B: 64, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "shot.png")
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

// TestKittyTransmissionSurvivesToView: rows are built both while painting
// and during update processing (cursor clamping, layout math), where their
// strings are discarded. The kitty payload therefore rides on View's output
// — the only string guaranteed to reach the terminal — and exactly once.
// Regression: images stayed invisible until an unrelated width change,
// because the one-shot payload had been "shown" by a discarded row build.
func TestKittyTransmissionSurvivesToView(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 100, 30
	m.wrapText = true
	m.images = ui.NewImageRenderer("kitty")
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatalf("buildLocation: %v", err)
	}
	m.draft.Add(loc, "shot:\n![shot]("+appPNGFixture(t)+")", snip)

	// Simulate an update-path row build before any paint.
	m.rows()

	out := m.View()
	if !strings.Contains(out, "\x1b_G") {
		t.Fatalf("View output missing the kitty payload transmission")
	}
	if strings.Contains(m.View(), "\x1b_G") {
		t.Errorf("payload retransmitted on the second paint")
	}
}
