package ghcli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/perrito666/leanreview/internal/forge"
)

// prJSON is the subset of the GitHub pull-request payload we consume.
type prJSON struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		SHA string `json:"sha"`
		Ref string `json:"ref"`
	} `json:"base"`
}

// PullRequest fetches pull-request metadata via `gh api`.
func (c *Client) PullRequest(ctx context.Context, ref forge.PullRequestRef) (*forge.PullRequest, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d", repoPath(ref), ref.Number)
	out, err := c.run(ctx, nil, apiArgs(ref, path)...)
	if err != nil {
		return nil, err
	}
	var p prJSON
	if err := json.Unmarshal(out, &p); err != nil {
		return nil, fmt.Errorf("decode pull request: %w", err)
	}
	return &forge.PullRequest{
		Ref:     ref,
		Title:   p.Title,
		Body:    p.Body,
		Author:  p.User.Login,
		URL:     p.HTMLURL,
		HeadOID: p.Head.SHA,
		BaseOID: p.Base.SHA,
		BaseRef: p.Base.Ref,
		HeadRef: p.Head.Ref,
	}, nil
}

// Diff fetches the canonical PR diff (the representation comments must align
// with) by requesting the diff media type from the pulls endpoint.
func (c *Client) Diff(ctx context.Context, ref forge.PullRequestRef) ([]byte, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d", repoPath(ref), ref.Number)
	out, err := c.run(ctx, nil, apiArgs(ref, path, "-H", "Accept: application/vnd.github.v3.diff")...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FileContent fetches path at rev via the contents API with the raw media
// type, so the bytes arrive verbatim instead of base64-wrapped JSON.
func (c *Client) FileContent(ctx context.Context, ref forge.PullRequestRef, path, rev string) ([]byte, error) {
	api := fmt.Sprintf("repos/%s/contents/%s", repoPath(ref), escapePath(path))
	if rev != "" {
		api += "?ref=" + url.QueryEscape(rev)
	}
	return c.run(ctx, nil, apiArgs(ref, api, "-H", "Accept: application/vnd.github.raw")...)
}

// escapePath escapes each path segment, keeping the separators: the contents
// API addresses files by path inside the URL.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
