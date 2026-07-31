// Package config holds user-facing configuration, resolved from (in increasing
// precedence) built-in defaults, a JSON config file, and environment variables.
// Command-line flags, handled in main, take precedence over all of these.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the resolved runtime configuration.
type Config struct {
	// Editor overrides editor auto-detection (highest precedence within the
	// editor resolution chain). Empty means fall back to GIT_EDITOR/VISUAL/…
	Editor string
	// Syntax enables source syntax highlighting (subject to NO_COLOR).
	Syntax bool
	// SyntaxStyle is the Chroma style name (e.g. "github", "monokai"), or
	// "auto" to pick one matching the terminal background.
	SyntaxStyle string
	// Theme is the TUI palette name ("default" or "mono").
	Theme string
	// TabWidth is how many columns a tab expands to.
	TabWidth int
	// Context is the default number of unified context lines when -U is unset.
	Context int
	// Keys overrides individual normal-mode key bindings (key -> action).
	Keys map[string]string
	// ListEngine is the default discovery engine for --list ("gh" or "glab").
	ListEngine string
	// ListFilter is the fallback discovery filter for --list when no filter is
	// supplied; empty applies the engine's built-in default.
	ListFilter string
	// ListFilters are named discovery filters, selectable as --list :name or
	// --list engine:name. Filter text is engine-specific syntax.
	ListFilters map[string]string
	// Wrap enables wrapping of long diff lines and comment previews.
	Wrap bool
	// WrapWidth caps the wrap point in the unified layout (split layouts wrap
	// at the side panel width).
	WrapWidth int
	// Images selects comment-image rendering: "auto" (kitty protocol on
	// kitty/ghostty, chafa when installed, off otherwise), "kitty", "chafa",
	// or "off". Detection is heuristic; the setting is the escape hatch.
	Images string
	// Author attributes this reviewer's replies in review-exchange
	// conversations. Defaults to $USER; the config file and
	// LEANREVIEW_AUTHOR override it.
	Author string
	// LogPath is where diagnostic logs are written (never stdout).
	LogPath string
	// Warning carries a non-fatal startup problem (e.g. a malformed config
	// file that was ignored) for main to surface; the config itself falls
	// back to defaults so startup never aborts over it.
	Warning string
}

// fileConfig mirrors the on-disk JSON, using pointers so absent keys are
// distinguishable from zero values.
type fileConfig struct {
	Editor      *string           `json:"editor"`
	Syntax      *bool             `json:"syntax"`
	SyntaxStyle *string           `json:"syntax_style"`
	Theme       *string           `json:"theme"`
	TabWidth    *int              `json:"tab_width"`
	Context     *int              `json:"context"`
	Keys        map[string]string `json:"keys"`
	ListEngine  *string           `json:"list_engine"`
	ListFilter  *string           `json:"list_filter"`
	ListFilters map[string]string `json:"list_filters"`
	Wrap        *bool             `json:"wrap"`
	WrapWidth   *int              `json:"wrap_width"`
	Author      *string           `json:"author"`
	Images      *string           `json:"images"`
}

// Load builds a Config from defaults, then the config file, then the environment.
func Load() Config {
	c := Config{
		Syntax:      true,
		SyntaxStyle: "auto",
		Theme:       "default",
		TabWidth:    4,
		Context:     3,
		ListEngine:  "gh",
		Wrap:        true,
		WrapWidth:   120,
		Author:      os.Getenv("USER"),
		Images:      "auto",
		LogPath:     logPath(),
	}

	fc, ok, warn := readFile(configPath())
	c.Warning = warn
	if ok {
		if fc.Editor != nil {
			c.Editor = *fc.Editor
		}
		if fc.Syntax != nil {
			c.Syntax = *fc.Syntax
		}
		if fc.SyntaxStyle != nil {
			c.SyntaxStyle = *fc.SyntaxStyle
		}
		if fc.Theme != nil {
			c.Theme = *fc.Theme
		}
		if fc.TabWidth != nil && *fc.TabWidth > 0 {
			c.TabWidth = *fc.TabWidth
		}
		if fc.Context != nil && *fc.Context >= 0 {
			c.Context = *fc.Context
		}
		if fc.Keys != nil {
			c.Keys = fc.Keys
		}
		if fc.ListEngine != nil && *fc.ListEngine != "" {
			c.ListEngine = *fc.ListEngine
		}
		if fc.ListFilter != nil {
			c.ListFilter = *fc.ListFilter
		}
		if fc.ListFilters != nil {
			c.ListFilters = fc.ListFilters
		}
		if fc.Wrap != nil {
			c.Wrap = *fc.Wrap
		}
		if fc.WrapWidth != nil && *fc.WrapWidth > 0 {
			c.WrapWidth = *fc.WrapWidth
		}
		if fc.Author != nil && *fc.Author != "" {
			c.Author = *fc.Author
		}
		if fc.Images != nil && *fc.Images != "" {
			c.Images = *fc.Images
		}
	}

	// Environment overrides the file.
	if v := os.Getenv("LEANREVIEW_EDITOR"); v != "" {
		c.Editor = v
	}
	if os.Getenv("LEANREVIEW_SYNTAX") == "0" || os.Getenv("NO_COLOR") != "" {
		c.Syntax = false
	}
	if v := os.Getenv("LEANREVIEW_AUTHOR"); v != "" {
		c.Author = v
	}
	if v := os.Getenv("LEANREVIEW_IMAGES"); v != "" {
		c.Images = v
	}
	// LEANREVIEW_LOG is already honoured by logPath(); no second read needed.

	return c
}

// configPath locates the config file per the XDG base-directory spec:
// $XDG_CONFIG_HOME/leanreview/config.json, defaulting to ~/.config. An empty
// return (no resolvable home) makes readFile treat the file as absent, so a
// homeless environment still starts with built-in defaults.
func configPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "leanreview", "config.json")
}

// readFile parses the JSON config at path. The config file is optional, so an
// absent or unreadable file reports ok=false and the caller proceeds with
// defaults. A file that exists but does not parse also reports ok=false, but
// returns a warning: a typo silently reverting every setting to defaults is
// exactly the failure users cannot diagnose without a message.
func readFile(path string) (fc fileConfig, ok bool, warn string) {
	if path == "" {
		return fileConfig{}, false, ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, false, ""
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		return fileConfig{}, false, fmt.Sprintf("config file %s ignored (malformed): %v", path, err)
	}
	return fc, true, ""
}

// logPath picks the default diagnostic log location: LEANREVIEW_LOG if set,
// else $XDG_STATE_HOME/leanreview/leanreview.log (state, not config, since
// logs are machine-generated data), defaulting to ~/.local/state and falling
// back to the OS temp dir when no home directory is resolvable — logging must
// always have somewhere to go because the TUI cannot use stderr.
func logPath() string {
	if p := os.Getenv("LEANREVIEW_LOG"); p != "" {
		return p
	}
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "leanreview.log")
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "leanreview", "leanreview.log")
}
