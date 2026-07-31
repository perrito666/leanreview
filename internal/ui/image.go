package ui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"strings"
)

// ImageMode selects how comment images are rendered.
type ImageMode uint8

const (
	// ImagesOff renders a textual tag only.
	ImagesOff ImageMode = iota
	// ImagesChafa shells out to chafa for ANSI cell art — works in any
	// terminal and scrolls like ordinary text.
	ImagesChafa
	// ImagesKitty uses the kitty graphics protocol with Unicode placeholders,
	// the only kitty mode whose images scroll with the surrounding rows.
	ImagesKitty
)

// ImageRenderer renders image files into terminal rows, memoizing per
// (path, size): rendering happens inside the draw path, and re-running chafa
// or re-encoding a PNG on every keystroke would be absurd.
type ImageRenderer struct {
	mode   ImageMode
	cache  map[string][]string
	nextID uint32
	// pending holds each image's one-shot transmission escape (by cache
	// key), consumed by the first Render that returns its rows.
	pendingTx map[string]string
}

// NewImageRenderer builds a renderer for the configured mode: "kitty",
// "chafa", "off", or "auto" — auto prefers the kitty protocol on terminals
// that speak it (kitty, ghostty), then chafa when installed, then off.
// Detection is heuristic, which is exactly why the config override exists.
func NewImageRenderer(mode string) *ImageRenderer {
	r := &ImageRenderer{cache: map[string][]string{}, pendingTx: map[string]string{}, nextID: 1}
	switch strings.ToLower(mode) {
	case "kitty":
		r.mode = ImagesKitty
	case "chafa":
		r.mode = ImagesChafa
	case "off":
		r.mode = ImagesOff
	default: // auto
		term := os.Getenv("TERM")
		if os.Getenv("KITTY_WINDOW_ID") != "" || strings.Contains(term, "kitty") || strings.Contains(term, "ghostty") {
			r.mode = ImagesKitty
		} else if _, err := exec.LookPath("chafa"); err == nil {
			r.mode = ImagesChafa
		}
	}
	return r
}

// Enabled reports whether any graphical rendering is active.
func (r *ImageRenderer) Enabled() bool { return r.mode != ImagesOff }

// Render returns terminal rows displaying the image at path within the given
// cell budget, or ok=false when the file cannot be rendered (missing,
// undecodable, mode off) — the caller falls back to a textual tag.
func (r *ImageRenderer) Render(path string, maxCols, maxRows int) ([]string, bool) {
	if r.mode == ImagesOff || maxCols < 4 || maxRows < 2 {
		return nil, false
	}
	key := fmt.Sprintf("%s|%d|%d|%d", path, maxCols, maxRows, r.mode)
	lines, ok := r.cache[key]
	if !ok {
		switch r.mode {
		case ImagesChafa:
			lines = renderChafa(path, maxCols, maxRows)
		case ImagesKitty:
			var tx string
			lines, tx = r.renderKitty(path, maxCols, maxRows)
			if tx != "" {
				r.pendingTx[key] = tx
			}
		}
		r.cache[key] = lines
	}
	if len(lines) == 0 {
		return nil, false
	}
	return lines, true
}

// TakeTransmissions returns (and clears) the pending kitty payload
// transmissions. It must be called from the code path whose output is
// guaranteed to reach the terminal — the top of View — never from row
// construction: rows are also built during update processing (cursor
// clamping, layout math) where the strings are discarded, and a payload
// emitted there dies unseen, leaving images invisible until an unrelated
// width change forces a re-render.
func (r *ImageRenderer) TakeTransmissions() string {
	if len(r.pendingTx) == 0 {
		return ""
	}
	var b strings.Builder
	for key, tx := range r.pendingTx {
		b.WriteString(tx)
		delete(r.pendingTx, key)
	}
	return b.String()
}

