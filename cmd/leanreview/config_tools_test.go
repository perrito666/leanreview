package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/app"
	"github.com/perrito666/leanreview/internal/config"
)

// TestConfigSchemaActionsInSync pins the published schema's action enum to
// the code's actual action inventory: a new action added to the keymap
// without a schema update would make valid configs fail editor validation,
// and vice versa.
func TestConfigSchemaActionsInSync(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", "leanreview-config.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema struct {
		Properties struct {
			Keys struct {
				AdditionalProperties struct {
					Enum []string `json:"enum"`
				} `json:"additionalProperties"`
			} `json:"keys"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	inSchema := map[string]bool{}
	for _, a := range schema.Properties.Keys.AdditionalProperties.Enum {
		inSchema[a] = true
	}
	if !inSchema[""] {
		t.Errorf("schema must allow \"\" (unbind)")
	}
	for _, a := range app.KnownActions() {
		if !inSchema[a] {
			t.Errorf("action %q missing from the config schema enum", a)
		}
	}
	// len check catches schema entries for actions that no longer exist.
	if got, want := len(schema.Properties.Keys.AdditionalProperties.Enum), len(app.KnownActions())+1; got != want {
		t.Errorf("schema enum has %d entries, code has %d actions (+unbind)", got, want)
	}
}

// TestBaselineConfigIsCompleteAndValid: the generated baseline must contain
// the entire default keymap (the issue's ask: remap from a base, not from
// guesses), reference the schema, and pass the tool's own validator.
func TestBaselineConfigIsCompleteAndValid(t *testing.T) {
	out, err := config.BaselineJSON(app.DefaultKeymap())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("baseline is not valid JSON: %v", err)
	}
	if doc["$schema"] != config.SchemaURL {
		t.Errorf("$schema = %v", doc["$schema"])
	}
	keys := doc["keys"].(map[string]any)
	if len(keys) != len(app.DefaultKeymap()) {
		t.Errorf("baseline keys = %d, want the full default keymap (%d)", len(keys), len(app.DefaultKeymap()))
	}
	if problems := config.Validate(out, app.KnownActions()); len(problems) != 0 {
		t.Errorf("baseline fails the tool's own validator: %v", problems)
	}
}

// TestValidateFindsProblems exercises the validator's checks end to end.
func TestValidateFindsProblems(t *testing.T) {
	bad := []byte(`{
		"theem": "dark",
		"theme": "solarized",
		"list_engine": "hg",
		"tab_width": 0,
		"keys": {"j": "downn", "q": ""}
	}`)
	problems := config.Validate(bad, app.KnownActions())
	wantSubstrings := []string{"theem", "solarized", "hg", "tab_width", `keys["j"]`}
	for _, w := range wantSubstrings {
		found := false
		for _, p := range problems {
			if strings.Contains(p, w) {
				found = true
			}
		}
		if !found {
			t.Errorf("no problem mentions %q in %v", w, problems)
		}
	}
	// The empty action (unbind) is legal and must not be reported.
	for _, p := range problems {
		if strings.Contains(p, `keys["q"]`) {
			t.Errorf("unbinding must not be a problem: %v", p)
		}
	}
	if got := config.Validate([]byte(`{"theme": "mono"}`), app.KnownActions()); len(got) != 0 {
		t.Errorf("clean config reported problems: %v", got)
	}
}

// TestAttachmentHelpers: only content-addressed URLs get cache keys (with
// rotating signatures dropped) — a branch-addressed raw URL keeps its path
// while the content changes, so caching it serves a stale image from a
// previous session. The image sniff keeps HTML error pages out of the cache.
func TestAttachmentHelpers(t *testing.T) {
	signed := "https://private-user-images.githubusercontent.com/123/456-uuid.png"
	if got := attachmentCacheKey(signed + "?jwt=ROTATES"); got != signed {
		t.Errorf("signed asset key = %q", got)
	}
	if got := attachmentCacheKey("https://github.com/user-attachments/assets/900ae85f-1"); got == "" {
		t.Errorf("user-attachments asset should be cacheable")
	}
	if got := attachmentCacheKey("https://gitlab.com/o/r/uploads/abc123/shot.png"); got == "" {
		t.Errorf("gitlab upload (secret-addressed) should be cacheable")
	}
	sha := strings.Repeat("0123456789", 4)
	if got := attachmentCacheKey("https://raw.githubusercontent.com/o/r/" + sha + "/img.png"); got == "" {
		t.Errorf("commit-pinned raw URL should be cacheable")
	}
	if got := attachmentCacheKey("https://raw.githubusercontent.com/o/r/some-branch/img.png"); got != "" {
		t.Errorf("branch-addressed raw URL must NOT be cached, key = %q", got)
	}
	if looksLikeImage([]byte("<!DOCTYPE html><html>nope</html>")) {
		t.Errorf("HTML accepted as image")
	}
	// A real PNG header + minimal body decodes via DecodeConfig.
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	if !looksLikeImage(png) {
		t.Errorf("PNG header rejected")
	}
}
