package ghcli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/perrito666/leanreview/internal/forge"
)

// issueCommentJSON is the subset of a GitHub issue (conversation) comment we
// consume. body_html rides along for the same reason as review comments:
// resolving session-gated attachment URLs to signed ones.
type issueCommentJSON struct {
	ID        int64                  `json:"id"`
	User      struct{ Login string } `json:"user"`
	Body      string                 `json:"body"`
	BodyHTML  string                 `json:"body_html"`
	CreatedAt time.Time              `json:"created_at"`
	HTMLURL   string                 `json:"html_url"`
}

// AddGeneralComment posts a conversation-level comment through the issues
// endpoint — the API surface general PR comments actually live on.
func (c *Client) AddGeneralComment(ctx context.Context, ref forge.PullRequestRef, body string) (*forge.Comment, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/comments", repoPath(ref), ref.Number)
	payload, _ := json.Marshal(map[string]string{"body": body})
	out, err := c.run(ctx, payload, apiArgs(ref, path, "--method", "POST", "--input", "-")...)
	if err != nil {
		return nil, err
	}
	var ic issueCommentJSON
	if err := json.Unmarshal(out, &ic); err != nil {
		return nil, fmt.Errorf("decode general comment: %w", err)
	}
	return &forge.Comment{ID: ic.ID, Author: ic.User.Login, Body: ic.Body, CreatedAt: ic.CreatedAt}, nil
}

// GeneralComments lists the PR's conversation comments. GitHub models them
// as issue comments — a PR is an issue with code attached — so this is the
// issues endpoint, not pulls.
func (c *Client) GeneralComments(ctx context.Context, ref forge.PullRequestRef) ([]forge.Comment, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/comments", repoPath(ref), ref.Number)
	out, err := c.run(ctx, nil, apiArgs(ref, path, "--paginate", "-H", "Accept: application/vnd.github.full+json")...)
	if err != nil {
		return nil, err
	}
	var comments []issueCommentJSON
	if err := json.Unmarshal(out, &comments); err != nil {
		return nil, fmt.Errorf("decode issue comments: %w", err)
	}
	res := make([]forge.Comment, 0, len(comments))
	for _, ic := range comments {
		res = append(res, forge.Comment{
			ID:        ic.ID,
			Author:    ic.User.Login,
			Body:      resolveAttachments(ic.Body, ic.BodyHTML),
			CreatedAt: ic.CreatedAt,
			URL:       ic.HTMLURL,
		})
	}
	return res, nil
}
