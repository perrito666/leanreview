package ui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// ContentLines highlights a whole file and returns one styled string per
// source line. Tokenizing the full content (rather than line by line) is what
// makes multi-line constructs — block comments, raw strings, heredocs —
// color correctly: the lexer keeps its state across lines.
//
// Each returned line is self-contained (token styles are re-emitted after
// every newline) and uses foreground-only SGR codes terminated by attribute
// clears, never a full reset — so a caller may wrap a whole line in a
// background tint and the tint survives the embedded styling. Returns nil
// when highlighting is disabled or fails; callers fall back to plain text.
func (h *Highlighter) ContentLines(path string, content []byte) []string {
	if !h.Enabled() || len(content) == 0 {
		return nil
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	it, err := lexer.Tokenise(nil, string(content))
	if err != nil {
		return nil
	}

	sgrCache := map[chroma.TokenType]string{}
	sgrFor := func(t chroma.TokenType) string {
		if s, ok := sgrCache[t]; ok {
			return s
		}
		s := sgrForEntry(h.style.Get(t))
		sgrCache[t] = s
		return s
	}

	var lines []string
	var b strings.Builder
	for tok := it(); tok != chroma.EOF; tok = it() {
		sgr := sgrFor(tok.Type)
		parts := strings.Split(tok.Value, "\n")
		for i, p := range parts {
			if i > 0 {
				lines = append(lines, b.String())
				b.Reset()
			}
			if p == "" {
				continue
			}
			if sgr == "" {
				b.WriteString(p)
				continue
			}
			b.WriteString(sgr)
			b.WriteString(p)
			b.WriteString(sgrOff)
		}
	}
	lines = append(lines, b.String())
	return lines
}

// sgrOff clears foreground color and text attributes without touching the
// background — the whole point of the fg-only discipline.
const sgrOff = "\x1b[39;22;23;24m"

// sgrForEntry translates a chroma style entry into a foreground-only SGR
// sequence ("" for unstyled). Colors are quantized to the xterm-256 palette
// for parity with the per-line formatter and terminals without truecolor.
func sgrForEntry(e chroma.StyleEntry) string {
	var attrs []string
	if e.Bold == chroma.Yes {
		attrs = append(attrs, "1")
	}
	if e.Italic == chroma.Yes {
		attrs = append(attrs, "3")
	}
	if e.Underline == chroma.Yes {
		attrs = append(attrs, "4")
	}
	if e.Colour.IsSet() {
		attrs = append(attrs, fmt.Sprintf("38;5;%d", rgbTo256(e.Colour.Red(), e.Colour.Green(), e.Colour.Blue())))
	}
	if len(attrs) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(attrs, ";") + "m"
}

// rgbTo256 maps an RGB color onto the xterm-256 palette: the 6x6x6 color cube
// or, for near-grays, the 24-step grayscale ramp — whichever is closer.
func rgbTo256(r, g, b uint8) int {
	cube := func(v uint8) int {
		if v < 48 {
			return 0
		}
		if v < 115 {
			return 1
		}
		return int(v-35) / 40
	}
	cr, cg, cb := cube(r), cube(g), cube(b)
	cubeIdx := 16 + 36*cr + 6*cg + cb
	cubeVal := func(c int) int {
		if c == 0 {
			return 0
		}
		return 55 + c*40
	}
	cubeDist := sqDiff(int(r), cubeVal(cr)) + sqDiff(int(g), cubeVal(cg)) + sqDiff(int(b), cubeVal(cb))

	gray := (int(r) + int(g) + int(b)) / 3
	gi := (gray - 3) / 10
	if gi < 0 {
		gi = 0
	}
	if gi > 23 {
		gi = 23
	}
	gv := 8 + gi*10
	grayDist := sqDiff(int(r), gv) + sqDiff(int(g), gv) + sqDiff(int(b), gv)
	if grayDist < cubeDist {
		return 232 + gi
	}
	return cubeIdx
}

// sqDiff is the squared channel difference — summed across R/G/B it gives
// the squared Euclidean distance used to pick the nearest palette color.
func sqDiff(a, b int) int {
	d := a - b
	return d * d
}
