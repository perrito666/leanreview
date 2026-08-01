package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestBuiltinThemeVariants: default-light and default-dark pin the adaptive
// palette to one side; unknown names are not built-in.
func TestBuiltinThemeVariants(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	for _, name := range []string{"", "default", "default-light", "default-dark", "mono"} {
		if _, ok := BuiltinTheme(name); !ok {
			t.Errorf("%q should be built-in", name)
		}
	}
	if _, ok := BuiltinTheme("dusk"); ok {
		t.Errorf("user names must not be built-in")
	}
	light, _ := BuiltinTheme("default-light")
	dark, _ := BuiltinTheme("default-dark")
	if light.Addition.Render("x") == dark.Addition.Render("x") {
		t.Errorf("light and dark variants should differ on adaptive roles")
	}
}

// TestWithOverrides: a theme file restyles only the roles it names; unknown
// roles are named errors, not silent no-ops.
func TestWithOverrides(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	base := DefaultTheme()
	got, err := base.WithOverrides(map[string]StyleOverride{
		"addition": {FG: "#ff00ff"},
	})
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if got.Addition.Render("x") == base.Addition.Render("x") {
		t.Errorf("override did not change the role")
	}
	if got.Deletion.Render("x") != base.Deletion.Render("x") {
		t.Errorf("unnamed roles must keep the base style")
	}
	if _, err := base.WithOverrides(map[string]StyleOverride{"additon": {}}); err == nil {
		t.Errorf("unknown role must error")
	}
}

// TestThemeRolesCoverEveryOverride: the documented role list and the
// override table stay in sync.
func TestThemeRolesCoverEveryOverride(t *testing.T) {
	base := DefaultTheme()
	for _, role := range ThemeRoles() {
		if _, err := base.WithOverrides(map[string]StyleOverride{role: {FG: "1"}}); err != nil {
			t.Errorf("documented role %q is not overridable: %v", role, err)
		}
	}
}
