package source

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/review"
)

func exchangeFixture(t *testing.T) string {
	t.Helper()
	patch, err := os.ReadFile(filepath.Join("..", "diff", "testdata", "simple.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	e := &review.Exchange{
		Version: review.ExchangeVersion,
		Title:   "llm review",
		Patch:   review.PatchText(patch),
		Comments: []review.ExchangeComment{{
			ID: "c1", Author: "assistant", Path: "internal/api/handler.go",
			Side: "RIGHT", StartLine: 72, Body: "handle the error",
		}},
	}
	data, err := review.MarshalExchange(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "review.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestPatchPathOpensExchangeSource(t *testing.T) {
	path := exchangeFixture(t)
	src, err := newPatchFileSource(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ex, ok := src.(*ExchangeSource)
	if !ok {
		t.Fatalf("exchange file opened as %T, want *ExchangeSource", src)
	}
	if ex.Title() != "llm review" {
		t.Errorf("title = %q", ex.Title())
	}
	files, err := ex.Files(context.Background())
	if err != nil || len(files) == 0 {
		t.Fatalf("embedded patch did not parse: %v", err)
	}
	raw, err := ex.RawPatch(context.Background())
	if err != nil || !strings.HasPrefix(string(raw), "diff --git") {
		t.Errorf("RawPatch should return the embedded patch: %v", err)
	}
	// Key must be path-based: the file content changes every round trip.
	if !strings.HasPrefix(ex.Key(), "exchange-") {
		t.Errorf("key = %q", ex.Key())
	}
}

func TestExchangeRejectedOnStdin(t *testing.T) {
	path := exchangeFixture(t)
	data, _ := os.ReadFile(path)
	if _, err := newStdinSource(strings.NewReader(string(data))); err == nil || !strings.Contains(err.Error(), "stdin") {
		t.Errorf("exchange on stdin should be refused with a pointer to files, got %v", err)
	}
}

func TestMalformedExchangeFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	os.WriteFile(path, []byte(`{"leanreview_review": 1}`), 0o644)
	if _, err := newPatchFileSource(path); err == nil {
		t.Errorf("exchange without a patch should fail to open, not parse as a diff")
	}
}
