package glabcli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/perrito666/leanreview/internal/forge"
)

// DefaultListFilter is the glab engine's default listing: open merge requests
// waiting for the authenticated user's review. "@me" is substituted with the
// authenticated username before the request.
const DefaultListFilter = "state=opened&reviewer_username=@me"

// listLimit bounds how many results a discovery listing returns.
const listLimit = 30

var _ forge.Lister = (*Client)(nil)

// listedMRJSON is the subset of the merge-request listing payload we consume.
type listedMRJSON struct {
	IID    int    `json:"iid"`
	Title  string `json:"title"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	UpdatedAt time.Time `json:"updated_at"`
	WebURL    string    `json:"web_url"`
}

// List discovers merge requests via the global merge_requests endpoint. The
// filter is a REST query string ("state=opened&labels=x"); an empty filter
// applies DefaultListFilter, and any "@me" value is replaced with the
// authenticated user's name.
func (c *Client) List(ctx context.Context, filter string) ([]forge.ListedRequest, error) {
	if strings.TrimSpace(filter) == "" {
		filter = DefaultListFilter
	}
	if strings.Contains(filter, "@me") {
		me, err := c.username(ctx)
		if err != nil {
			return nil, err
		}
		filter = strings.ReplaceAll(filter, "@me", me)
	}

	path := fmt.Sprintf("merge_requests?%s&per_page=%d", filter, listLimit)
	out, err := c.run(ctx, nil, "api", path)
	if err != nil {
		return nil, err
	}
	var mrs []listedMRJSON
	if err := json.Unmarshal(out, &mrs); err != nil {
		return nil, fmt.Errorf("decode merge-request listing: %w", err)
	}

	res := make([]forge.ListedRequest, 0, len(mrs))
	for _, m := range mrs {
		ref, ok := forge.ParseRef(m.WebURL)
		if !ok {
			continue
		}
		res = append(res, forge.ListedRequest{
			Ref:       ref,
			Title:     m.Title,
			Author:    m.Author.Username,
			UpdatedAt: m.UpdatedAt,
			URL:       m.WebURL,
		})
	}
	return res, nil
}

// username resolves the authenticated user's name via `glab api user`.
func (c *Client) username(ctx context.Context) (string, error) {
	out, err := c.run(ctx, nil, "api", "user")
	if err != nil {
		return "", err
	}
	var u struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(out, &u); err != nil {
		return "", fmt.Errorf("decode user: %w", err)
	}
	if u.Username == "" {
		return "", fmt.Errorf("could not resolve the authenticated GitLab user")
	}
	return u.Username, nil
}
