package ghcli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/perrito666/leanreview/internal/forge"
)

// reviewPayload is the JSON body for creating a pull-request review.
type reviewPayload struct {
	Event    string                 `json:"event"`
	Body     string                 `json:"body,omitempty"`
	Comments []reviewCommentPayload `json:"comments,omitempty"`
}

type reviewCommentPayload struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Line      int    `json:"line"`
	Side      string `json:"side,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
}

// CreateReview submits all comments as one review with the given event,
// grouping them atomically rather than posting unrelated immediate comments.
func (c *Client) CreateReview(ctx context.Context, ref forge.PullRequestRef, event forge.ReviewEvent, summary string, comments []forge.ReviewComment) (*forge.SubmittedReview, error) {
	payload := reviewPayload{Event: string(event), Body: summary}
	for _, rc := range comments {
		p := reviewCommentPayload{
			Path: rc.Path,
			Body: rc.Body,
			Line: rc.Line,
			Side: rc.Side,
		}
		if rc.StartLine > 0 && rc.StartLine != rc.Line {
			p.StartLine = rc.StartLine
			p.StartSide = rc.StartSide
			if p.StartSide == "" {
				p.StartSide = rc.Side
			}
		}
		payload.Comments = append(payload.Comments, p)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode review: %w", err)
	}
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews", repoPath(ref), ref.Number)
	out, err := c.run(ctx, body, apiArgs(ref, path, "--method", "POST", "--input", "-")...)
	if err != nil {
		return nil, err
	}
	var res struct {
		ID      int64  `json:"id"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("decode review response: %w", err)
	}
	return &forge.SubmittedReview{ID: res.ID, URL: res.HTMLURL}, nil
}
