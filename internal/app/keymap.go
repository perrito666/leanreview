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
		"up":     "up",
		"down":   "down",
		"left":   "motion-left",
		"right":  "motion-right",
		"h":      "motion-left",
		"l":      "motion-right",
		"pgup":   "page-up",
		"pgdown": "page-down",
		"t":      "toggle-layout",
		"i":      "toggle-inline",
		"w":      "toggle-wrap",
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

// extraActions are valid binding targets that no default key maps to: the
// split-only and scroll-only halves of the combined motion actions.
var extraActions = []string{"side-left", "side-right", "scroll-left", "scroll-right"}

// knownActions is the set of action names a binding may target — the union of
// the default single-key actions, the two-key sequence actions, and the extra
// actions — so a user may bind a single key to any action (e.g. "next-hunk" or
// "scroll-left"). Overrides naming anything else are ignored.
var knownActions = func() map[string]bool {
	m := map[string]bool{}
	for _, a := range DefaultKeymap() {
		m[a] = true
	}
	for _, a := range twoKey {
		m[a] = true
	}
	for _, a := range extraActions {
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
