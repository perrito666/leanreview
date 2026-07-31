package glabcli

import (
	"context"
	"fmt"
	"strings"

	"github.com/perrito666/leanreview/internal/forge"
)

// Attachment fetches a comment attachment. GitLab embeds uploads as
// project-relative /uploads/<secret>/<file> paths, served by the project
// uploads API with the user's authentication; absolute URLs fall back to a
// plain, size-capped HTTP fetch.
func (c *Client) Attachment(ctx context.Context, ref forge.PullRequestRef, url string) ([]byte, error) {
	if strings.HasPrefix(url, "/uploads/") {
		api := fmt.Sprintf("projects/%s%s", projectPath(ref), url)
		return c.run(ctx, nil, apiArgs(ref, api)...)
	}
	if out, err := c.run(ctx, nil, "api", url); err == nil && len(out) > 0 {
		return out, nil
	}
	return forge.FetchAttachmentURL(ctx, url)
}
