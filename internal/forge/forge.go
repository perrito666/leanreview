// Package forge defines the seam between the review UI and a code-hosting
// service. The TUI depends only on the Forge interface and the value types
// here; the concrete adapters — ghcli shelling out to `gh`, glabcli to
// `glab` — live in subpackages and are wired in from cmd, keeping the UI
// ignorant of which host (or API style) it is talking to.
package forge

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/perrito666/leanreview/internal/diff"
)

// Kind identifies which family of forge a host belongs to, so the CLI can pick
// the matching adapter.
type Kind uint8

const (
	KindGitHub Kind = iota
	KindGitLab
)

// String returns the short forge name, matching the discovery engine names
// ("gh", "glab") so the UI badge and the --list engine filter agree.
func (k Kind) String() string {
	if k == KindGitLab {
		return "glab"
	}
	return "gh"
}

// KindForHost guesses the forge kind from a host name. gitlab.com and any host
// containing "gitlab" (the common self-hosted convention) map to GitLab;
// everything else — including GitHub Enterprise hosts and an empty host —
// defaults to GitHub. Hosts that do not follow the convention can be supported
// later via configuration.
func KindForHost(host string) Kind {
	h := strings.ToLower(host)
	if h == "gitlab.com" || strings.Contains(h, "gitlab") {
		return KindGitLab
	}
	return KindGitHub
}

// PullRequestRef identifies a pull request (or GitLab merge request) on a host.
type PullRequestRef struct {
	Host   string // e.g. "github.com"
	Owner  string
	Repo   string
	Number int
}

// String renders the canonical short form "host/owner/repo#N" — with GitLab's
// "!" separator when the host looks like a GitLab instance — as used in error
// messages and the discovery picker. An empty host prints as github.com,
// matching ParseRef's default.
func (r PullRequestRef) String() string {
	host := r.Host
	if host == "" {
		host = "github.com"
	}
	sep := "#"
	if KindForHost(host) == KindGitLab {
		sep = "!"
	}
	return fmt.Sprintf("%s/%s/%s%s%d", host, r.Owner, r.Repo, sep, r.Number)
}

// WebURL returns the browsable page for the pull request on its host, used as
// a fallback when the adapter did not report one.
func (r PullRequestRef) WebURL() string {
	host := r.Host
	if host == "" {
		host = "github.com"
	}
	if KindForHost(host) == KindGitLab {
		return fmt.Sprintf("https://%s/%s/%s/-/merge_requests/%d", host, r.Owner, r.Repo, r.Number)
	}
	return fmt.Sprintf("https://%s/%s/%s/pull/%d", host, r.Owner, r.Repo, r.Number)
}

// PullRequest is the metadata the UI needs about a PR.
type PullRequest struct {
	Ref     PullRequestRef
	Title   string
	Body    string // description, as Markdown
	Author  string
	URL     string // browsable page; empty when the adapter did not report one
	HeadOID string
	// BaseOID is the base commit the diff is against, when the host reports
	// it — the revision old-side file content lives at.
	BaseOID string
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
	// FileContent returns the raw content of path at rev (a commit id,
	// typically the PR head) — the full-file context view's data source.
	FileContent(ctx context.Context, ref PullRequestRef, path, rev string) ([]byte, error)
	// Attachment fetches an image/file referenced from a comment body (an
	// absolute attachment URL, or a host-relative /uploads path), using the
	// adapter's authentication where the host requires it.
	Attachment(ctx context.Context, ref PullRequestRef, url string) ([]byte, error)
}

// maxAttachmentBytes caps comment-attachment downloads: a preview is not
// worth an unbounded pull.
const maxAttachmentBytes = 10 << 20

// FetchAttachmentURL is the adapters' shared plain-HTTP path for attachment
// URLs that need no API authentication (or as a fallback when the CLI cannot
// serve them). Size-capped and context-bound.
func FetchAttachmentURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAttachmentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAttachmentBytes {
		return nil, fmt.Errorf("attachment exceeds %d bytes", maxAttachmentBytes)
	}
	return data, nil
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
	glURLRe = regexp.MustCompile(`^https?://([^/]+)/(.+)/-/merge_requests/(\d+)`)
	shortRe = regexp.MustCompile(`^([^/#\s]+)/([^/#\s]+)#(\d+)$`)
	bangRe  = regexp.MustCompile(`^([^!#\s]+)!(\d+)$`)
	numRe   = regexp.MustCompile(`^[#!]?(\d+)$`)
)

// ParseRef recognises a pull/merge-request reference in any of these shapes:
//
//	https://github.com/owner/repo/pull/418
//	https://gitlab.com/group/subgroup/repo/-/merge_requests/418
//	owner/repo#418          (GitHub-style short form)
//	group/repo!418          (GitLab-style short form, nested groups allowed)
//	418  (or #418, !418)    — owner/repo (and host) must be supplied separately,
//	                          e.g. inferred from the origin remote, so Host,
//	                          Owner, and Repo are left empty.
//
// It returns ok=false when s is not a reference at all (e.g. a file path).
func ParseRef(s string) (ref PullRequestRef, ok bool) {
	s = strings.TrimSpace(s)
	if m := urlRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[4])
		return PullRequestRef{Host: m[1], Owner: m[2], Repo: strings.TrimSuffix(m[3], ".git"), Number: n}, true
	}
	if m := glURLRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[3])
		owner, repo, ok := splitProjectPath(m[2])
		if !ok {
			return PullRequestRef{}, false
		}
		return PullRequestRef{Host: m[1], Owner: owner, Repo: repo, Number: n}, true
	}
	if m := shortRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[3])
		return PullRequestRef{Host: "github.com", Owner: m[1], Repo: m[2], Number: n}, true
	}
	if m := bangRe.FindStringSubmatch(s); m != nil {
		if owner, repo, pathOK := splitProjectPath(m[1]); pathOK {
			n, _ := strconv.Atoi(m[2])
			return PullRequestRef{Host: "gitlab.com", Owner: owner, Repo: repo, Number: n}, true
		}
	}
	if m := numRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		// Host is left empty so the caller can infer it (with owner/repo) from
		// the origin remote; an empty host behaves as github.com by default.
		return PullRequestRef{Number: n}, true
	}
	return PullRequestRef{}, false
}

// splitProjectPath splits "group/subgroup/repo" into owner ("group/subgroup")
// and repo ("repo"). ok is false when there is no "/" at all.
func splitProjectPath(path string) (owner, repo string, ok bool) {
	path = strings.Trim(path, "/")
	i := strings.LastIndex(path, "/")
	if i <= 0 || i == len(path)-1 {
		return "", "", false
	}
	return path[:i], path[i+1:], true
}
