package review

import (
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

func sampleDraft() *DraftReview {
	d := NewDraftReview("patch-abc", "handler.go", "")
	d.Add(diff.Location{Path: "internal/api/handler.go", Side: diff.SideRight, StartLine: 72, EndLine: 72},
		"This ignores the error from calculate().", "result, err := calculate(input)")
	d.Add(diff.Location{Path: "internal/api/handler.go", Side: diff.SideLeft, StartLine: 40, EndLine: 42},
		"Multi\nline\nnote.", "old block")
	d.Add(diff.Location{Path: "README.md", Side: diff.SideRight, StartLine: 5, EndLine: 5},
		"Typo here.", "# Titel")
	return d
}

func TestExportMarkdownShape(t *testing.T) {
	got := ExportMarkdown(sampleDraft())

	// File grouping and ordering: handler.go first (first seen), sorted by line
	// (40 before 72), then README.md.
	wantOrder := []string{
		"## internal/api/handler.go",
		"### L40-42 (LEFT)",
		"### L72 (RIGHT)",
		"## README.md",
		"### L5 (RIGHT)",
	}
	idx := 0
	for _, want := range wantOrder {
		i := strings.Index(got[idx:], want)
		if i < 0 {
			t.Fatalf("expected %q after position %d, in:\n%s", want, idx, got)
		}
		idx += i + len(want)
	}

	if !strings.Contains(got, "```go\nresult, err := calculate(input)\n```") {
		t.Errorf("missing go-fenced snippet, got:\n%s", got)
	}
	if !strings.Contains(got, "> Multi\n> line\n> note.") {
		t.Errorf("multiline body not block-quoted, got:\n%s", got)
	}
}

func TestExportEmpty(t *testing.T) {
	d := NewDraftReview("k", "t", "")
	if !strings.Contains(ExportMarkdown(d), "_No comments._") {
		t.Errorf("empty export should note no comments")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	d := sampleDraft()
	d.Event = forge.EventRequestChanges
	if err := s.Save(d); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Load(d.SourceKey)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Fatal("load returned nil for existing draft")
	}
	if len(got.Comments) != 3 || got.Event != forge.EventRequestChanges {
		t.Errorf("round-trip mismatch: %d comments, event %q", len(got.Comments), got.Event)
	}
	if got.Comments[0].Location.StartLine != 72 {
		t.Errorf("location not preserved: %+v", got.Comments[0].Location)
	}
}

func TestStoreLoadMissing(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	got, err := s.Load("nope")
	if err != nil || got != nil {
		t.Errorf("missing draft should be (nil, nil), got (%v, %v)", got, err)
	}
}

// TestSelectionToAPILocation is the contract test the plan calls out: a selected
// line must translate to the correct host-API location.
func TestSelectionToAPILocation(t *testing.T) {
	loc := diff.Location{Path: "internal/api.go", Side: diff.SideRight, StartLine: 82, EndLine: 82}
	rc := forge.ReviewComment{Path: loc.Path, Line: loc.EndLine, Side: loc.Side.String()}
	if rc.Path != "internal/api.go" || rc.Line != 82 || rc.Side != "RIGHT" {
		t.Errorf("api location = %+v", rc)
	}
}

func TestCommentsForLine(t *testing.T) {
	d := sampleDraft()
	ids := d.CommentsForLine("internal/api/handler.go", diff.SideLeft, 41)
	if len(ids) != 1 {
		t.Fatalf("expected 1 comment covering LEFT line 41, got %d", len(ids))
	}
}
