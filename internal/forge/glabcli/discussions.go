package glabcli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

// noteJSON is one note (comment) within a GitLab discussion.
type noteJSON struct {
	ID     int64  `json:"id"`
	Body   string `json:"body"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	System    bool      `json:"system"`
	Resolved  bool      `json:"resolved"`
	Position  *posJSON  `json:"position"`
}

type posJSON struct {
	NewPath string `json:"new_path"`
	OldPath string `json:"old_path"`
	NewLine *int   `json:"new_line"`
	OldLine *int   `json:"old_line"`
}

// discussionJSON is a GitLab discussion: a thread of notes keyed by a string id.
type discussionJSON struct {
	ID    string     `json:"id"`
	Notes []noteJSON `json:"notes"`
}

func (n noteJSON) comment() forge.Comment {
	return forge.Comment{
		ID:        n.ID,
		Author:    n.Author.Username,
		Body:      n.Body,
		CreatedAt: n.CreatedAt,
	}
}

func (p *posJSON) location() *diff.Location {
	if p == nil {
		return nil
	}
	switch {
	case p.NewLine != nil:
		return &diff.Location{Path: p.NewPath, Side: diff.SideRight, StartLine: *p.NewLine, EndLine: *p.NewLine}
	case p.OldLine != nil:
		return &diff.Location{Path: p.OldPath, Side: diff.SideLeft, StartLine: *p.OldLine, EndLine: *p.OldLine}
	default:
		return nil
	}
}

func (c *Client) discussions(ctx context.Context, ref forge.PullRequestRef) ([]discussionJSON, error) {
	path := fmt.Sprintf("projects/%s/merge_requests/%d/discussions", projectPath(ref), ref.Number)
	out, err := c.run(ctx, nil, apiArgs(ref, path, "--paginate")...)
	if err != nil {
		return nil, err
	}
	var ds []discussionJSON
	if err := json.Unmarshal(out, &ds); err != nil {
		return nil, fmt.Errorf("decode discussions: %w", err)
	}
	return ds, nil
}

// Threads lists the merge request's discussions as forge threads. System notes
// (e.g. "added 3 commits") are skipped; the first human note of a discussion is
// the root and the rest are replies. A diff position with neither line resolved
// marks the thread outdated.
func (c *Client) Threads(ctx context.Context, ref forge.PullRequestRef) ([]forge.Thread, error) {
	ds, err := c.discussions(ctx, ref)
	if err != nil {
		return nil, err
	}
	var threads []forge.Thread
	for _, d := range ds {
		var th *forge.Thread
		for _, n := range d.Notes {
			if n.System {
				continue
			}
			if th == nil {
				threads = append(threads, forge.Thread{
					Root:     n.comment(),
					Location: n.Position.location(),
					Resolved: n.Resolved,
					Outdated: n.Position != nil && n.Position.NewLine == nil && n.Position.OldLine == nil,
				})
				th = &threads[len(threads)-1]
				continue
			}
			th.Replies = append(th.Replies, n.comment())
		}
	}
	return threads, nil
}

// Reply posts a reply into the discussion that contains the given note.
// GitLab keys replies by discussion id (a string), so the discussion list is
// consulted to find the thread the note belongs to.
func (c *Client) Reply(ctx context.Context, ref forge.PullRequestRef, commentID int64, body string) (*forge.Comment, error) {
	ds, err := c.discussions(ctx, ref)
	if err != nil {
		return nil, err
	}
	discussionID := ""
	for _, d := range ds {
		for _, n := range d.Notes {
			if n.ID == commentID {
				discussionID = d.ID
				break
			}
		}
		if discussionID != "" {
			break
		}
	}
	if discussionID == "" {
		return nil, fmt.Errorf("no discussion contains note %d", commentID)
	}

	payload, _ := json.Marshal(map[string]string{"body": body})
	path := fmt.Sprintf("projects/%s/merge_requests/%d/discussions/%s/notes", projectPath(ref), ref.Number, discussionID)
	out, err := c.run(ctx, payload, apiArgs(ref, path, "--method", "POST", "--input", "-")...)
	if err != nil {
		return nil, err
	}
	var n noteJSON
	if err := json.Unmarshal(out, &n); err != nil {
		return nil, fmt.Errorf("decode reply: %w", err)
	}
	cm := n.comment()
	return &cm, nil
}
