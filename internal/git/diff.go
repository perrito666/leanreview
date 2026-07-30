package git

import (
	"context"
	"fmt"
	"strings"
)

// DiffSpec selects which comparison to produce.
type DiffSpec struct {
	// Context is the number of unified context lines (-U). Defaults to 3 when 0.
	Context int
	// Staged compares the index against HEAD (git diff --staged).
	Staged bool
	// Base, when set, compares merge-base(Base, HEAD)..HEAD (git diff Base...HEAD).
	Base string
	// RevA/RevB, when both set, compare two explicit revisions.
	RevA string
	RevB string
}

// Diff runs the appropriate git diff for spec and returns the raw unified patch.
// The flags force a stable, parseable format regardless of user config:
// no color, no external diff drivers, default a/ b/ prefixes, rename detection.
func (r *Repository) Diff(ctx context.Context, spec DiffSpec) ([]byte, error) {
	ctxLines := spec.Context
	if ctxLines == 0 {
		ctxLines = 3
	}
	args := []string{
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--src-prefix=a/",
		"--dst-prefix=b/",
		"-M", // detect renames
		fmt.Sprintf("-U%d", ctxLines),
	}

	switch {
	case spec.RevA != "" && spec.RevB != "":
		args = append(args, spec.RevA, spec.RevB)
	case spec.Base != "":
		args = append(args, fmt.Sprintf("%s...HEAD", spec.Base))
	case spec.Staged:
		args = append(args, "--staged")
	default:
		// Working tree (staged + unstaged) against HEAD.
		args = append(args, "HEAD")
	}

	out, err := r.git(ctx, args...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Title renders a short human description of what a spec compares.
func (s DiffSpec) Title() string {
	switch {
	case s.RevA != "" && s.RevB != "":
		return fmt.Sprintf("%s..%s", s.RevA, s.RevB)
	case s.Base != "":
		return fmt.Sprintf("%s...HEAD", s.Base)
	case s.Staged:
		return "staged changes"
	default:
		return "working tree"
	}
}

// ResolveRev reports whether name resolves to a revision in the repo.
func (r *Repository) ResolveRev(ctx context.Context, name string) bool {
	_, err := r.git(ctx, "rev-parse", "--verify", "--quiet", name+"^{commit}")
	return err == nil
}

// stableKey builds a draft-store key component from a diff spec.
func (s DiffSpec) stableKey() string {
	return strings.ReplaceAll(s.Title(), "/", "-")
}
