package app

// command is a resolved normal-mode command: a name plus an optional numeric
// count prefix (defaulting to 1 via CountOr).
type command struct {
	name  string
	count int
}

// CountOr returns the accumulated count, or d when none was typed.
func (c command) CountOr(d int) int {
	if c.count <= 0 {
		return d
	}
	return c.count
}

// pendingCommand implements a small Vim-like grammar: numeric prefixes and
// two-key sequences (gg, ]c, … by default). Single keys resolve immediately;
// a key that begins a known sequence waits for its second.
type pendingCommand struct {
	prefix string
	count  int
}

// seqKey identifies a two-key sequence by its keys in press order. Keys are
// kept separate (not concatenated) so multi-character key names like
// "ctrl+d" can participate without ambiguity.
type seqKey struct{ first, second string }

// Sequences maps two-key sequences to action names. Unlisted combinations
// are discarded by the grammar.
type Sequences map[seqKey]string

// DefaultSequences returns the built-in two-key bindings as a fresh map on
// every call, so apply can mutate one model's sequences without affecting
// other instances or the knownActions set derived from the defaults.
func DefaultSequences() Sequences {
	return Sequences{
		{"g", "g"}: "first-line",
		{"]", "c"}: "next-hunk",
		{"[", "c"}: "prev-hunk",
		{"]", "f"}: "next-file",
		{"[", "f"}: "prev-file",
		{"d", "d"}: "delete-comment",
		{"z", "a"}: "toggle-fold",
		{"z", "R"}: "expand-all",
		{"z", "M"}: "collapse-all",
	}
}

// startsWith reports whether any sequence begins with key — the grammar's
// cue to hold the key as a pending prefix instead of dispatching it.
func (s Sequences) startsWith(key string) bool {
	for k := range s {
		if k.first == key {
			return true
		}
	}
	return false
}

// Feed consumes one key, resolving single keys through the supplied keymap
// and two-key sequences through seqs. It returns (cmd, true) when a command
// is ready, or (_, false) when more input is needed (a pending prefix) or
// the key was a digit that only extended the count. Sequence prefixes are
// checked before single bindings, so a key that both starts a sequence and
// is singly bound acts as a prefix — the validator flags that overlap.
func (p *pendingCommand) Feed(key string, single Keymap, seqs Sequences) (command, bool) {
	// Numeric prefix accumulation (0 only counts once a prefix has started, so
	// a bare "0" remains the go-to-line-start motion).
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' || (key == "0" && p.count > 0) {
		p.count = p.count*10 + int(key[0]-'0')
		return command{}, false
	}

	if p.prefix != "" {
		name, ok := seqs[seqKey{p.prefix, key}]
		p.prefix = ""
		cnt := p.count
		p.count = 0
		if !ok {
			return command{}, false
		}
		return command{name: name, count: cnt}, true
	}

	if seqs.startsWith(key) {
		p.prefix = key
		return command{}, false
	}

	name := single[key]
	if name == "" {
		// Unknown single key: reset any count and ignore.
		p.count = 0
		return command{}, false
	}
	cnt := p.count
	p.count = 0
	return command{name: name, count: cnt}, true
}
