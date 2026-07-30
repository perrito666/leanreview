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

// pendingCommand implements a small Vim-like grammar: numeric prefixes and the
// two-key sequences gg, ]c, [c, ]f, [f, dd, za, zR. Single keys resolve
// immediately; a recognised first key (g, ], [, d, z) waits for its second.
type pendingCommand struct {
	prefix string
	count  int
}

// firstKeys are keys that begin a known two-key sequence.
var firstKeys = map[string]bool{"g": true, "]": true, "[": true, "d": true, "z": true}

// twoKey maps a completed two-key sequence to its command name. Unlisted
// combinations are discarded.
var twoKey = map[string]string{
	"gg": "first-line",
	"]c": "next-hunk",
	"[c": "prev-hunk",
	"]f": "next-file",
	"[f": "prev-file",
	"dd": "delete-comment",
	"za": "toggle-fold",
	"zR": "expand-all",
	"zM": "collapse-all",
}

// Feed consumes one key, resolving single keys through the supplied keymap. It
// returns (cmd, true) when a command is ready, or (_, false) when more input is
// needed (a pending prefix) or the key was a digit that only extended the count.
func (p *pendingCommand) Feed(key string, single Keymap) (command, bool) {
	// Numeric prefix accumulation (0 only counts once a prefix has started, so
	// a bare "0" remains the go-to-line-start motion).
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' || (key == "0" && p.count > 0) {
		p.count = p.count*10 + int(key[0]-'0')
		return command{}, false
	}

	if p.prefix != "" {
		seq := p.prefix + key
		p.prefix = ""
		name, ok := twoKey[seq]
		cnt := p.count
		p.count = 0
		if !ok {
			return command{}, false
		}
		return command{name: name, count: cnt}, true
	}

	if firstKeys[key] {
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

// Pending reports whether the grammar is mid-sequence (a prefix is held).
func (p *pendingCommand) Pending() bool { return p.prefix != "" }

// Reset clears any pending state.
func (p *pendingCommand) Reset() { p.prefix = ""; p.count = 0 }
