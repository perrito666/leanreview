package app

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Keymap maps a key (as reported by Bubble Tea's KeyMsg.String) to a normal-mode
// action name. Single-key normal-mode dispatch flows through this table, and
// two-key sequences through Sequences — both remappable via configuration.
// Numeric count prefixes and overlay/input keys are handled by the grammar
// and each overlay's own handler, and are not remappable.
type Keymap map[string]string

// DefaultKeymap returns the built-in bindings as a fresh map on every call,
// so apply can mutate one model's keymap without affecting other instances or
// the knownActions set derived from the defaults.
func DefaultKeymap() Keymap {
	return Keymap{
		"j":         "down",
		"k":         "up",
		"J":         "next-change",
		"K":         "prev-change",
		"G":         "last-line",
		"0":         "line-start",
		"$":         "line-end",
		"up":        "up",
		"down":      "down",
		"left":      "motion-left",
		"right":     "motion-right",
		"h":         "motion-left",
		"l":         "motion-right",
		"pgup":      "page-up",
		"pgdown":    "page-down",
		"t":         "toggle-layout",
		"T":         "toggle-context",
		"S":         "toggle-syntax",
		"i":         "toggle-inline",
		"w":         "toggle-wrap",
		"\\":        "sidebar",
		"v":         "select",
		"V":         "select-block",
		"c":         "comment",
		"e":         "edit",
		"f":         "files",
		"C":         "comments",
		"?":         "help",
		"q":         "quit",
		"esc":       "escape",
		"enter":     "open",
		"/":         "search",
		"n":         "next-match",
		"N":         "prev-match",
		":":         "cmdline",
		"ctrl+d":    "half-page-down",
		"ctrl+u":    "half-page-up",
		"r":         "reply",
		"s":         "submit",
		"p":         "pr-info",
		"x":         "dismiss",
		"R":         "suggest",
		"tab":       "next-file",
		"shift+tab": "prev-file",
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
	for _, a := range DefaultSequences() {
		m[a] = true
	}
	for _, a := range extraActions {
		m[a] = true
	}
	return m
}()

// SeqBinding is one two-key sequence override from configuration: the two
// keys in press order plus the action, or "" to remove the sequence.
type SeqBinding struct {
	First  string
	Second string
	Action string
}

// apply overlays configured sequence overrides: an empty action removes the
// sequence, unknown actions are ignored (Validate reports them; a typo must
// not take the whole binding table down at startup).
func (s Sequences) apply(overrides []SeqBinding) {
	for _, o := range overrides {
		k := seqKey{o.First, o.Second}
		if o.Action == "" {
			delete(s, k)
			continue
		}
		if knownActions[o.Action] {
			s[k] = o.Action
		}
	}
}

// DefaultSequenceBindings returns the built-in two-key sequences in a stable
// order — the config generator spells them out so remapping starts from the
// real current bindings.
func DefaultSequenceBindings() []SeqBinding {
	var out []SeqBinding
	for k, a := range DefaultSequences() {
		out = append(out, SeqBinding{First: k.first, Second: k.second, Action: a})
	}
	slices.SortFunc(out, func(a, b SeqBinding) int {
		if c := strings.Compare(a.First, b.First); c != 0 {
			return c
		}
		return strings.Compare(a.Second, b.Second)
	})
	return out
}

// ValidateBindings reports overlap problems the structural config validator
// cannot see: the grammar consumes digits (counts) and sequence prefixes
// before single-key dispatch, so bindings shadowed by that order are dead —
// exactly the kind of config mistake that otherwise surfaces as "my key does
// nothing". The effective tables (defaults + overrides) are what is checked,
// so removing the default sequence also clears the conflict.
func ValidateBindings(keys map[string]string, seqOverrides []SeqBinding) []string {
	var problems []string
	km := DefaultKeymap()
	km.apply(keys)
	for _, k := range slices.Sorted(maps.Keys(keys)) {
		if keys[k] != "" && len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
			problems = append(problems, fmt.Sprintf("keys[%q]: digits are count prefixes (e.g. 12j) and cannot be bound", k))
		}
	}

	seqs := DefaultSequences()
	seen := map[seqKey]bool{}
	for _, o := range seqOverrides {
		k := seqKey{o.First, o.Second}
		if seen[k] {
			problems = append(problems, fmt.Sprintf("sequences: %q %q is defined more than once", o.First, o.Second))
		}
		seen[k] = true
		if len(o.First) == 1 && o.First[0] >= '0' && o.First[0] <= '9' && o.Action != "" {
			problems = append(problems, fmt.Sprintf("sequences: %q %q starts with a digit, which the count grammar consumes first", o.First, o.Second))
		}
	}
	seqs.apply(seqOverrides)

	prefixes := map[string]bool{}
	for k := range seqs {
		prefixes[k.first] = true
	}
	for _, p := range slices.Sorted(maps.Keys(prefixes)) {
		if a := km[p]; a != "" {
			problems = append(problems, fmt.Sprintf("key %q is bound to %q but also starts a two-key sequence — the sequence prefix wins and the single binding is dead (unbind one)", p, a))
		}
	}
	return problems
}

// KnownActions returns the sorted set of action names a key may bind to —
// the shared contract between the keymap, the CLI config validator, and the
// published config schema (a sync test keeps the latter honest).
func KnownActions() []string {
	return slices.Sorted(maps.Keys(knownActions))
}

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
