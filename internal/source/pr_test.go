package source

import (
	"context"
	"testing"

	"github.com/perrito666/leanreview/internal/forge"
)

// fakeForge is a minimal in-memory Forge for source tests.
func (f *fakeForge) FileContent(context.Context, forge.PullRequestRef, string, string) ([]byte, error) {
	return nil, nil
}

func (f *fakeForge) Attachment(context.Context, forge.PullRequestRef, string) ([]byte, error) {
	return nil, nil
}

type fakeForge struct {
	pr      *forge.PullRequest
	diff    string
	threads []forge.Thread
}

func (f *fakeForge) PullRequest(context.Context, forge.PullRequestRef) (*forge.PullRequest, error) {
	return f.pr, nil
}
func (f *fakeForge) Diff(context.Context, forge.PullRequestRef) ([]byte, error) {
	return []byte(f.diff), nil
}
func (f *fakeForge) Threads(context.Context, forge.PullRequestRef) ([]forge.Thread, error) {
	return f.threads, nil
}
func (f *fakeForge) CreateReview(context.Context, forge.PullRequestRef, forge.ReviewEvent, string, []forge.ReviewComment) (*forge.SubmittedReview, error) {
	return &forge.SubmittedReview{ID: 1}, nil
}
func (f *fakeForge) Reply(context.Context, forge.PullRequestRef, int64, string) (*forge.Comment, error) {
	return &forge.Comment{}, nil
}

const prDiff = `diff --git a/f.go b/f.go
index 1..2 100644
--- a/f.go
+++ b/f.go
@@ -1,2 +1,2 @@
 package p
-var x = 1
+var x = 2
`

func TestPRSource(t *testing.T) {
	ref := forge.PullRequestRef{Host: "github.com", Owner: "o", Repo: "r", Number: 7}
	f := &fakeForge{
		pr:   &forge.PullRequest{Ref: ref, Title: "Fix x", HeadOID: "deadbeef"},
		diff: prDiff,
	}
	s, err := NewPRSource(context.Background(), f, ref)
	if err != nil {
		t.Fatalf("NewPRSource: %v", err)
	}
	if got := s.Title(); got != "o/r#7: Fix x" {
		t.Errorf("title = %q", got)
	}
	if s.HeadOID(context.Background()) != "deadbeef" {
		t.Errorf("headOID mismatch")
	}
	if s.Key() != "gh-github.com-o-r-pr7" {
		t.Errorf("key = %q", s.Key())
	}
	files, err := s.Files(context.Background())
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(files) != 1 || files[0].Path() != "f.go" {
		t.Fatalf("files = %+v", files)
	}
}
