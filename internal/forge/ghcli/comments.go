package ghcli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

// reviewCommentJSON is the subset of a GitHub review comment we consume.
type reviewCommentJSON struct {
	ID           int64                  `json:"id"`
	User         struct{ Login string } `json:"user"`
	Body         string                 `json:"body"`
	CreatedAt    time.Time              `json:"created_at"`
	HTMLURL      string                 `json:"html_url"`
	Path         string                 `json:"path"`
	Line         *int                   `json:"line"`
	OriginalLine *int                   `json:"original_line"`
	StartLine    *int                   `json:"start_line"`
	Side         string                 `json:"side"`
	InReplyToID  *int64                 `json:"in_reply_to_id"`
	Position     *int                   `json:"position"`
}

func (rc reviewCommentJSON) comment() forge.Comment {
	return forge.Comment{
		ID:        rc.ID,
		Author:    rc.User.Login,
		Body:      rc.Body,
		CreatedAt: rc.CreatedAt,
		URL:       rc.HTMLURL,
	}
}

func (rc reviewCommentJSON) location() *diff.Location {
	if rc.Path == "" {
		return nil
	}
	side := diff.SideRight
	if rc.Side == "LEFT" {
		side = diff.SideLeft
	}
	line := 0
	switch {
	case rc.Line != nil:
		line = *rc.Line
	case rc.OriginalLine != nil:
		line = *rc.OriginalLine
	}
	loc := &diff.Location{Path: rc.Path, Side: side, StartLine: line, EndLine: line}
	if rc.StartLine != nil {
		loc.StartLine = *rc.StartLine
	}
	return loc
}

// Threads lists the PR's review comments and groups them into threads: root
// comments (those not replying to another) with their replies attached. The
// GitHub API is "outdated" when a comment's line no longer maps to the current
// diff (line is null but original_line is set).
func (c *Client) Threads(ctx context.Context, ref forge.PullRequestRef) ([]forge.Thread, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/comments", repoPath(ref), ref.Number)
	out, err := c.run(ctx, nil, apiArgs(ref, path, "--paginate")...)
	if err != nil {
		return nil, err
	}
	var comments []reviewCommentJSON
	if err := json.Unmarshal(out, &comments); err != nil {
		return nil, fmt.Errorf("decode review comments: %w", err)
	}

	var threads []forge.Thread
	index := map[int64]int{} // root comment id -> index in threads
	for _, rc := range comments {
		if rc.InReplyToID == nil {
			threads = append(threads, forge.Thread{
				Root:     rc.comment(),
				Location: rc.location(),
				Outdated: rc.Line == nil && rc.OriginalLine != nil,
			})
			index[rc.ID] = len(threads) - 1
			continue
		}
		if ti, ok := index[*rc.InReplyToID]; ok {
			threads[ti].Replies = append(threads[ti].Replies, rc.comment())
		} else {
			// Reply whose root wasn't seen (paging edge): start a thread for it.
			threads = append(threads, forge.Thread{Root: rc.comment(), Location: rc.location()})
			index[rc.ID] = len(threads) - 1
		}
	}
	return threads, nil
}

// Reply posts a reply to an existing review comment.
func (c *Client) Reply(ctx context.Context, ref forge.PullRequestRef, commentID int64, body string) (*forge.Comment, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/comments/%d/replies", repoPath(ref), ref.Number, commentID)
	payload, _ := json.Marshal(map[string]string{"body": body})
	out, err := c.run(ctx, payload, apiArgs(ref, path, "--method", "POST", "--input", "-")...)
	if err != nil {
		return nil, err
	}
	var rc reviewCommentJSON
	if err := json.Unmarshal(out, &rc); err != nil {
		return nil, fmt.Errorf("decode reply: %w", err)
	}
	cm := rc.comment()
	return &cm, nil
}
