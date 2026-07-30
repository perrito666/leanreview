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
	"os/exec"
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

func dirOrCWD(dir string) string {
	if dir == "" {
		return "current directory"
	}
	return dir
}
