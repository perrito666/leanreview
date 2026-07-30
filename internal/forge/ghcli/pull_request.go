package ghcli

import (
	"context"
	"encoding/json"
	"fmt"

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
