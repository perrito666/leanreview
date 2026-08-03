package glabcli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/perrito666/leanreview/internal/forge"
)

// mrNoteJSON is the subset of a GitLab merge-request note we consume. System
// notes (pushed, marked ready, …) are activity, not conversation, and are
// filtered out.
type mrNoteJSON struct {
	ID     int64 `json:"id"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	Body      string    `json:"body"`
	System    bool      `json:"system"`
	CreatedAt time.Time `json:"created_at"`
}

// AddGeneralComment posts a conversation-level note on the merge request.
func (c *Client) AddGeneralComment(ctx context.Context, ref forge.PullRequestRef, body string) (*forge.Comment, error) {
	path := fmt.Sprintf("projects/%s/merge_requests/%d/notes", projectPath(ref), ref.Number)
	payload, _ := json.Marshal(map[string]string{"body": body})
	out, err := c.run(ctx, payload, apiArgs(ref, path, "--method", "POST", "--input", "-")...)
	if err != nil {
		return nil, err
	}
	var n mrNoteJSON
	if err := json.Unmarshal(out, &n); err != nil {
		return nil, fmt.Errorf("decode note: %w", err)
	}
	return &forge.Comment{ID: n.ID, Author: n.Author.Username, Body: n.Body, CreatedAt: n.CreatedAt}, nil
}

// GeneralComments lists the MR's human conversation notes, oldest first.
func (c *Client) GeneralComments(ctx context.Context, ref forge.PullRequestRef) ([]forge.Comment, error) {
	path := fmt.Sprintf("projects/%s/merge_requests/%d/notes?sort=asc&order_by=created_at", projectPath(ref), ref.Number)
	out, err := c.run(ctx, nil, apiArgs(ref, path)...)
	if err != nil {
		return nil, err
	}
	var notes []mrNoteJSON
	if err := json.Unmarshal(out, &notes); err != nil {
		return nil, fmt.Errorf("decode notes: %w", err)
	}
	var res []forge.Comment
	for _, n := range notes {
		if n.System {
			continue
		}
		res = append(res, forge.Comment{
			ID:        n.ID,
			Author:    n.Author.Username,
			Body:      n.Body,
			CreatedAt: n.CreatedAt,
		})
	}
	return res, nil
}
