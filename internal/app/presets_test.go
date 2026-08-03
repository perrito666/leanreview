package app

import (
	"strings"
	"testing"
)

// TestPresetLayersOverDefaults: a preset adds its chords without losing the
// default bindings — terminals cannot report every editor chord, so presets
// are deliberately supersets, not clones.
func TestPresetLayersOverDefaults(t *testing.T) {
	km, sq := resolveKeymap("vscode", nil, nil, nil)
	if km["ctrl+f"] != "search" || km["f3"] != "next-match" || km["home"] != "first-line" {
		t.Errorf("vscode chords missing: %v", km)
	}
	if km["j"] != "down" || km["c"] != "comment" {
		t.Errorf("defaults lost under the preset")
	}
	if sq[seqKey{"g", "g"}] != "first-line" {
		t.Errorf("default sequences lost under the preset")
	}
}

// TestEveryBuiltinPresetResolves: each advertised name yields working tables
// and only known actions.
func TestEveryBuiltinPresetResolves(t *testing.T) {
	for _, name := range BuiltinKeymapNames() {
		km, _ := resolveKeymap(name, nil, nil, nil)
		for k, a := range km {
			if !knownActions[a] {
				t.Errorf("%s: %q → unknown action %q", name, k, a)
			}
		}
	}
	if len(BuiltinKeymapNames()) != 5 {
		t.Errorf("presets = %v, want default/vim/vscode/sublime/intellij", BuiltinKeymapNames())
	}
}

// TestUserKeymapAndOverridePrecedence: a named user keymap layers on the
// defaults, and top-level keys still override the chosen base.
func TestUserKeymapAndOverridePrecedence(t *testing.T) {
	user := map[string]NamedKeymap{"mine": {
		Keys:      map[string]string{"o": "next-hunk"},
		Sequences: []SeqBinding{{First: ",", Second: "c", Action: "prev-hunk"}},
	}}
	km, sq := resolveKeymap("mine", user, map[string]string{"o": "prev-hunk"}, nil)
	if km["o"] != "prev-hunk" {
		t.Errorf("top-level keys must override the named keymap, got %q", km["o"])
	}
	if sq[seqKey{",", "c"}] != "prev-hunk" {
		t.Errorf("named keymap sequences not applied")
	}
	if km["j"] != "down" {
		t.Errorf("defaults lost under the user keymap")
	}
}

// TestValidateKeymapChoice: unknown names and reserved-name shadowing are
// reported — a silent fallback to defaults would be the invisible typo.
func TestValidateKeymapChoice(t *testing.T) {
	if got := ValidateKeymapChoice("vim", nil); len(got) != 0 {
		t.Errorf("builtin choice reported problems: %v", got)
	}
	if got := ValidateKeymapChoice("mine", map[string]NamedKeymap{"mine": {}}); len(got) != 0 {
		t.Errorf("defined user keymap reported problems: %v", got)
	}
	got := ValidateKeymapChoice("nope", nil)
	if len(got) != 1 || !strings.Contains(got[0], "nope") {
		t.Errorf("unknown name not reported: %v", got)
	}
	got = ValidateKeymapChoice("", map[string]NamedKeymap{"vim": {}})
	if len(got) != 1 || !strings.Contains(got[0], "built-in preset name") {
		t.Errorf("reserved shadowing not reported: %v", got)
	}
}
