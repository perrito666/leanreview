package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pngFixture writes a small PNG (wider than tall) and returns its path.
func pngFixture(t *testing.T) string {
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

func TestImagesOffRendersNothing(t *testing.T) {
	r := NewImageRenderer("off")
	if r.Enabled() {
		t.Errorf("off mode reports enabled")
	}
	if _, ok := r.Render(pngFixture(t), 40, 12); ok {
		t.Errorf("off mode rendered an image")
	}
}

func TestKittyPlaceholderStructure(t *testing.T) {
	r := NewImageRenderer("kitty")
	path := pngFixture(t)

	lines, ok := r.Render(path, 20, 12)
	if !ok || len(lines) == 0 {
		t.Fatalf("kitty render failed")
	}
	// First render carries the one-shot transmission with the protocol keys.
	first := lines[0]
	for _, want := range []string{"\x1b_G", "f=100", "U=1", "q=2", "a=T"} {
		if !strings.Contains(first, want) {
			t.Errorf("transmission missing %q", want)
		}
	}
	// Every row is placeholder cells addressed by row/col diacritics.
	for i, ln := range lines {
		if !strings.ContainsRune(ln, kittyPlaceholder) {
			t.Errorf("row %d has no placeholder cells", i)
		}
		if !strings.ContainsRune(ln, kittyDiacritics[i]) {
			t.Errorf("row %d missing its row diacritic", i)
		}
	}
	// A wide, short image must produce few rows (aspect respected).
	if len(lines) > 6 {
		t.Errorf("40x10 image rendered %d rows; aspect ignored", len(lines))
	}

	// Second render must NOT retransmit the payload — repaints would flood
	// the terminal otherwise.
	again, ok := r.Render(path, 20, 12)
	if !ok {
		t.Fatalf("second render failed")
	}
	if strings.Contains(again[0], "\x1b_G") {
		t.Errorf("payload retransmitted on repaint")
	}
	if len(again) != len(lines) {
		t.Errorf("row count changed between renders")
	}
}

func TestKittyRejectsMissingFile(t *testing.T) {
	r := NewImageRenderer("kitty")
	if _, ok := r.Render("/no/such/image.png", 20, 12); ok {
		t.Errorf("missing file rendered")
	}
}

func TestChafaRenderIfAvailable(t *testing.T) {
	if _, err := exec.LookPath("chafa"); err != nil {
		t.Skip("chafa not installed")
	}
	r := NewImageRenderer("chafa")
	lines, ok := r.Render(pngFixture(t), 20, 8)
	if !ok || len(lines) == 0 || len(lines) > 8 {
		t.Errorf("chafa render: ok=%v rows=%d", ok, len(lines))
	}
}
