package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withConfig writes a config.json under a temp XDG_CONFIG_HOME and points the
// environment at it, clearing overriding env vars.
func withConfig(t *testing.T, json string) {
	t.Helper()
	dir := t.TempDir()
	if json != "" {
		cfgDir := filepath.Join(dir, "leanreview")
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(json), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("LEANREVIEW_EDITOR", "")
	t.Setenv("LEANREVIEW_SYNTAX", "")
	t.Setenv("NO_COLOR", "")
}

func TestDefaults(t *testing.T) {
	withConfig(t, "")
	c := Load()
	if !c.Syntax || c.SyntaxStyle != "auto" || c.Theme != "default" || c.TabWidth != 4 || c.Context != 3 {
		t.Errorf("defaults = %+v", c)
	}
}

func TestFileConfig(t *testing.T) {
	withConfig(t, `{"editor":"nvim -f","syntax":false,"syntax_style":"monokai","theme":"mono","tab_width":8,"context":5}`)
	c := Load()
	if c.Editor != "nvim -f" || c.Syntax || c.SyntaxStyle != "monokai" || c.Theme != "mono" || c.TabWidth != 8 || c.Context != 5 {
		t.Errorf("file config not applied: %+v", c)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	withConfig(t, `{"editor":"nvim","syntax":true}`)
	t.Setenv("LEANREVIEW_EDITOR", "code --wait")
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	c := Load()
	if c.Editor != "code --wait" {
		t.Errorf("env should override file editor, got %q", c.Editor)
	}
	if c.Syntax {
		t.Errorf("LEANREVIEW_SYNTAX=0 should disable syntax")
	}
}

func TestNoColorDisablesSyntax(t *testing.T) {
	withConfig(t, `{"syntax":true}`)
	t.Setenv("NO_COLOR", "1")
	if Load().Syntax {
		t.Errorf("NO_COLOR should disable syntax")
	}
}

func TestWrapConfig(t *testing.T) {
	withConfig(t, "")
	if c := Load(); !c.Wrap || c.WrapWidth != 120 {
		t.Errorf("wrap defaults = %v / %d, want true / 120", c.Wrap, c.WrapWidth)
	}
	withConfig(t, `{"wrap":false,"wrap_width":100}`)
	c := Load()
	if c.Wrap || c.WrapWidth != 100 {
		t.Errorf("wrap config not applied: %v / %d", c.Wrap, c.WrapWidth)
	}
}

func TestListConfig(t *testing.T) {
	withConfig(t, "")
	if c := Load(); c.ListEngine != "gh" || c.ListFilter != "" {
		t.Errorf("list defaults = %q / %q", c.ListEngine, c.ListFilter)
	}
	withConfig(t, `{"list_engine":"glab","list_filter":"state=opened&labels=bug"}`)
	c := Load()
	if c.ListEngine != "glab" || c.ListFilter != "state=opened&labels=bug" {
		t.Errorf("list config not applied: %q / %q", c.ListEngine, c.ListFilter)
	}

	withConfig(t, `{"list_filters":{"bugs":"is:open label:bug","docs":"label:documentation"}}`)
	c = Load()
	if c.ListFilters["bugs"] != "is:open label:bug" || c.ListFilters["docs"] != "label:documentation" {
		t.Errorf("named filters not parsed: %+v", c.ListFilters)
	}
}

func TestKeysConfig(t *testing.T) {
	withConfig(t, `{"keys":{"x":"down","j":""}}`)
	c := Load()
	if c.Keys["x"] != "down" || c.Keys["j"] != "" {
		t.Errorf("keys not parsed: %+v", c.Keys)
	}
}

func TestInvalidFileIgnored(t *testing.T) {
	withConfig(t, `{not valid json`)
	c := Load()
	if !c.Syntax || c.TabWidth != 4 {
		t.Errorf("invalid config should fall back to defaults: %+v", c)
	}
	// The typo must be surfaced, not silently swallowed — an ignored config
	// file is the failure users cannot diagnose otherwise.
	if c.Warning == "" || !strings.Contains(c.Warning, "malformed") {
		t.Errorf("expected a malformed-config warning, got %q", c.Warning)
	}
}

func TestValidFileHasNoWarning(t *testing.T) {
	withConfig(t, `{"tab_width": 8}`)
	if c := Load(); c.Warning != "" {
		t.Errorf("valid config should carry no warning, got %q", c.Warning)
	}
}