// renderChafa shells out for symbol art. chafa's output rows are plain ANSI
// text, so they clip, scroll, and repaint like any other row.
func renderChafa(path string, cols, rows int) []string {
	// chafa detects color support from the tty; run through a pipe (as here,
	// always) it would emit colorless spaces — invisible art. Force truecolor.
	out, err := exec.Command("chafa", "--format", "symbols", "--colors", "full", "--size", fmt.Sprintf("%dx%d", cols, rows), path).Output()
	if err != nil {
		return nil
	}
	s := strings.TrimRight(string(out), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// kittyDiacritics are the leading entries of kitty's row/column diacritic
// table; entry N marks placeholder row/column N+1. Only this prefix is
// embedded, capping images at that many cells per axis — plenty for a
// comment thread, and small enough to audit.
var kittyDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
	0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
	0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F, 0x0483, 0x0484,
	0x0485, 0x0486, 0x0487, 0x0592,
}

// kittyPlaceholder is U+10EEEE, the codepoint the kitty protocol reserves for
// Unicode image placeholders.
const kittyPlaceholder = '\U0010EEEE'

// renderKitty transmits the image (PNG, base64, chunked) and returns rows of
// placeholder cells addressed by diacritics; the terminal paints the image
// over them, and because they are ordinary cells the image scrolls with the
// view. The transmission escape rides on the first row the first time only —
// re-sending the payload on every repaint would flood the terminal.
func (r *ImageRenderer) renderKitty(path string, maxCols, maxRows int) ([]string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width == 0 {
		return nil, ""
	}
	if format != "png" {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, ""
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, ""
		}
		data = buf.Bytes()
	}

	// Fit the cell grid to the image's aspect; terminal cells are ~2x taller
	// than wide, hence the 0.5 factor.
	limit := len(kittyDiacritics)
	cols := min3(maxCols, limit, 60)
	rows := int(float64(cols) * (float64(cfg.Height) / float64(cfg.Width)) * 0.5)
	if rows < 1 {
		rows = 1
	}
	if rows > maxRows {
		cols = cols * maxRows / rows
		rows = maxRows
	}
	if rows > limit {
		rows = limit
	}
	if cols < 1 {
		return nil, ""
	}

	id := r.nextID
	r.nextID++

	// The image id travels in the foreground color of the placeholder cells.
	// The cached rows are placeholder-only: the payload transmission rides on
	// the first row of the FIRST render only (see Render), because re-sending
	// a base64 image on every repaint would flood the terminal.
	fg := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", (id>>16)&0xff, (id>>8)&0xff, id&0xff)
	lines := make([]string, rows)
	for row := 0; row < rows; row++ {
		var b strings.Builder
		b.WriteString(fg)
		for col := 0; col < cols; col++ {
			b.WriteRune(kittyPlaceholder)
			b.WriteRune(kittyDiacritics[row])
			b.WriteRune(kittyDiacritics[col])
		}
		b.WriteString("\x1b[39m")
		lines[row] = b.String()
	}
	return lines, kittyTransmit(id, cols, rows, data)
}

// kittyTransmit builds the chunked graphics-transmission escape: PNG data
// (f=100), direct payload (t=d), Unicode placeholder mode (U=1), quiet (q=2)
// so unsupporting terminals stay silent, sized to the placeholder grid.
func kittyTransmit(id uint32, cols, rows int, png []byte) string {
	payload := base64.StdEncoding.EncodeToString(png)
	var b strings.Builder
	first := true
	for len(payload) > 0 {
		chunk := payload
		if len(chunk) > 4096 {
			chunk = chunk[:4096]
		}
		payload = payload[len(chunk):]
		more := 1
		if len(payload) == 0 {
			more = 0
		}
		if first {
			fmt.Fprintf(&b, "\x1b_Gf=100,t=d,a=T,U=1,q=2,i=%d,c=%d,r=%d,m=%d;%s\x1b\\", id, cols, rows, more, chunk)
			first = false
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}
	return b.String()
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
