package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// StyleOverride restyles one theme role from a theme file. Colors are any
// lipgloss-accepted terminal color: an ANSI palette index ("114") or hex
// ("#87d787"). Attribute pointers distinguish "unset" (keep the base) from
// an explicit false (clear the base's attribute).
type StyleOverride struct {
	FG        string
	BG        string
	Bold      *bool
	Underline *bool
	Reverse   *bool
}

// ThemeRoles are the restylable role names, in the order they are documented.
// The validator uses this list so a typoed role in a theme file is named
// instead of silently ignored.
func ThemeRoles() []string {
	return []string{
		"addition", "deletion", "context", "metadata",
		"gutter", "cursor", "select", "search", "marker",
		"title", "status", "error", "key", "faint", "comment",
		"addition_tint", "deletion_tint",
	}
}

// WithOverrides returns a copy of the theme with the given role overrides
// applied on top — a theme file restyles only the roles it names, so a
// four-line theme is a valid theme. Unknown roles error (the caller surfaces
// them; silently ignoring a typo would make the theme look broken instead).
func (t Theme) WithOverrides(overrides map[string]StyleOverride) (Theme, error) {
	roles := map[string]*lipgloss.Style{
		"addition": &t.Addition, "deletion": &t.Deletion, "context": &t.Context,
		"metadata": &t.Metadata, "gutter": &t.Gutter, "cursor": &t.Cursor,
		"select": &t.Select, "search": &t.Search, "marker": &t.Marker,
		"title": &t.Title, "status": &t.Status, "error": &t.Error,
		"key": &t.Key, "faint": &t.Faint, "comment": &t.Comment,
		"addition_tint": &t.AdditionTint, "deletion_tint": &t.DeletionTint,
	}
	for role, o := range overrides {
		s, ok := roles[role]
		if !ok {
			return t, fmt.Errorf("unknown style role %q", role)
		}
		st := *s
		if o.FG != "" {
			st = st.Foreground(lipgloss.Color(o.FG))
		}
		if o.BG != "" {
			st = st.Background(lipgloss.Color(o.BG))
		}
		if o.Bold != nil {
			st = st.Bold(*o.Bold)
		}
		if o.Underline != nil {
			st = st.Underline(*o.Underline)
		}
		if o.Reverse != nil {
			st = st.Reverse(*o.Reverse)
		}
		*s = st
	}
	return t, nil
}
