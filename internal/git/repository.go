// Package git wraps the installed git executable. We deliberately shell out to
// the user's git rather than reimplementing transport, so their existing
// authentication, remotes, filters, attributes, diff drivers, and config all
// apply. This package owns repository semantics; nothing above it should exec
// git directly.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repository is a handle to a git working tree rooted at Root.
type Repository struct {
	Root string
}

// Open locates the repository that contains dir (or the current directory when
// dir is empty) and returns a handle to it.
func Open(ctx context.Context, dir string) (*Repository, error) {
	out, err := run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a git repository (%s): %w", dirOrCWD(dir), err)
	}
	return &Repository{Root: strings.TrimSpace(string(out))}, nil
}

// HeadOID returns the full object id of HEAD.
func (r *Repository) HeadOID(ctx context.Context) (string, error) {
	out, err := run(ctx, r.Root, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// git runs a git subcommand in the repository root.
func (r *Repository) git(ctx context.Context, args ...string) ([]byte, error) {
	return run(ctx, r.Root, args...)
}

// run executes git with args in dir (or the process CWD when dir is empty,
// which Open needs before a root is known) and returns stdout. Stderr is
// captured separately and folded into the error, because that is where git
// puts its human-readable diagnostics and a bare exit status is useless to
// the user.
func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// dirOrCWD names dir for error messages, substituting "current directory"
// for the empty string so "not a git repository ()" never reaches the user.
func dirOrCWD(dir string) string {
	if dir == "" {
		return "current directory"
	}
	return dir
}

// ShowFile returns the content of path at rev ("" reads the working tree
// file, ":" the index) — the raw material of the full-file context view.
func (r *Repository) ShowFile(ctx context.Context, rev, path string) ([]byte, error) {
	if rev == "" {
		return os.ReadFile(filepath.Join(r.Root, path))
	}
	return r.git(ctx, "show", rev+":"+path)
}

// BlobID returns the object id of path at rev — the content identity the
// file cache keys on, so a change upstream changes the key instead of
// requiring invalidation.
func (r *Repository) BlobID(ctx context.Context, rev, path string) (string, error) {
	out, err := r.git(ctx, "rev-parse", rev+":"+path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
