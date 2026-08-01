package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ThemeSchemaURL is the published JSON Schema for theme files; a "$schema"
// key referencing it gives editor validation, and is ignored here.
const ThemeSchemaURL = "https://perrito666.github.io/leanreview/schema/leanreview-theme.schema.json"

// ThemeFile is one theme document from the themes directory. The metadata
// name — not the filename — is how configuration references the theme, so
// renaming a file never breaks a config.
type ThemeFile struct {
	Schema      string                `json:"$schema,omitempty"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Author      string                `json:"author,omitempty"`
	Styles      map[string]ThemeStyle `json:"styles"`
}

// ThemeStyle restyles one role. Colors are terminal colors (ANSI index or
// #hex); attribute pointers distinguish unset from explicit false.
type ThemeStyle struct {
	FG        string `json:"fg,omitempty"`
	BG        string `json:"bg,omitempty"`
	Bold      *bool  `json:"bold,omitempty"`
	Underline *bool  `json:"underline,omitempty"`
	Reverse   *bool  `json:"reverse,omitempty"`
}

// ThemesDir is the theme-file location: a "themes" folder next to the
// config file.
func ThemesDir() string {
	p := configPath()
	if p == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(p), "themes")
}

// LoadThemes reads every *.json in the themes directory. Problems (unreadable
// or malformed files, missing names) are returned alongside the readable
// themes rather than aborting: one broken file must not take down the good
// ones, and --check-config reports the problem list verbatim.
func LoadThemes() ([]ThemeFile, []string) {
	dir := ThemesDir()
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil // no themes folder is the common, healthy case
	}
	var themes []ThemeFile
	var problems []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("themes/%s: %v", e.Name(), err))
			continue
		}
		var tf ThemeFile
		if err := json.Unmarshal(data, &tf); err != nil {
			problems = append(problems, fmt.Sprintf("themes/%s: %v", e.Name(), err))
			continue
		}
		if strings.TrimSpace(tf.Name) == "" {
			problems = append(problems, fmt.Sprintf("themes/%s: missing \"name\" — the name is how the config references the theme", e.Name()))
			continue
		}
		themes = append(themes, tf)
	}
	return themes, problems
}

// FindTheme returns the theme file whose metadata name matches, if any.
func FindTheme(name string) (ThemeFile, bool) {
	themes, _ := LoadThemes()
	for _, tf := range themes {
		if tf.Name == name {
			return tf, true
		}
	}
	return ThemeFile{}, false
}

// ValidateThemes reports problems across the themes directory: broken files,
// reserved (built-in) names, duplicate names, and unknown style roles.
// reservedNames and knownRoles are injected by the caller — the ui package
// owns both inventories.
func ValidateThemes(reservedNames, knownRoles []string) []string {
	themes, problems := LoadThemes()
	reserved := map[string]bool{}
	for _, n := range reservedNames {
		reserved[n] = true
	}
	roles := map[string]bool{}
	for _, r := range knownRoles {
		roles[r] = true
	}
	seen := map[string]bool{}
	for _, tf := range themes {
		if reserved[tf.Name] {
			problems = append(problems, fmt.Sprintf("theme %q: the built-in theme names (%s) are reserved", tf.Name, strings.Join(reservedNames, ", ")))
		}
		if seen[tf.Name] {
			problems = append(problems, fmt.Sprintf("theme %q is defined by more than one file", tf.Name))
		}
		seen[tf.Name] = true
		for role := range tf.Styles {
			if !roles[role] {
				problems = append(problems, fmt.Sprintf("theme %q: unknown style role %q (known: %s)", tf.Name, role, strings.Join(knownRoles, ", ")))
			}
		}
	}
	return problems
}
