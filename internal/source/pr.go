package source

import (
	"context"
	"fmt"
	"strings"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/git"
)

// InferPRRef fills in a missing owner/repo (and host) from the origin remote of
// the repository at dir, so a bare PR number resolves inside a checkout. It is a
// no-op when owner and repo are already set (e.g. from a full URL).
func InferPRRef(ctx context.Context, dir string, ref forge.PullRequestRef) (forge.PullRequestRef, error) {
	if ref.Owner != "" && ref.Repo != "" {
		return ref, nil
	}
	repo, err := git.Open(ctx, dir)
	if err != nil {
		return ref, fmt.Errorf("cannot infer repository for #%d (not in a git repo); pass owner/repo#%d or a full URL", ref.Number, ref.Number)
	}
	origin, err := repo.OriginRef(ctx)
	if err != nil {
		return ref, fmt.Errorf("cannot read origin remote to infer repository: %w", err)
	}
	ref.Owner = origin.Owner
	ref.Repo = origin.Repo
	if ref.Host == "" {
		ref.Host = origin.Host
	}
	return ref, nil
}

// PRSource is a ReviewSource backed by a forge pull request. It uses the host's
// canonical diff (the representation comments must align with) and exposes the
// forge, ref, and threads so the TUI can reply and submit reviews.
type PRSource struct {
	forge forge.Forge
	ref   forge.PullRequestRef
	pr    *forge.PullRequest
}

// NewPRSource fetches pull-request metadata and returns a ready source. The
// metadata call establishes the title and head commit up front.
func NewPRSource(ctx context.Context, f forge.Forge, ref forge.PullRequestRef) (*PRSource, error) {
	pr, err := f.PullRequest(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("fetch pull request %s: %w", ref, err)
	}
	return &PRSource{forge: f, ref: ref, pr: pr}, nil
}

// Files fetches and parses the canonical PR diff.
func (s *PRSource) Files(ctx context.Context) ([]diff.FileDiff, error) {
	raw, err := s.forge.Diff(ctx, s.ref)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil, nil
	}
	return diff.ParsePatchBytes(raw)
}

// Threads fetches existing review threads for the PR.
func (s *PRSource) Threads(ctx context.Context) ([]forge.Thread, error) {
	return s.forge.Threads(ctx, s.ref)
}

// Forge returns the underlying forge (for replies and submission).
func (s *PRSource) Forge() forge.Forge { return s.forge }

// Ref returns the pull-request reference.
func (s *PRSource) Ref() forge.PullRequestRef { return s.ref }

// PullRequest returns the fetched metadata.
func (s *PRSource) PullRequest() *forge.PullRequest { return s.pr }

func (s *PRSource) Title() string {
	return fmt.Sprintf("%s/%s#%d: %s", s.ref.Owner, s.ref.Repo, s.ref.Number, s.pr.Title)
}

func (s *PRSource) Key() string {
	host := s.ref.Host
	if host == "" {
		host = "github.com"
	}
	owner := strings.ReplaceAll(s.ref.Owner, "/", "-")
	return fmt.Sprintf("gh-%s-%s-%s-pr%d", host, owner, s.ref.Repo, s.ref.Number)
}

func (s *PRSource) HeadOID(context.Context) string {
	if s.pr == nil {
		return ""
	}
	return s.pr.HeadOID
}
