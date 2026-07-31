package ghcli

import (
	"context"

	"github.com/perrito666/leanreview/internal/forge"
)

// Attachment fetches a comment attachment. gh api handles absolute GitHub
// URLs with the user's authentication (private user-attachments need it);
// anything it cannot serve falls back to a plain, size-capped HTTP fetch —
// public attachment URLs are bearer-secret links that need no auth.
func (c *Client) Attachment(ctx context.Context, _ forge.PullRequestRef, url string) ([]byte, error) {
	if out, err := c.run(ctx, nil, "api", url); err == nil && len(out) > 0 {
		return out, nil
	}
	return forge.FetchAttachmentURL(ctx, url)
}
