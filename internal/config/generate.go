package config

import (
	"bytes"
	"encoding/json"
)

// SchemaURL is the published JSON Schema for the config file. The generated
// baseline references it via "$schema" so editors validate and complete the
// file with zero setup; Load and Validate ignore the key.
const SchemaURL = "https://perrito666.github.io/leanreview/schema/leanreview-config.schema.json"

// baseline mirrors fileConfig with concrete values and a stable field order,
// so the generated document reads top-down from identity to keys.
type baseline struct {
	Schema      string            `json:"$schema"`
	Editor      string            `json:"editor"`
	Author      string            `json:"author"`
	Syntax      bool              `json:"syntax"`
	SyntaxStyle string            `json:"syntax_style"`
	Theme       string            `json:"theme"`
	TabWidth    int               `json:"tab_width"`
	Context     int               `json:"context"`
	Wrap        bool              `json:"wrap"`
	WrapWidth   int               `json:"wrap_width"`
	ListEngine  string            `json:"list_engine"`
	ListFilter  string            `json:"list_filter"`
	ListFilters map[string]string `json:"list_filters"`
	Keys        map[string]string `json:"keys"`
}

// BaselineJSON renders the built-in defaults as a complete config document:
// every setting present with its default value and — the point of having a
// generator at all — the full default keymap spelled out, so remapping starts
// from the actual current bindings instead of guessing key and action names.
// keys is injected by the caller (the app package owns the keymap; config
// cannot depend on it).
func BaselineJSON(keys map[string]string) ([]byte, error) {
	b := baseline{
		Schema:      SchemaURL,
		SyntaxStyle: "auto",
		Syntax:      true,
		Theme:       "default",
		TabWidth:    4,
		Context:     3,
		Wrap:        true,
		WrapWidth:   120,
		ListEngine:  "gh",
		ListFilters: map[string]string{},
		Keys:        keys,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Path returns the config file location (see configPath), exported for the
// generator and validator CLI modes that must name it to the user.
func Path() string { return configPath() }
