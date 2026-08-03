// Package ui holds presentation helpers for the review client: the color theme
// and static text (help, overlays). Rendering logic that needs model state
// lives in the app package; this package stays free of app dependencies.
package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Theme is the set of styles used to render the diff and chrome. When NO_COLOR
// is set (or the terminal has no color), the palette collapses to attributes
// only (bold/reverse), keeping the UI usable.
type Theme struct {
	Addition lipgloss.Style
	Deletion lipgloss.Style
	Context  lipgloss.Style
	Metadata lipgloss.Style

	Gutter lipgloss.Style
	Cursor lipgloss.Style
	Select lipgloss.Style
	Search lipgloss.Style
	Marker lipgloss.Style

	Title  lipgloss.Style
	Status lipgloss.Style
	Error  lipgloss.Style
	Key    lipgloss.Style
	Faint  lipgloss.Style

	// Comment styles inline comment/thread preview text; brighter than Faint
	// so review discussion stays readable.
	Comment lipgloss.Style

	// AdditionTint/DeletionTint are faint background washes carrying diff
	// identity under syntax-colored changed lines (change_colors: syntax).
	// Background-only so the syntax foregrounds show through.
	AdditionTint lipgloss.Style
	DeletionTint lipgloss.Style
}

// colorPick resolves a light/dark color pair: adaptively (terminal
// background decides at render time), or pinned to one variant — the
// difference between the "default" theme and "default-light"/"default-dark".
type colorPick func(light, dark string) lipgloss.TerminalColor

func adaptivePick(l, d string) lipgloss.TerminalColor {
	return lipgloss.AdaptiveColor{Light: l, Dark: d}
}
func lightPick(l, _ string) lipgloss.TerminalColor { return lipgloss.Color(l) }
func darkPick(_, d string) lipgloss.TerminalColor  { return lipgloss.Color(d) }

// BuiltinThemeNames are the reserved theme names: user theme files may not
// claim them.
func BuiltinThemeNames() []string {
	return []string{"default", "default-dark", "default-light", "mono"}
}

// BuiltinTheme returns a built-in theme by name ("" means default) and
// whether the name was recognised.
func BuiltinTheme(name string) (Theme, bool) {
	switch name {
	case "", "default":
		return DefaultTheme(), true
	case "default-light":
		return paletteTheme(lightPick), true
	case "default-dark":
		return paletteTheme(darkPick), true
	case "mono":
		return monoTheme(), true
	}
	return Theme{}, false
}

// DefaultTheme returns the standard palette (terminal background picks the
// light or dark colors), or a monochrome variant when NO_COLOR is set.
func DefaultTheme() Theme {
	if os.Getenv("NO_COLOR") != "" {
		return monoTheme()
	}
	return paletteTheme(adaptivePick)
}

// paletteTheme is the standard palette with the light/dark choice injected,
// so default, default-light, and default-dark share one table.
func paletteTheme(c colorPick) Theme {
	return Theme{
		// A true 256-color green: the ANSI palette slot 2 renders olive/mustard
		// in many terminal schemes.
		Addition:     lipgloss.NewStyle().Foreground(c("28", "114")),
		Deletion:     lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		Context:      lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		Metadata:     lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		Gutter:       lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Cursor:       lipgloss.NewStyle().Reverse(true),
		Select:       lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")),
		Search:       lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0")),
		Marker:       lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
		Title:        lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")).Bold(true),
		Status:       lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		Error:        lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		Key:          lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		Faint:        lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Comment:      lipgloss.NewStyle().Foreground(c("240", "252")),
		AdditionTint: lipgloss.NewStyle().Background(c("194", "22")),
		DeletionTint: lipgloss.NewStyle().Background(c("224", "52")),
	}
}

// monoTheme is the palette with all color removed: every style collapses to
// plain text or an attribute (bold, reverse, underline) so the UI stays
// legible on monochrome terminals and honours NO_COLOR. Cursor/Select keep
// reverse video and Search gains an underline, since attributes are the only
// remaining way to distinguish them.
func monoTheme() Theme {
	plain := lipgloss.NewStyle()
	bold := lipgloss.NewStyle().Bold(true)
	return Theme{
		Addition:     plain,
		Deletion:     plain,
		Context:      plain,
		Metadata:     bold,
		Gutter:       plain,
		Cursor:       lipgloss.NewStyle().Reverse(true),
		Select:       lipgloss.NewStyle().Reverse(true),
		Search:       lipgloss.NewStyle().Underline(true).Bold(true),
		Marker:       bold,
		Title:        bold,
		Status:       plain,
		Error:        bold,
		Key:          bold,
		Faint:        plain,
		Comment:      plain,
		AdditionTint: plain,
		DeletionTint: plain,
	}
}
