// Package source resolves command-line arguments into a ReviewSource: something
// that yields a parsed set of file diffs, a human title, and a stable key for
// draft persistence. It unifies the patch-file / stdin path with the local-git
// path, and recognises (but, in Milestone 1, declines) GitHub PR references.
package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/git"
)

// ReviewSource is a resolved, ready-to-load set of changes under review.
type ReviewSource interface {
	// Files parses and returns the diff for this source.
	Files(ctx context.Context) ([]diff.FileDiff, error)
	// Title is a short human description shown in the status bar.
	Title() string
	// Key is a stable identifier used to namespace persisted drafts.
	Key() string
	// HeadOID returns the head commit the source was captured against, or "".
	HeadOID(ctx context.Context) string
}

// Options are the parsed CLI inputs used to resolve a source.
type Options struct {
	Args    []string  // positional arguments
	Base    string    // --base <ref>
	Staged  bool      // --staged
	Context int       // -U context lines
	Stdin   io.Reader // stdin, used when the argument is "-"
	Dir     string    // working directory (defaults to process CWD)
}

// Resolve inspects opts and returns the appropriate ReviewSource.
func Resolve(ctx context.Context, opts Options) (ReviewSource, error) {
	dir := opts.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// Explicit two-revision comparison: needs a repository.
	if len(opts.Args) == 2 {
		return newGitRevSource(ctx, dir, opts.Args[0], opts.Args[1], opts.Context)
	}

	arg := ""
	if len(opts.Args) == 1 {
		arg = opts.Args[0]
	}

	switch {
	case arg == "-":
		return newStdinSource(opts.Stdin)
	case arg == "" || arg == ".":
		return newGitSource(ctx, dir, git.DiffSpec{Base: opts.Base, Staged: opts.Staged, Context: opts.Context})
	}

	// A path that exists on disk is a patch file.
	if fileExists(arg) {
		return newPatchFileSource(arg)
	}

	// Otherwise, a recognised PR reference is (for now) declined.
	if ref, ok := forge.ParseRef(arg); ok {
		return nil, fmt.Errorf("pull-request review (%s) is not available yet — it lands in Milestone 3; for now pass a patch file, \".\", or --base <ref>", ref)
	}

	return nil, fmt.Errorf("could not resolve %q: not a file, \".\", or a pull-request reference", arg)
}

// fileExists reports whether p is an existing regular file. It is the test
// that decides whether a bare argument means a patch file or should be tried
// as a pull-request reference, so directories deliberately do not count.
func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// hashString returns a short (16 hex digit) SHA-256 digest, used to fold
// arbitrary strings — file paths, patch contents, diff-spec titles — into
// draft keys that are stable, filesystem-safe, and of bounded length.
func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// --- patch file / stdin ---

type patchSource struct {
	title string
	key   string
	data  []byte
}

// newPatchFileSource reads the whole patch up front (Files may be called more
// than once, and the file could change or vanish mid-session). The key hashes
// the absolute path, so reviewing the same file from a different working
// directory resumes the same draft.
func newPatchFileSource(path string) (ReviewSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read patch %s: %w", path, err)
	}
	abs, _ := filepath.Abs(path)
	return &patchSource{
		title: filepath.Base(path),
		key:   "patch-" + hashString(abs),
		data:  data,
	}, nil
}

// newStdinSource captures stdin in full at resolution time — a pipe cannot be
// re-read later. With no path to identify the patch, the key hashes the
// content itself: feeding the same patch again resumes its draft.
func newStdinSource(r io.Reader) (ReviewSource, error) {
	if r == nil {
		return nil, fmt.Errorf("no stdin available for \"-\"")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	return &patchSource{
		title: "stdin",
		key:   "patch-" + hashString(string(data)),
		data:  data,
	}, nil
}

// Files parses the bytes captured at construction; re-parsing per call keeps
// the source stateless and is cheap at patch sizes.
func (p *patchSource) Files(context.Context) ([]diff.FileDiff, error) {
	return diff.ParsePatchBytes(p.data)
}

// Title (the base filename, or "stdin") is shown in the title bar.
func (p *patchSource) Title() string { return p.title }

// Key seeds the draft store's filename (path hash for files, content hash for stdin).
func (p *patchSource) Key() string { return p.key }

// HeadOID is empty: a raw patch carries no commit identity, so head-change
// staleness detection and comment relocation are disabled for this source.
func (p *patchSource) HeadOID(context.Context) string { return "" }

// --- local git ---

type gitSource struct {
	repo  *git.Repository
	spec  git.DiffSpec
	title string
	key   string
}

// newGitSource builds a source over the local repository containing dir. The
// diff itself is produced lazily by Files; only opening the repo can fail
// here. The key combines the repo root with the spec's title so different
// comparisons of the same repo (e.g. --staged vs --base main) keep separate
// drafts.
func newGitSource(ctx context.Context, dir string, spec git.DiffSpec) (ReviewSource, error) {
	repo, err := git.Open(ctx, dir)
	if err != nil {
		return nil, err
	}
	return &gitSource{
		repo:  repo,
		spec:  spec,
		title: spec.Title(),
		key:   "git-" + hashString(repo.Root) + "-" + hashString(spec.Title()),
	}, nil
}

// newGitRevSource handles the explicit two-argument form. Both arguments must
// resolve as revisions up front, so a typo fails with a clear message here
// instead of surfacing later as a confusing git-diff error.
func newGitRevSource(ctx context.Context, dir, revA, revB string, ctxLines int) (ReviewSource, error) {
	repo, err := git.Open(ctx, dir)
	if err != nil {
		return nil, err
	}
	if !repo.ResolveRev(ctx, revA) || !repo.ResolveRev(ctx, revB) {
		return nil, fmt.Errorf("both arguments must be revisions in the repository (got %q, %q)", revA, revB)
	}
	spec := git.DiffSpec{RevA: revA, RevB: revB, Context: ctxLines}
	return &gitSource{
		repo:  repo,
		spec:  spec,
		title: spec.Title(),
		key:   "git-" + hashString(repo.Root) + "-" + hashString(spec.Title()),
	}, nil
}

// Files runs the git diff each time it is called, so a reload picks up
// whatever the working tree looks like now. Empty output (no changes) yields
// nil files rather than an error.
func (g *gitSource) Files(ctx context.Context) ([]diff.FileDiff, error) {
	raw, err := g.repo.Diff(ctx, g.spec)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil, nil
	}
	return diff.ParsePatchBytes(raw)
}

// Title is the diff spec's description (e.g. "HEAD", "staged", "main..HEAD"),
// shown in the title bar.
func (g *gitSource) Title() string { return g.title }

// Key seeds the draft store's filename; see newGitSource for its composition.
func (g *gitSource) Key() string { return g.key }

// HeadOID returns the repository's current HEAD, letting the draft layer
// detect that commits landed since the draft was saved and re-anchor its
// comments. Errors degrade to "", which simply disables that check.
func (g *gitSource) HeadOID(ctx context.Context) string {
	oid, err := g.repo.HeadOID(ctx)
	if err != nil {
		return ""
	}
	return oid
}
