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
		Addition: lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		Deletion: lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		Context:  lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		Metadata: lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		Gutter:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Cursor:   lipgloss.NewStyle().Reverse(true),
		Select:   lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")),
		Search:   lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0")),
		Marker:   lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
		Title:    lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")).Bold(true),
		Status:   lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		Error:    lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		Key:      lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		Faint:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
}

func monoTheme() Theme {
	plain := lipgloss.NewStyle()
	bold := lipgloss.NewStyle().Bold(true)
	return Theme{
		Addition: plain,
		Deletion: plain,
		Context:  plain,
		Metadata: bold,
		Gutter:   plain,
		Cursor:   lipgloss.NewStyle().Reverse(true),
		Select:   lipgloss.NewStyle().Reverse(true),
		Search:   lipgloss.NewStyle().Underline(true).Bold(true),
		Marker:   bold,
		Title:    bold,
		Status:   plain,
		Error:    bold,
		Key:      bold,
		Faint:    plain,
	}
}
