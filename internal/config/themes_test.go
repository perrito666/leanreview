package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withThemes writes theme files under a temp XDG_CONFIG_HOME's themes folder.
func withThemes(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	td := filepath.Join(dir, "leanreview", "themes")
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(td, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestThemesLoadByMetadataName: the metadata name — not the filename — is
// the reference, so renaming a file never breaks a config.
func TestThemesLoadByMetadataName(t *testing.T) {
	withThemes(t, map[string]string{
		"whatever-filename.json": `{"name": "dusk", "styles": {"addition": {"fg": "114"}}}`,
	})
	tf, ok := FindTheme("dusk")
	if !ok {
		t.Fatalf("theme not found by metadata name")
	}
	if tf.Styles["addition"].FG != "114" {
		t.Errorf("styles not parsed: %+v", tf.Styles)
	}
	if _, ok := FindTheme("whatever-filename"); ok {
		t.Errorf("filename must not be a reference")
	}
}

// TestValidateThemes: reserved names, duplicates, unknown roles, and broken
// files are all reported; one broken file does not hide the good ones.
func TestValidateThemes(t *testing.T) {
	withThemes(t, map[string]string{
		"a.json":      `{"name": "mono", "styles": {}}`,
		"b.json":      `{"name": "dusk", "styles": {"additon": {"fg": "1"}}}`,
		"c.json":      `{"name": "dusk", "styles": {}}`,
		"broken.json": `{`,
		"noname.json": `{"styles": {}}`,
	})
	problems := ValidateThemes([]string{"default", "default-light", "default-dark", "mono"},
		[]string{"addition", "deletion"})
	for _, want := range []string{"reserved", "more than one file", `unknown style role "additon"`, "broken.json", "noname.json"} {
		found := false
		for _, p := range problems {
			if strings.Contains(p, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("problems %v missing %q", problems, want)
		}
	}
	if _, ok := FindTheme("dusk"); !ok {
		t.Errorf("valid themes must survive broken siblings")
	}
}
