package glabcli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/perrito666/leanreview/internal/forge"
)

// UploadAttachment sends a local file to the project's Markdown uploads
// endpoint (multipart via glab's --form) and returns the project-relative
// /uploads reference GitLab resolves inside MR notes and descriptions —
// exactly the form the attachment fetcher already knows how to read back.
func (c *Client) UploadAttachment(ctx context.Context, ref forge.PullRequestRef, path string) (string, error) {
	apiPath := fmt.Sprintf("projects/%s/uploads", projectPath(ref))
	out, err := c.run(ctx, nil, apiArgs(ref, apiPath, "--method", "POST", "--form", "file=@"+path)...)
	if err != nil {
		return "", err
	}
	var up struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(out, &up); err != nil {
		return "", fmt.Errorf("decode upload: %w", err)
	}
	if up.URL == "" {
		return "", fmt.Errorf("upload returned no url")
	}
	return up.URL, nil
}
