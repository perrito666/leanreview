package app

import (
	"maps"
	"slices"
)

// NamedKeymap is a user-defined keymap from configuration: overrides layered
// on top of the defaults, selectable by name via the keymap setting.
type NamedKeymap struct {
	Keys      map[string]string
	Sequences []SeqBinding
}

// presetOverlays are the built-in presets' deltas over the default bindings.
// Presets layer familiar chords on top of the defaults rather than replacing
// them wholesale: terminals only report a subset of each editor's real
// chords (no ctrl+shift combinations, no cmd), so a faithful clone is
// impossible and a harmless superset is more useful than a lossy port.
var presetOverlays = map[string]map[string]string{
	"default": {},
	// Pure Vim: the defaults are already Vim-flavored; this adds the scroll
	// chords Vim users reach for.
	"vim": {
		"ctrl+f": "page-down",
		"ctrl+b": "page-up",
		"ctrl+e": "down",
		"ctrl+y": "up",
	},
	// VS Code: search on ctrl+f/F3, quick-open on ctrl+p, home/end jumps.
	"vscode": {
		"ctrl+f":   "search",
		"f3":       "next-match",
		"shift+f3": "prev-match",
		"ctrl+p":   "files",
		"home":     "first-line",
		"end":      "last-line",
	},
	// Sublime Text: same search cluster, goto-anything on ctrl+p.
	"sublime": {
		"ctrl+f":   "search",
		"f3":       "next-match",
		"shift+f3": "prev-match",
		"ctrl+p":   "files",
		"home":     "first-line",
		"end":      "last-line",
	},
	// IntelliJ family: search on ctrl+f/F3, recent-files on ctrl+e.
	"intellij": {
		"ctrl+f":   "search",
		"f3":       "next-match",
		"shift+f3": "prev-match",
		"ctrl+e":   "files",
		"home":     "first-line",
		"end":      "last-line",
	},
}

// BuiltinKeymapNames returns the built-in preset names, sorted. These names
// are reserved: a user-defined keymap may not shadow them.
func BuiltinKeymapNames() []string {
	return slices.Sorted(maps.Keys(presetOverlays))
}

// IsBuiltinKeymap reports whether name is a built-in preset.
func IsBuiltinKeymap(name string) bool {
	_, ok := presetOverlays[name]
	return ok
}

// resolveKeymap builds the effective binding tables: the named base first
// (a built-in preset, or a user-defined keymap layered on the defaults),
// then the top-level keys/sequences overrides — so a config can pick
// "vscode" and still retune individual keys. Unknown names fall back to the
// defaults; --check-config names the mistake.
func resolveKeymap(name string, user map[string]NamedKeymap, keys map[string]string, seqs []SeqBinding) (Keymap, Sequences) {
	km := DefaultKeymap()
	sq := DefaultSequences()
	if overlay, ok := presetOverlays[name]; ok {
		km.apply(overlay)
	} else if nk, ok := user[name]; ok {
		km.apply(nk.Keys)
		sq.apply(nk.Sequences)
	}
	km.apply(keys)
	sq.apply(seqs)
	return km, sq
}

// ValidateKeymapChoice reports problems with the keymap selection and the
// user-defined keymap table: an unknown name (silently falling back to the
// defaults would be the classic invisible typo), and user keymaps shadowing
// reserved built-in names.
func ValidateKeymapChoice(name string, user map[string]NamedKeymap) []string {
	var problems []string
	for _, n := range slices.Sorted(maps.Keys(user)) {
		if IsBuiltinKeymap(n) {
			problems = append(problems, "keymaps["+n+"]: shadows a built-in preset name (reserved: "+joinNames()+")")
		}
	}
	if name == "" || IsBuiltinKeymap(name) {
		return problems
	}
	if _, ok := user[name]; !ok {
		problems = append(problems, "keymap "+name+" is neither a built-in preset ("+joinNames()+") nor defined in keymaps")
	}
	return problems
}

func joinNames() string {
	out := ""
	for i, n := range BuiltinKeymapNames() {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
