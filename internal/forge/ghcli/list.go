package ghcli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/perrito666/leanreview/internal/forge"
)

// DefaultListFilter is the gh engine's default search: open pull requests
// waiting for the authenticated user's review.
const DefaultListFilter = "is:open review-requested:@me"

// listLimit bounds how many results a discovery listing returns.
const listLimit = 30

var _ forge.Lister = (*Client)(nil)

// searchPRJSON is the subset of `gh search prs --json` output we consume.
type searchPRJSON struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	UpdatedAt time.Time `json:"updatedAt"`
	URL       string    `json:"url"`
}

// List discovers pull requests via `gh search prs`. The filter is a GitHub
// search query (qualifiers like "review-requested:@me", "author:x",
// "repo:owner/name"); an empty filter applies DefaultListFilter.
func (c *Client) List(ctx context.Context, filter string) ([]forge.ListedRequest, error) {
	if strings.TrimSpace(filter) == "" {
		filter = DefaultListFilter
	}
	args := []string{"search", "prs"}
	args = append(args, strings.Fields(filter)...)
	args = append(args,
		"--json", "number,title,author,repository,updatedAt,url",
		"--limit", fmt.Sprint(listLimit),
	)
	out, err := c.run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	var prs []searchPRJSON
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("decode search results: %w", err)
	}

	res := make([]forge.ListedRequest, 0, len(prs))
	for _, p := range prs {
		ref, ok := forge.ParseRef(p.URL)
		if !ok {
			// Fall back to nameWithOwner when the URL shape is unexpected.
			owner, repo, pathOK := splitOwnerRepo(p.Repository.NameWithOwner)
			if !pathOK {
				continue
			}
			ref = forge.PullRequestRef{Host: "github.com", Owner: owner, Repo: repo, Number: p.Number}
		}
		res = append(res, forge.ListedRequest{
			Ref:       ref,
			Title:     p.Title,
			Author:    p.Author.Login,
			UpdatedAt: p.UpdatedAt,
			URL:       p.URL,
		})
	}
	return res, nil
}

// splitOwnerRepo splits a "nameWithOwner" string at its last "/" into owner
// and repo; ok is false when either side would be empty. It only backs up
// ParseRef when a search result's URL has an unexpected shape.
func splitOwnerRepo(s string) (owner, repo string, ok bool) {
	i := strings.LastIndex(s, "/")
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}
