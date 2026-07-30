// Package forge defines the seam between the review UI and a code-hosting
// service (GitHub, later GitLab/Forgejo). The TUI depends only on the Forge
// interface and the value types here; concrete adapters (e.g. one that shells
// out to `gh`) live in subpackages and are wired in from cmd. In Milestone 1
// no adapter exists yet — only the seam and pull-request reference parsing,
// which the source resolver uses to recognise (and politely decline) PR refs.
package forge

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/perrito666/leanreview/internal/diff"
)

// PullRequestRef identifies a pull request on a host.
type PullRequestRef struct {
	Host   string // e.g. "github.com"
	Owner  string
	Repo   string
	Number int
}

func (r PullRequestRef) String() string {
	host := r.Host
	if host == "" {
		host = "github.com"
	}
	return fmt.Sprintf("%s/%s/%s#%d", host, r.Owner, r.Repo, r.Number)
}

// PullRequest is the metadata the UI needs about a PR.
type PullRequest struct {
	Ref     PullRequestRef
	Title   string
	Author  string
	HeadOID string
	BaseRef string
	HeadRef string
}

// Comment is a single review comment as returned by the host.
type Comment struct {
	ID        int64
	Author    string
	Body      string
	CreatedAt time.Time
	URL       string
}

// Thread groups a root review comment with its replies and host-side state.
type Thread struct {
	Root     Comment
	Replies  []Comment
	Resolved bool
	Outdated bool
	Location *diff.Location
}

// ReviewEvent is the action taken when submitting a review.
type ReviewEvent string

const (
	EventComment        ReviewEvent = "COMMENT"
	EventApprove        ReviewEvent = "APPROVE"
	EventRequestChanges ReviewEvent = "REQUEST_CHANGES"
)

// SubmittedReview is the host's acknowledgement of a created review.
type SubmittedReview struct {
	ID  int64
	URL string
}

// Forge is the host-agnostic contract the UI submits reviews through. The UI
// must not know whether the implementation uses gh, REST, GraphQL, github.com,
// or an enterprise host.
type Forge interface {
	PullRequest(ctx context.Context, ref PullRequestRef) (*PullRequest, error)
	Diff(ctx context.Context, ref PullRequestRef) ([]byte, error)
	Threads(ctx context.Context, ref PullRequestRef) ([]Thread, error)
	CreateReview(ctx context.Context, ref PullRequestRef, event ReviewEvent, summary string, comments []ReviewComment) (*SubmittedReview, error)
	Reply(ctx context.Context, ref PullRequestRef, commentID int64, body string) (*Comment, error)
}

// ReviewComment is a single line comment expressed in host API terms.
type ReviewComment struct {
	Path      string
	Body      string
	Line      int
	Side      string // "LEFT" or "RIGHT"
	StartLine int    // 0 when single-line
	StartSide string
}

var (
	urlRe   = regexp.MustCompile(`^https?://([^/]+)/([^/]+)/([^/]+)/pull/(\d+)`)
	shortRe = regexp.MustCompile(`^([^/#\s]+)/([^/#\s]+)#(\d+)$`)
	numRe   = regexp.MustCompile(`^#?(\d+)$`)
)

// ParseRef recognises a pull-request reference in any of these shapes:
//
//	https://github.com/owner/repo/pull/418
//	owner/repo#418
//	418  (or #418)  — owner/repo must be supplied separately (e.g. inferred
//	                  from the origin remote), so Owner/Repo are left empty.
//
// It returns ok=false when s is not a PR reference at all (e.g. a file path).
func ParseRef(s string) (ref PullRequestRef, ok bool) {
	s = strings.TrimSpace(s)
	if m := urlRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[4])
		return PullRequestRef{Host: m[1], Owner: m[2], Repo: strings.TrimSuffix(m[3], ".git"), Number: n}, true
	}
	if m := shortRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[3])
		return PullRequestRef{Host: "github.com", Owner: m[1], Repo: m[2], Number: n}, true
	}
	if m := numRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		return PullRequestRef{Host: "github.com", Number: n}, true
	}
	return PullRequestRef{}, false
}
