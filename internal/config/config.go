// Package config holds user-facing configuration. In Milestone 1 this is
// intentionally tiny: the editor override and the log destination, sourced from
// the environment. A file-based config (review.editor, theme, keymaps) arrives
// with later milestones.
package config

import (
	"os"
	"path/filepath"
)

// Config is the resolved runtime configuration.
type Config struct {
	// Editor overrides editor auto-detection (highest precedence). Empty means
	// fall back to GIT_EDITOR/VISUAL/EDITOR/git.
	Editor string
	// LogPath is where diagnostic logs are written (never stdout, which the TUI
	// owns while active).
	LogPath string
}

// Load builds a Config from the environment.
func Load() Config {
	return Config{
		Editor:  os.Getenv("LEANREVIEW_EDITOR"),
		LogPath: logPath(),
	}
}

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
