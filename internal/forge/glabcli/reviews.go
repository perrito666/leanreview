package glabcli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/perrito666/leanreview/internal/forge"
)

// discussionPayload is the JSON body for creating a positioned diff discussion.
type discussionPayload struct {
	Body     string           `json:"body"`
	Position *positionPayload `json:"position,omitempty"`
}

type positionPayload struct {
	PositionType string `json:"position_type"`
	BaseSHA      string `json:"base_sha"`
	StartSHA     string `json:"start_sha"`
	HeadSHA      string `json:"head_sha"`
	OldPath      string `json:"old_path"`
	NewPath      string `json:"new_path"`
	NewLine      int    `json:"new_line,omitempty"`
	OldLine      int    `json:"old_line,omitempty"`
}

// CreateReview submits the review to GitLab. GitLab's REST API has no atomic
// review-with-comments endpoint (that is a GitHub concept), so this maps the
// forge contract onto GitLab's model: each line comment becomes a positioned
// diff discussion, the summary becomes a merge-request note, and the event maps
// to an approval (APPROVE) or a "Changes requested" note (REQUEST_CHANGES).
// Comments are posted in order; on the first failure the error reports how many
// were already published so the reviewer is not left guessing.
func (c *Client) CreateReview(ctx context.Context, ref forge.PullRequestRef, event forge.ReviewEvent, summary string, comments []forge.ReviewComment) (*forge.SubmittedReview, error) {
	mr, err := c.mr(ctx, ref)
	if err != nil {
		return nil, err
	}

	for i, rc := range comments {
		pos := &positionPayload{
			PositionType: "text",
			BaseSHA:      mr.DiffRefs.BaseSHA,
			StartSHA:     mr.DiffRefs.StartSHA,
			HeadSHA:      mr.DiffRefs.HeadSHA,
			OldPath:      rc.Path,
			NewPath:      rc.Path,
		}
		// GitLab positions a discussion on a single line; for a multi-line
		// selection the end line anchors the discussion (the body still quotes
		// the full range).
		if rc.Side == "LEFT" {
			pos.OldLine = rc.Line
		} else {
			pos.NewLine = rc.Line
		}
		body, err := json.Marshal(discussionPayload{Body: rc.Body, Position: pos})
		if err != nil {
			return nil, fmt.Errorf("encode discussion: %w", err)
		}
		path := fmt.Sprintf("projects/%s/merge_requests/%d/discussions", projectPath(ref), ref.Number)
		if _, err := c.run(ctx, body, apiArgs(ref, path, "--method", "POST", "--input", "-")...); err != nil {
			return nil, fmt.Errorf("posted %d of %d comments, then: %w", i, len(comments), err)
		}
	}

	note := summary
	if event == forge.EventRequestChanges {
		if note == "" {
			note = "**Changes requested.**"
		} else {
			note = "**Changes requested.**\n\n" + note
		}
	}
	if note != "" {
		body, _ := json.Marshal(map[string]string{"body": note})
		path := fmt.Sprintf("projects/%s/merge_requests/%d/notes", projectPath(ref), ref.Number)
		if _, err := c.run(ctx, body, apiArgs(ref, path, "--method", "POST", "--input", "-")...); err != nil {
			return nil, fmt.Errorf("comments posted, but the summary note failed: %w", err)
		}
	}

	if event == forge.EventApprove {
		path := fmt.Sprintf("projects/%s/merge_requests/%d/approve", projectPath(ref), ref.Number)
		if _, err := c.run(ctx, nil, apiArgs(ref, path, "--method", "POST")...); err != nil {
			return nil, fmt.Errorf("comments posted, but approval failed: %w", err)
		}
	}

	return &forge.SubmittedReview{URL: mr.WebURL}, nil
}
