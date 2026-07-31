package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// knownFileKeys are the top-level keys the config file understands, plus the
// "$schema" editor convention. Derived from fileConfig's tags at init so the
// validator can never drift from the parser.
var knownFileKeys = func() map[string]bool {
	keys := map[string]bool{"$schema": true}
	// Marshal an empty fileConfig by round-tripping its field tags: encode a
	// fully-populated instance and collect the emitted keys.
	s, b8, n := "", false, 0
	fc := fileConfig{
		Editor: &s, Syntax: &b8, SyntaxStyle: &s, Theme: &s, TabWidth: &n,
		Context: &n, Keys: map[string]string{}, ListEngine: &s, ListFilter: &s,
		ListFilters: map[string]string{}, Wrap: &b8, WrapWidth: &n, Author: &s,
		Images: &s, ChangeColors: &s, ChangeTint: &b8,
		Sequences: []SequenceBinding{},
	}
	raw, _ := json.Marshal(fc)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	for k := range m {
		keys[k] = true
	}
	return keys
}()

// Validate reports every problem in a config document as a human-readable
// list: malformed JSON, unknown keys (the classic silent typo), out-of-range
// numbers, invalid enum values, and key bindings naming actions that do not
// exist. knownActions is injected by the caller because the app package owns
// the action inventory. An empty result means the file is clean.
func Validate(data []byte, knownActions []string) []string {
	var problems []string
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("not a JSON object: %v", err)}
	}

	// Unknown keys, deterministically ordered for stable output.
	var unknown []string
	for k := range raw {
		if !knownFileKeys[k] {
			unknown = append(unknown, k)
		}
	}
	slices.Sort(unknown)
	for _, k := range unknown {
		problems = append(problems, fmt.Sprintf("unknown setting %q (typo? it is silently ignored)", k))
	}

	// Type errors, via the same decoder Load uses.
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		problems = append(problems, fmt.Sprintf("type error: %v", err))
		return problems
	}

	if fc.Theme != nil && *fc.Theme != "" && *fc.Theme != "default" && *fc.Theme != "mono" {
		problems = append(problems, fmt.Sprintf("theme %q is not one of: default, mono", *fc.Theme))
	}
	if fc.ChangeColors != nil && *fc.ChangeColors != "" && *fc.ChangeColors != "diff" && *fc.ChangeColors != "syntax" {
		problems = append(problems, fmt.Sprintf("change_colors %q is not one of: diff, syntax", *fc.ChangeColors))
	}
	if fc.Images != nil && *fc.Images != "" && *fc.Images != "auto" && *fc.Images != "kitty" && *fc.Images != "chafa" && *fc.Images != "off" {
		problems = append(problems, fmt.Sprintf("images %q is not one of: auto, kitty, chafa, off", *fc.Images))
	}
	if fc.ListEngine != nil && *fc.ListEngine != "" && *fc.ListEngine != "gh" && *fc.ListEngine != "glab" {
		problems = append(problems, fmt.Sprintf("list_engine %q is not one of: gh, glab", *fc.ListEngine))
	}
	if fc.TabWidth != nil && *fc.TabWidth <= 0 {
		problems = append(problems, fmt.Sprintf("tab_width %d must be positive", *fc.TabWidth))
	}
	if fc.Context != nil && *fc.Context < 0 {
		problems = append(problems, fmt.Sprintf("context %d must not be negative", *fc.Context))
	}
	if fc.WrapWidth != nil && *fc.WrapWidth <= 0 {
		problems = append(problems, fmt.Sprintf("wrap_width %d must be positive", *fc.WrapWidth))
	}

	actions := map[string]bool{}
	for _, a := range knownActions {
		actions[a] = true
	}
	for _, k := range slices.Sorted(maps.Keys(fc.Keys)) {
		if a := fc.Keys[k]; a != "" && !actions[a] {
			problems = append(problems, fmt.Sprintf("keys[%q] names unknown action %q (known: %s)", k, a, strings.Join(knownActions, ", ")))
		}
	}
	for i, sb := range fc.Sequences {
		if len(sb.Keys) != 2 {
			problems = append(problems, fmt.Sprintf("sequences[%d]: needs exactly 2 keys, has %d", i, len(sb.Keys)))
			continue
		}
		if sb.Keys[0] == "" || sb.Keys[1] == "" {
			problems = append(problems, fmt.Sprintf("sequences[%d]: keys must not be empty", i))
		}
		if sb.Action != "" && !actions[sb.Action] {
			problems = append(problems, fmt.Sprintf("sequences[%d] (%q %q) names unknown action %q (known: %s)", i, sb.Keys[0], sb.Keys[1], sb.Action, strings.Join(knownActions, ", ")))
		}
	}
	return problems
}
