package forge

import (
	"context"
	"time"
)

// ListedRequest is one pull/merge request returned by a discovery listing.
type ListedRequest struct {
	Ref       PullRequestRef
	Title     string
	Author    string
	UpdatedAt time.Time
	URL       string
}

// Lister discovers open pull/merge requests matching a filter. The filter
// string is engine-specific: a GitHub search query for the gh engine
// ("is:open review-requested:@me author:x"), a REST query string for the glab
// engine ("state=opened&reviewer_username=@me"). An empty filter applies the
// engine's default.
type Lister interface {
	List(ctx context.Context, filter string) ([]ListedRequest, error)
}
