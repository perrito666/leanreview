package git

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// RemoteRef identifies the host, owner, and repository of a git remote.
type RemoteRef struct {
	Host  string
	Owner string
	Repo  string
}

// OriginRef returns the parsed "origin" remote of the repository.
func (r *Repository) OriginRef(ctx context.Context) (RemoteRef, error) {
	return r.RemoteRef(ctx, "origin")
}

// RemoteRef returns the parsed URL of the named remote.
func (r *Repository) RemoteRef(ctx context.Context, name string) (RemoteRef, error) {
	out, err := r.git(ctx, "remote", "get-url", name)
	if err != nil {
		return RemoteRef{}, err
	}
	return ParseRemoteURL(strings.TrimSpace(string(out)))
}

var (
	// scpLike matches "git@host:owner/repo(.git)".
	scpLike = regexp.MustCompile(`^(?:([^@]+)@)?([^:/]+):(.+?)(?:\.git)?$`)
	// urlLike matches ssh://, https://, git:// style URLs.
	urlLike = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://(?:[^@/]+@)?([^:/]+)(?::\d+)?/(.+?)(?:\.git)?$`)
)

// ParseRemoteURL extracts host/owner/repo from a git remote URL. It accepts
// scp-like SSH ("git@github.com:owner/repo.git"), ssh:// URLs, and https:// URLs
// (with or without a trailing ".git"). The owner is the path up to the last
// segment, supporting nested groups (e.g. GitLab subgroups).
func ParseRemoteURL(raw string) (RemoteRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RemoteRef{}, fmt.Errorf("empty remote URL")
	}

	var host, path string
	if m := urlLike.FindStringSubmatch(raw); m != nil {
		host, path = m[1], m[2]
	} else if m := scpLike.FindStringSubmatch(raw); m != nil {
		host, path = m[2], m[3]
	} else {
		return RemoteRef{}, fmt.Errorf("unrecognised remote URL: %s", raw)
	}

	path = strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return RemoteRef{}, fmt.Errorf("remote URL has no owner/repo: %s", raw)
	}
	owner := path[:idx]
	repo := path[idx+1:]
	if owner == "" || repo == "" {
		return RemoteRef{}, fmt.Errorf("remote URL has empty owner or repo: %s", raw)
	}
	return RemoteRef{Host: host, Owner: owner, Repo: repo}, nil
}
