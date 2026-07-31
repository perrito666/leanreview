package ghcli

import (
	"context"
	"strings"

	"github.com/perrito666/leanreview/internal/forge"
)

// Attachment fetches a comment attachment. Signed asset URLs (what
// resolveAttachments substitutes into bodies) and other public links are
// plain, size-capped fetches; only api.* URLs go through gh, which attaches
// credentials. Deliberately NOT gh for github.com/user-attachments URLs:
// those answer API credentials with an HTML viewer page, not the asset.
func (c *Client) Attachment(ctx context.Context, _ forge.PullRequestRef, url string) ([]byte, error) {
	if strings.HasPrefix(url, "https://api.") {
		return c.run(ctx, nil, "api", url)
	}
	return forge.FetchAttachmentURL(ctx, url)
}
