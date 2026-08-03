package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
)

// FillJSON completes an existing config document with the defaults it omits:
// every missing top-level setting is added at its default value, missing
// default key bindings join the user's keys map, and missing default
// sequences join the sequences list. Existing values — including settings the
// validator would flag — are never touched, and unknown keys are preserved
// (fill exists to complete a config, not to police or reformat it). The
// returned list describes what was added; empty means the file was already
// complete and the caller need not rewrite it.
func FillJSON(existing []byte, keys map[string]string, seqs []SequenceBinding) ([]byte, []string, error) {
	var user map[string]json.RawMessage
	if err := json.Unmarshal(existing, &user); err != nil {
		return nil, nil, fmt.Errorf("not a JSON object: %w", err)
	}
	baseRaw, err := BaselineJSON(keys, seqs)
	if err != nil {
		return nil, nil, err
	}
	var base map[string]json.RawMessage
	if err := json.Unmarshal(baseRaw, &base); err != nil {
		return nil, nil, err
	}

	var added []string
	merged := map[string]json.RawMessage{}
	for k, v := range base {
		if uv, ok := user[k]; ok {
			merged[k] = uv
		} else {
			merged[k] = v
			added = append(added, k)
		}
	}
	slices.Sort(added)

	// The keys map fills per binding: the user's bindings win, absent
	// defaults are spelled out (absent already meant the default applied, so
	// this changes nothing behaviorally — it makes the map remappable-from).
	if uv, ok := user["keys"]; ok {
		var uk map[string]string
		if err := json.Unmarshal(uv, &uk); err != nil {
			return nil, nil, fmt.Errorf("keys: %w", err)
		}
		n := 0
		for k, a := range keys {
			if _, exists := uk[k]; !exists {
				uk[k] = a
				n++
			}
		}
		if n > 0 {
			added = append(added, fmt.Sprintf("keys (%d default binding(s))", n))
			if merged["keys"], err = compactJSON(uk); err != nil {
				return nil, nil, err
			}
		}
	}

	// Sequences fill by key pair, appended after the user's entries.
	if uv, ok := user["sequences"]; ok {
		var us []SequenceBinding
		if err := json.Unmarshal(uv, &us); err != nil {
			return nil, nil, fmt.Errorf("sequences: %w", err)
		}
		have := map[[2]string]bool{}
		for _, sb := range us {
			if len(sb.Keys) == 2 {
				have[[2]string{sb.Keys[0], sb.Keys[1]}] = true
			}
		}
		n := 0
		for _, sb := range seqs {
			if !have[[2]string{sb.Keys[0], sb.Keys[1]}] {
				us = append(us, sb)
				n++
			}
		}
		if n > 0 {
			added = append(added, fmt.Sprintf("sequences (%d default sequence(s))", n))
			if merged["sequences"], err = compactJSON(us); err != nil {
				return nil, nil, err
			}
		}
	}

	if len(added) == 0 {
		return existing, nil, nil
	}

	// Output in the baseline's field order (identity first, keys last), with
	// the user's unknown keys preserved at the end — fill must never lose
	// data, even data the validator would complain about.
	order := topLevelKeyOrder(baseRaw)
	var extras []string
	for k := range user {
		if _, ok := base[k]; !ok {
			extras = append(extras, k)
			merged[k] = user[k]
		}
	}
	slices.Sort(extras)
	order = append(order, extras...)

	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, k := range order {
		var val bytes.Buffer
		if err := json.Indent(&val, merged[k], "  ", "  "); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", k, err)
		}
		fmt.Fprintf(&buf, "  %q: %s", k, val.Bytes())
		if i < len(order)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("}\n")
	return buf.Bytes(), added, nil
}

// compactJSON marshals without HTML escaping (the config codec's standing
// rule) and without the trailing newline json.Encoder appends.
func compactJSON(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

// topLevelKeyOrder reads a JSON object's key order via the token stream —
// maps cannot carry it.
func topLevelKeyOrder(data []byte) []string {
	dec := json.NewDecoder(bytes.NewReader(data))
	var order []string
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			return order
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		case string:
			if depth == 1 {
				order = append(order, t)
				// Skip the value so its inner keys are not mistaken for
				// top-level ones.
				var v json.RawMessage
				if err := dec.Decode(&v); err != nil {
					return order
				}
			}
		}
	}
}
