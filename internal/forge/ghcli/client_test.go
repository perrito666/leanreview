package ghcli

import (
	"context"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

// capture records the args (and stdin) of each runner call and returns canned
// output keyed by a substring of the joined args.
type capture struct {
	calls    [][]string
	stdins   [][]byte
	response map[string]string
}

func (c *capture) run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	c.calls = append(c.calls, args)
	c.stdins = append(c.stdins, stdin)
	joined := strings.Join(args, " ")
	for k, v := range c.response {
		if strings.Contains(joined, k) {
			return []byte(v), nil
		}
	}
	return []byte("{}"), nil
}

var ref = forge.PullRequestRef{Host: "github.com", Owner: "perrito666", Repo: "leanreview", Number: 7}

func TestPullRequest(t *testing.T) {
	cap := &capture{response: map[string]string{
		"pulls/7": `{"number":7,"title":"Add thing","user":{"login":"alice"},"head":{"sha":"abc123","ref":"feature"},"base":{"ref":"main"}}`,
	}}
	c := NewWithRunner(cap.run)
	pr, err := c.PullRequest(context.Background(), ref)
	if err != nil {
		t.Fatalf("PullRequest: %v", err)
	}
	if pr.Title != "Add thing" || pr.Author != "alice" || pr.HeadOID != "abc123" || pr.BaseRef != "main" {
		t.Errorf("pr = %+v", pr)
	}
	if got := strings.Join(cap.calls[0], " "); !strings.Contains(got, "api repos/perrito666/leanreview/pulls/7") {
		t.Errorf("unexpected args: %s", got)
	}
}

func TestPullRequestEnterpriseHost(t *testing.T) {
	cap := &capture{}
	c := NewWithRunner(cap.run)
	ghe := ref
	ghe.Host = "ghe.example.com"
	_, _ = c.PullRequest(context.Background(), ghe)
	got := strings.Join(cap.calls[0], " ")
	if !strings.Contains(got, "--hostname ghe.example.com") {
		t.Errorf("enterprise host not passed: %s", got)
	}
}

func TestDiffRequestsDiffMediaType(t *testing.T) {
	cap := &capture{response: map[string]string{"pulls/7": "diff --git a/x b/x\n"}}
	c := NewWithRunner(cap.run)
	out, err := c.Diff(context.Background(), ref)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.HasPrefix(string(out), "diff --git") {
		t.Errorf("diff = %q", out)
	}
	if got := strings.Join(cap.calls[0], " "); !strings.Contains(got, "Accept: application/vnd.github.v3.diff") {
		t.Errorf("diff media type not requested: %s", got)
	}
}

func TestThreadsGrouping(t *testing.T) {
	cap := &capture{response: map[string]string{
		"comments": `[
		  {"id":1,"user":{"login":"a"},"body":"root","path":"f.go","line":10,"side":"RIGHT"},
		  {"id":2,"user":{"login":"b"},"body":"reply","in_reply_to_id":1},
		  {"id":3,"user":{"login":"c"},"body":"outdated","path":"f.go","original_line":5,"side":"RIGHT"}
		]`,
	}}
	c := NewWithRunner(cap.run)
	threads, err := c.Threads(context.Background(), ref)
	if err != nil {
		t.Fatalf("Threads: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("threads = %d, want 2", len(threads))
	}
	if len(threads[0].Replies) != 1 || threads[0].Replies[0].Body != "reply" {
		t.Errorf("root thread replies = %+v", threads[0].Replies)
	}
	if threads[0].Location == nil || threads[0].Location.StartLine != 10 || threads[0].Location.Side != diff.SideRight {
		t.Errorf("root location = %+v", threads[0].Location)
	}
	if !threads[1].Outdated {
		t.Errorf("expected second thread to be outdated")
	}
}

func TestCreateReviewPayload(t *testing.T) {
	cap := &capture{response: map[string]string{"reviews": `{"id":99,"html_url":"http://x/99"}`}}
	c := NewWithRunner(cap.run)
	res, err := c.CreateReview(context.Background(), ref, forge.EventRequestChanges, "please fix", []forge.ReviewComment{
		{Path: "f.go", Body: "bug", Line: 12, Side: "RIGHT"},
		{Path: "f.go", Body: "range", Line: 20, Side: "RIGHT", StartLine: 18, StartSide: "RIGHT"},
	})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if res.ID != 99 {
		t.Errorf("review id = %d", res.ID)
	}
	// The last call carries the JSON body on stdin.
	body := string(cap.stdins[len(cap.stdins)-1])
	if !strings.Contains(body, `"event":"REQUEST_CHANGES"`) {
		t.Errorf("event missing from payload: %s", body)
	}
	if !strings.Contains(body, `"start_line":18`) {
		t.Errorf("multi-line range not encoded: %s", body)
	}
	if got := strings.Join(cap.calls[len(cap.calls)-1], " "); !strings.Contains(got, "--method POST") || !strings.Contains(got, "--input -") {
		t.Errorf("review not sent as POST with stdin input: %s", got)
	}
}

func TestReplyPostsBody(t *testing.T) {
	cap := &capture{response: map[string]string{"replies": `{"id":5,"user":{"login":"me"},"body":"ok"}`}}
	c := NewWithRunner(cap.run)
	cm, err := c.Reply(context.Background(), ref, 1, "ok")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if cm.ID != 5 || cm.Body != "ok" {
		t.Errorf("reply = %+v", cm)
	}
	if got := string(cap.stdins[len(cap.stdins)-1]); !strings.Contains(got, `"body":"ok"`) {
		t.Errorf("reply body not sent: %s", got)
	}
}
