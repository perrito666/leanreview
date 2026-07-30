package glabcli

import (
	"context"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

// capture records the args (and stdin) of each runner call and returns canned
// output keyed by a substring of the joined args; earlier keys win on ties via
// ordered lookup of the longest match.
type capture struct {
	calls    [][]string
	stdins   [][]byte
	response map[string]string
}

func (c *capture) run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	c.calls = append(c.calls, args)
	c.stdins = append(c.stdins, stdin)
	joined := strings.Join(args, " ")
	best, bestLen := "", -1
	for k := range c.response {
		if strings.Contains(joined, k) && len(k) > bestLen {
			best, bestLen = k, len(k)
		}
	}
	if bestLen >= 0 {
		return []byte(c.response[best]), nil
	}
	return []byte("{}"), nil
}

func (c *capture) lastArgs() string  { return strings.Join(c.calls[len(c.calls)-1], " ") }
func (c *capture) lastStdin() string { return string(c.stdins[len(c.stdins)-1]) }

var ref = forge.PullRequestRef{Host: "gitlab.com", Owner: "group/sub", Repo: "proj", Number: 5}

const mrPayload = `{"iid":5,"title":"Fix thing","author":{"username":"alice"},"sha":"abc123",
  "diff_refs":{"base_sha":"b1","start_sha":"s1","head_sha":"h1"},
  "source_branch":"feature","target_branch":"main","web_url":"https://gitlab.com/group/sub/proj/-/merge_requests/5"}`

func TestPullRequestEncodesProjectPath(t *testing.T) {
	cap := &capture{response: map[string]string{"merge_requests/5": mrPayload}}
	c := NewWithRunner(cap.run)
	pr, err := c.PullRequest(context.Background(), ref)
	if err != nil {
		t.Fatalf("PullRequest: %v", err)
	}
	if pr.Title != "Fix thing" || pr.Author != "alice" || pr.HeadOID != "abc123" || pr.BaseRef != "main" {
		t.Errorf("pr = %+v", pr)
	}
	got := cap.lastArgs()
	if !strings.Contains(got, "projects/group%2Fsub%2Fproj/merge_requests/5") {
		t.Errorf("project path not URL-encoded: %s", got)
	}
	if strings.Contains(got, "--hostname") {
		t.Errorf("gitlab.com should not pass --hostname: %s", got)
	}
}

func TestSelfHostedPassesHostname(t *testing.T) {
	cap := &capture{}
	c := NewWithRunner(cap.run)
	self := ref
	self.Host = "gitlab.example.com"
	_, _ = c.PullRequest(context.Background(), self)
	if got := cap.lastArgs(); !strings.Contains(got, "--hostname gitlab.example.com") {
		t.Errorf("self-hosted host not passed: %s", got)
	}
}

func TestDiffSynthesisParses(t *testing.T) {
	changes := `{"changes":[
	  {"old_path":"a.go","new_path":"a.go","diff":"@@ -1,2 +1,2 @@\n context\n-old\n+new\n"},
	  {"old_path":"born.txt","new_path":"born.txt","new_file":true,"diff":"@@ -0,0 +1,1 @@\n+hello\n"},
	  {"old_path":"gone.txt","new_path":"gone.txt","deleted_file":true,"diff":"@@ -1,1 +0,0 @@\n-bye\n"},
	  {"old_path":"old.go","new_path":"new.go","renamed_file":true,"diff":"@@ -1,1 +1,1 @@\n-x\n+y\n"}
	]}`
	cap := &capture{response: map[string]string{"changes": changes}}
	c := NewWithRunner(cap.run)
	raw, err := c.Diff(context.Background(), ref)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	files, err := diff.ParsePatchBytes(raw)
	if err != nil {
		t.Fatalf("synthesized diff does not parse: %v\n%s", err, raw)
	}
	if len(files) != 4 {
		t.Fatalf("files = %d, want 4", len(files))
	}
	wantStatus := []diff.FileStatus{diff.StatusModified, diff.StatusAdded, diff.StatusDeleted, diff.StatusRenamed}
	for i, w := range wantStatus {
		if files[i].Status != w {
			t.Errorf("file %d status = %v, want %v", i, files[i].Status, w)
		}
	}
}

const discussionsPayload = `[
  {"id":"d1","notes":[
    {"id":100,"body":"added 3 commits","author":{"username":"bot"},"system":true},
    {"id":101,"body":"handle the error","author":{"username":"alice"},
     "position":{"new_path":"a.go","old_path":"a.go","new_line":12}},
    {"id":102,"body":"will do","author":{"username":"bob"}}
  ]},
  {"id":"d2","notes":[
    {"id":200,"body":"old-side note","author":{"username":"carol"},
     "position":{"new_path":"a.go","old_path":"a.go","old_line":3}}
  ]},
  {"id":"d3","notes":[
    {"id":300,"body":"outdated","author":{"username":"dan"},
     "position":{"new_path":"a.go","old_path":"a.go"}}
  ]}
]`

