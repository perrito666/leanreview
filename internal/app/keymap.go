package app

// Keymap maps a key (as reported by Bubble Tea's KeyMsg.String) to a normal-mode
// action name. Single-key normal-mode dispatch flows through this table so
// bindings can be remapped via configuration. Two-key sequences (gg, ]c, …),
// numeric count prefixes, and overlay/input keys are handled by the grammar and
// are not remappable.
type Keymap map[string]string

// DefaultKeymap returns the built-in bindings.
func DefaultKeymap() Keymap {
	return Keymap{
		"j":      "down",
		"k":      "up",
		"J":      "next-change",
		"K":      "prev-change",
		"G":      "last-line",
		"0":      "line-start",
		"$":      "line-end",
		"t":      "toggle-layout",
		"\\":     "sidebar",
		"v":      "select",
		"V":      "select-block",
		"c":      "comment",
		"e":      "edit",
		"f":      "files",
		"C":      "comments",
		"?":      "help",
		"q":      "quit",
		"esc":    "escape",
		"enter":  "open",
		"/":      "search",
		"n":      "next-match",
		"N":      "prev-match",
		":":      "cmdline",
		"ctrl+d": "half-page-down",
		"ctrl+u": "half-page-up",
		"r":      "reply",
		"s":      "submit",
	}
}

// knownActions is the set of action names a binding may target; overrides naming
// anything else are ignored.
var knownActions = func() map[string]bool {
	m := map[string]bool{}
	for _, a := range DefaultKeymap() {
		m[a] = true
	}
	return m
}()

// apply overlays user overrides (key -> action) onto the keymap. An empty action
// unbinds the key; an unknown action is ignored.
func (k Keymap) apply(overrides map[string]string) {
	for key, action := range overrides {
		if action == "" {
			delete(k, key)
			continue
		}
		if knownActions[action] {
			k[key] = action
		}
	}
}
