package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
)

func fixtureFiles(t *testing.T) ([]diff.FileDiff, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "diff", "testdata", "simple.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	files, err := diff.ParsePatchBytes(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return files, data
}

func TestExchangeSniffing(t *testing.T) {
	if IsExchange([]byte("diff --git a/x b/x\n")) {
		t.Errorf("plain patch sniffed as exchange")
	}
	if IsExchange([]byte(`{"some": "json"}`)) {
		t.Errorf("unrelated JSON sniffed as exchange")
	}
	if !IsExchange([]byte("\n  {\"leanreview_review\": 1, \"patch\": \"x\"}")) {
		t.Errorf("exchange document not sniffed")
	}
}

func TestExchangeImportAnchorsAndOrphans(t *testing.T) {
	files, patch := fixtureFiles(t)
	e := &Exchange{
		Version: ExchangeVersion,
		Title:   "llm review",
		Summary: "two notes",
		Patch:   PatchText(patch),
		Comments: []ExchangeComment{
			{ID: "a1", Author: "assistant", Path: "internal/api/handler.go", Side: "RIGHT", StartLine: 72, Body: "handle the error"},
			{ID: "a2", Author: "assistant", Path: "internal/api/handler.go", Side: "RIGHT", StartLine: 9999, Body: "line is gone"},
			{ID: "a3", Author: "assistant", Path: "no/such.go", Side: "RIGHT", StartLine: 1, Body: "file is gone", State: "dismissed"},
		},
	}
	d := e.ToDraft("key", files)

	if len(d.Comments) != 3 {
		t.Fatalf("imported %d comments, want 3", len(d.Comments))
	}
	a1 := d.Get("a1")
	if a1 == nil || a1.State != DraftActive {
		t.Fatalf("anchorable comment not active: %+v", a1)
	}
	if a1.Snippet == "" || a1.Location.Anchor.AnchorText == "" {
		t.Errorf("anchored import should capture snippet and anchor: %+v", a1)
	}
	if a1.Author != "assistant" {
		t.Errorf("author lost on import")
	}
	if a2 := d.Get("a2"); a2 == nil || a2.State != DraftOrphaned {
		t.Errorf("unanchorable line should import as orphaned: %+v", a2)
	}
	// A dismissed comment keeps its human verdict even when unanchorable —
	// orphaning it would erase the reviewer's decision.
	if a3 := d.Get("a3"); a3 == nil || a3.State != DraftDismissed {
		t.Errorf("dismissed comment should stay dismissed: %+v", a3)
	}
	if d.Summary != "two notes" {
		t.Errorf("summary lost on import")
	}
}

func TestExchangeRoundTrip(t *testing.T) {
	files, patch := fixtureFiles(t)
	e := &Exchange{
		Version: ExchangeVersion,
		Title:   "t",
		Patch:   PatchText(patch),
		Comments: []ExchangeComment{{
			ID: "c1", Author: "assistant", Path: "internal/api/handler.go",
			Side: "RIGHT", StartLine: 72, Body: "note",
			Replies: []ReviewReply{{Author: "human", Body: "disagree"}},
		}},
	}
	d := e.ToDraft("key", files)
	d.Get("c1").State = DraftDismissed // the human's verdict

	out, err := MarshalExchange(FromDraft(d, patch))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !IsExchange(out) {
		t.Fatalf("round-tripped document does not sniff as exchange")
	}
	e2, err := ParseExchange(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	c := e2.Comments[0]
	if c.ID != "c1" || c.State != "dismissed" || c.Author != "assistant" {
		t.Errorf("round trip lost identity/state: %+v", c)
	}
	if len(c.Replies) != 1 || c.Replies[0].Body != "disagree" {
		t.Errorf("round trip lost replies: %+v", c.Replies)
	}
	if string(e2.Patch) != string(patch) {
		t.Errorf("round trip altered the patch")
	}
}

func TestParseExchangeRejectsBadDocuments(t *testing.T) {
	if _, err := ParseExchange([]byte(`{"leanreview_review": 99, "patch": "x"}`)); err == nil {
		t.Errorf("unknown version accepted")
	}
	if _, err := ParseExchange([]byte(`{"leanreview_review": 1}`)); err == nil {
		t.Errorf("missing patch accepted")
	}
}

func TestRenderExportDispatchesByExtension(t *testing.T) {
	_, patch := fixtureFiles(t)
	d := NewDraftReview("k", "t", "")
	d.Add(diff.Location{Path: "f.go", Side: diff.SideRight, StartLine: 1, EndLine: 1}, "note", "snip")

	md, err := RenderExport("out.md", d, patch)
	if err != nil || !strings.HasPrefix(string(md), "# Review:") {
		t.Errorf("markdown export wrong: %v\n%s", err, md)
	}
	js, err := RenderExport("out.JSON", d, patch)
	if err != nil || !IsExchange(js) {
		t.Errorf("json export should produce an exchange document: %v", err)
	}
	if _, err := RenderExport("out.json", d, nil); err == nil {
		t.Errorf("exchange export without a patch should error")
	}
}