func TestThreadsGrouping(t *testing.T) {
	cap := &capture{response: map[string]string{"discussions": discussionsPayload}}
	c := NewWithRunner(cap.run)
	threads, err := c.Threads(context.Background(), ref)
	if err != nil {
		t.Fatalf("Threads: %v", err)
	}
	if len(threads) != 3 {
		t.Fatalf("threads = %d, want 3", len(threads))
	}
	if threads[0].Root.Body != "handle the error" || len(threads[0].Replies) != 1 {
		t.Errorf("system note not skipped or reply lost: %+v", threads[0])
	}
	if loc := threads[0].Location; loc == nil || loc.Side != diff.SideRight || loc.StartLine != 12 {
		t.Errorf("right-side location = %+v", threads[0].Location)
	}
	if loc := threads[1].Location; loc == nil || loc.Side != diff.SideLeft || loc.StartLine != 3 {
		t.Errorf("left-side location = %+v", threads[1].Location)
	}
	if !threads[2].Outdated {
		t.Errorf("positionless-line thread should be outdated")
	}
}

func TestReplyFindsDiscussion(t *testing.T) {
	cap := &capture{response: map[string]string{
		"discussions --paginate": discussionsPayload,
		"discussions/d2/notes":   `{"id":201,"body":"agreed","author":{"username":"me"}}`,
	}}
	c := NewWithRunner(cap.run)
	cm, err := c.Reply(context.Background(), ref, 200, "agreed")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if cm.ID != 201 || cm.Body != "agreed" {
		t.Errorf("reply = %+v", cm)
	}
	if got := cap.lastArgs(); !strings.Contains(got, "discussions/d2/notes") || !strings.Contains(got, "--method POST") {
		t.Errorf("reply not posted to the containing discussion: %s", got)
	}
	if got := cap.lastStdin(); !strings.Contains(got, `"body":"agreed"`) {
		t.Errorf("reply body not sent: %s", got)
	}
}

func TestReplyUnknownNote(t *testing.T) {
	cap := &capture{response: map[string]string{"discussions": discussionsPayload}}
	c := NewWithRunner(cap.run)
	if _, err := c.Reply(context.Background(), ref, 999, "x"); err == nil {
		t.Errorf("expected an error for a note in no discussion")
	}
}

func TestCreateReviewPostsDiscussionsAndApproves(t *testing.T) {
	cap := &capture{response: map[string]string{"merge_requests/5": mrPayload}}
	c := NewWithRunner(cap.run)
	res, err := c.CreateReview(context.Background(), ref, forge.EventApprove, "nice work", []forge.ReviewComment{
		{Path: "a.go", Body: "right side", Line: 12, Side: "RIGHT"},
		{Path: "a.go", Body: "left side", Line: 3, Side: "LEFT"},
	})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if !strings.Contains(res.URL, "merge_requests/5") {
		t.Errorf("review URL = %q", res.URL)
	}

	var discussions, notes, approves int
	for i, call := range cap.calls {
		joined := strings.Join(call, " ")
		switch {
		case strings.Contains(joined, "/discussions") && strings.Contains(joined, "POST"):
			discussions++
			body := string(cap.stdins[i])
			if !strings.Contains(body, `"base_sha":"b1"`) || !strings.Contains(body, `"position_type":"text"`) {
				t.Errorf("discussion missing diff refs/position: %s", body)
			}
			if strings.Contains(body, "left side") && !strings.Contains(body, `"old_line":3`) {
				t.Errorf("LEFT comment should use old_line: %s", body)
			}
			if strings.Contains(body, "right side") && !strings.Contains(body, `"new_line":12`) {
				t.Errorf("RIGHT comment should use new_line: %s", body)
			}
		case strings.Contains(joined, "/notes") && strings.Contains(joined, "POST"):
			notes++
		case strings.Contains(joined, "/approve"):
			approves++
		}
	}
	if discussions != 2 || notes != 1 || approves != 1 {
		t.Errorf("calls = %d discussions, %d notes, %d approves; want 2/1/1", discussions, notes, approves)
	}
}

func TestRequestChangesPostsPrefixedNote(t *testing.T) {
	cap := &capture{response: map[string]string{"merge_requests/5": mrPayload}}
	c := NewWithRunner(cap.run)
	if _, err := c.CreateReview(context.Background(), ref, forge.EventRequestChanges, "", nil); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if got := cap.lastStdin(); !strings.Contains(got, "Changes requested") {
		t.Errorf("request-changes note missing: %s", got)
	}
	for _, call := range cap.calls {
		if strings.Contains(strings.Join(call, " "), "/approve") {
			t.Errorf("request-changes must not approve")
		}
	}
}
