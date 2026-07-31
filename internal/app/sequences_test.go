package app

import (
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/ui"
)

// TestSequenceOverridesReachTheGrammar: configured sequences add, rebind, and
// remove two-key bindings — the point of making them data instead of a
// hardcoded map.
func TestSequenceOverridesReachTheGrammar(t *testing.T) {
	t.Setenv("LEANREVIEW_SYNTAX", "0")
	m := New(Config{Files: loadAppFixture(t), Title: "test", Theme: ui.DefaultTheme(), Sequences: []SeqBinding{
		{First: ",", Second: "c", Action: "next-hunk"}, // new sequence
		{First: "d", Second: "d", Action: ""},          // removed default
	}})
	m.width, m.height = 100, 30

	// The new sequence dispatches like a built-in one.
	var p pendingCommand
	if _, ok := p.Feed(",", m.keymap, m.sequences); ok {
		t.Fatalf("',' should be held as a pending prefix")
	}
	cmd, ok := p.Feed("c", m.keymap, m.sequences)
	if !ok || cmd.name != "next-hunk" {
		t.Errorf(",c = (%v, %v), want next-hunk", cmd, ok)
	}

	// The removed default no longer resolves — and with no other d-sequence
	// left, d is not even a prefix.
	if _, ok := m.sequences[seqKey{"d", "d"}]; ok {
		t.Errorf("dd survived its removal")
	}
	if m.sequences.startsWith("d") {
		t.Errorf("d still counts as a sequence prefix after dd removal")
	}
}

// TestSequenceCountPrefix: counts apply to configured sequences exactly as
// they do to built-ins (3]c means third next-hunk).
func TestSequenceCountPrefix(t *testing.T) {
	var p pendingCommand
	seqs := DefaultSequences()
	km := DefaultKeymap()
	p.Feed("3", km, seqs)
	p.Feed("]", km, seqs)
	cmd, ok := p.Feed("c", km, seqs)
	if !ok || cmd.name != "next-hunk" || cmd.CountOr(1) != 3 {
		t.Errorf("3]c = (%+v, %v), want next-hunk x3", cmd, ok)
	}
}

// TestValidateBindings: the overlap rules the structural validator cannot
// see — dispatch-order shadowing that leaves bindings silently dead.
func TestValidateBindings(t *testing.T) {
	cases := []struct {
		name    string
		keys    map[string]string
		seqs    []SeqBinding
		errPart string
	}{
		{name: "clean defaults", errPart: ""},
		{name: "single binding shadowed by default sequence prefix",
			keys:    map[string]string{"g": "down"},
			errPart: `key "g" is bound`},
		{name: "new sequence shadowing a default binding",
			seqs:    []SeqBinding{{First: "j", Second: "j", Action: "quit"}},
			errPart: `key "j" is bound`},
		{name: "digit single binding is dead",
			keys:    map[string]string{"5": "quit"},
			errPart: "digits are count prefixes"},
		{name: "digit-first sequence is dead",
			seqs:    []SeqBinding{{First: "2", Second: "x", Action: "quit"}},
			errPart: "starts with a digit"},
		{name: "duplicate sequence",
			seqs: []SeqBinding{
				{First: ",", Second: "c", Action: "next-hunk"},
				{First: ",", Second: "c", Action: "prev-hunk"},
			},
			errPart: "defined more than once"},
		{name: "removing the default sequence clears the conflict",
			keys: map[string]string{"d": "down"},
			seqs: []SeqBinding{{First: "d", Second: "d", Action: ""}},
		},
	}
	for _, c := range cases {
		got := ValidateBindings(c.keys, c.seqs)
		if c.errPart == "" {
			if len(got) != 0 {
				t.Errorf("%s: unexpected problems %v", c.name, got)
			}
			continue
		}
		found := false
		for _, p := range got {
			if strings.Contains(p, c.errPart) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: problems %v missing %q", c.name, got, c.errPart)
		}
	}
}
