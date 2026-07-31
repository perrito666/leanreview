package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestParseArgs pins the contract of the hand-rolled parser: flags and
// positionals interleave freely, value flags consume exactly one argument,
// boolean flags consume none (their following argument is a positional —
// --list's engine/filter and any review target arrive through opts.args),
// and errors name the offending input.
func TestParseArgs(t *testing.T) {
	cases := []struct {
		name    string
		argv    []string
		want    options
		errPart string
	}{
		// Positionals: the review target forms.
		{name: "no args", argv: nil,
			want: options{contextN: 3}},
		{name: "diff file target", argv: []string{"a.diff"},
			want: options{contextN: 3, args: []string{"a.diff"}}},
		{name: "stdin dash target", argv: []string{"-"},
			want: options{contextN: 3, args: []string{"-"}}},
		{name: "revision range", argv: []string{"main", "HEAD"},
			want: options{contextN: 3, args: []string{"main", "HEAD"}}},
		{name: "PR URL target", argv: []string{"https://github.com/o/r/pull/7"},
			want: options{contextN: 3, args: []string{"https://github.com/o/r/pull/7"}}},

		// Value flags consume exactly the next argument.
		{name: "base", argv: []string{"--base", "main"},
			want: options{contextN: 3, base: "main"}},
		{name: "base then target", argv: []string{"--base", "main", "x.diff"},
			want: options{contextN: 3, base: "main", args: []string{"x.diff"}}},
		{name: "context long", argv: []string{"--context", "7"},
			want: options{contextN: 7, contextSet: true}},
		{name: "context short", argv: []string{"-U", "7"},
			want: options{contextN: 7, contextSet: true}},
		{name: "export", argv: []string{"--export", "out.md"},
			want: options{contextN: 3, exportPath: "out.md"}},

		// Boolean flags consume nothing: what follows is a positional.
		{name: "staged then target", argv: []string{"--staged", "x.diff"},
			want: options{contextN: 3, staged: true, args: []string{"x.diff"}}},
		{name: "list engine and filter are positionals", argv: []string{"--list", "gh", "org:corp"},
			want: options{contextN: 3, list: true, args: []string{"gh", "org:corp"}}},
		{name: "discard then target", argv: []string{"--discard", "a.diff"},
			want: options{contextN: 3, discard: true, args: []string{"a.diff"}}},
		{name: "init-config", argv: []string{"--init-config"},
			want: options{contextN: 3, initConfig: true}},
		{name: "check-config", argv: []string{"--check-config"},
			want: options{contextN: 3, checkConfig: true}},

		// Interleaving: flags may come after positionals.
		{name: "target then flag", argv: []string{"x.diff", "--staged"},
			want: options{contextN: 3, staged: true, args: []string{"x.diff"}}},
		{name: "flag positional flag", argv: []string{"--staged", "x.diff", "-U", "1"},
			want: options{contextN: 1, contextSet: true, staged: true, args: []string{"x.diff"}}},

		// "--" ends flag parsing: it is consumed, and everything after it
		// is a positional even when it starts with a dash.
		{name: "separator escapes a flag-looking target", argv: []string{"--", "--staged"},
			want: options{contextN: 3, args: []string{"--staged"}}},
		{name: "flags before separator still parse", argv: []string{"--base", "main", "--", "-x.diff"},
			want: options{contextN: 3, base: "main", args: []string{"-x.diff"}}},
		{name: "trailing separator alone", argv: []string{"--"},
			want: options{contextN: 3}},
		{name: "second separator is a literal", argv: []string{"--", "--"},
			want: options{contextN: 3, args: []string{"--"}}},

		// Errors name the offending input.
		{name: "base missing value", argv: []string{"--base"}, errPart: `"--base"`},
		{name: "base followed by a flag", argv: []string{"--base", "--staged"}, errPart: `"--base"`},
		{name: "context missing value", argv: []string{"--context"}, errPart: `"--context/-U"`},
		{name: "context non-numeric", argv: []string{"-U", "many"}, errPart: "many"},
		{name: "export missing value", argv: []string{"--export"}, errPart: `"--export"`},
		{name: "unknown long flag", argv: []string{"--frobnicate"}, errPart: `"--frobnicate"`},
		{name: "unknown short flag", argv: []string{"-z"}, errPart: `"-z"`},
		{name: "unknown flag before valid args", argv: []string{"--nope", "a.diff"}, errPart: `"--nope"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseArgs(c.argv)
			if c.errPart != "" {
				if err == nil || !strings.Contains(err.Error(), c.errPart) {
					t.Fatalf("parseArgs(%q) err = %v, want containing %s", c.argv, err, c.errPart)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArgs(%q) unexpected error: %v", c.argv, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseArgs(%q)\n got: %+v\nwant: %+v", c.argv, got, c.want)
			}
		})
	}
}

// TestParseArgsHelpSentinels: -h/--help and -v/--version return their
// sentinels (never a failure), so run prints and exits 0.
func TestParseArgsHelpSentinels(t *testing.T) {
	for _, f := range []string{"-h", "--help"} {
		if _, err := parseArgs([]string{f}); !errors.Is(err, errHelp) {
			t.Errorf("parseArgs(%q) err = %v, want errHelp", f, err)
		}
	}
	for _, f := range []string{"-v", "--version"} {
		if _, err := parseArgs([]string{f}); !errors.Is(err, errVersion) {
			t.Errorf("parseArgs(%q) err = %v, want errVersion", f, err)
		}
	}
	// The sentinel wins even with other arguments present.
	if _, err := parseArgs([]string{"--staged", "--help"}); !errors.Is(err, errHelp) {
		t.Errorf("--help after other flags err = %v, want errHelp", err)
	}
}

// TestStrFlagToCmdFlag: dash stripping maps spellings to flag identities; a
// bare "-" is not a flag (it is the stdin target), and unknown names are
// reported as such.
func TestStrFlagToCmdFlag(t *testing.T) {
	cases := []struct {
		in   string
		want cmdFlag
	}{
		{"--base", flagBase},
		{"-U", flagContext},
		{"--context", flagContext},
		{"-h", flagHelp},
		{"--help", flagHelp},
		{"-", flagNoFlag},
		{"--", flagNoFlag},
		{"--frobnicate", flagUnknown},
	}
	for _, c := range cases {
		if got := strFlagToCmdFlag(c.in); got != c.want {
			t.Errorf("strFlagToCmdFlag(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
