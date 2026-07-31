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

// ThemeByName returns a named theme. "mono" forces the monochrome palette;
// anything else (including "" and "default") returns the standard palette,
// which itself collapses to monochrome under NO_COLOR.
func ThemeByName(name string) Theme {
	if name == "mono" {
		return monoTheme()
	}
	return DefaultTheme()
}

// DefaultTheme returns the standard palette, or a monochrome variant when
// NO_COLOR is set.
func DefaultTheme() Theme {
	if os.Getenv("NO_COLOR") != "" {
		return monoTheme()
	}
	return Theme{
		// A true 256-color green: the ANSI palette slot 2 renders olive/mustard
		// in many terminal schemes.
		Addition:     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "114"}),
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
		Comment:      lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "252"}),
		AdditionTint: lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "194", Dark: "22"}),
		DeletionTint: lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "224", Dark: "52"}),
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
