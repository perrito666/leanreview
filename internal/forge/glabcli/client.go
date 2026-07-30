// Package glabcli implements the forge.Forge interface by shelling out to the
// GitLab CLI (`glab`), whose `api` subcommand is a deliberate port of `gh api`
// (same --method/--input/--hostname/--paginate flags). Wrapping glab keeps
// authentication and self-hosted instances the user's problem to configure
// once, not ours. The command runner is injectable so the client is
// unit-testable without network access or a real glab binary.
//
// Terminology note: GitLab calls them merge requests and models review
// discussion as "discussions" containing "notes"; this package maps those onto
// the forge vocabulary (PullRequest, Thread, Comment).
package glabcli

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/perrito666/leanreview/internal/forge"
)

// Runner executes a `glab` invocation and returns its stdout. stdin, when
// non-nil, is fed to the process (used to send JSON request bodies). Tests
// substitute a fake; production uses ExecRunner.
type Runner func(ctx context.Context, stdin []byte, args ...string) ([]byte, error)

// Client is a forge.Forge backed by the `glab` CLI.
type Client struct {
	run Runner
}

var _ forge.Forge = (*Client)(nil)

// New returns a Client that shells out to the real `glab` binary.
func New() *Client {
	return &Client{run: ExecRunner}
}

// NewWithRunner returns a Client using a custom runner (for tests).
func NewWithRunner(r Runner) *Client {
	return &Client{run: r}
}

// ExecRunner runs `glab` with the given arguments, optionally feeding stdin.
func ExecRunner(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "glab", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("glab %s: %s", strings.Join(args, " "), msg)
		}
		return nil, fmt.Errorf("glab %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// apiArgs builds "api [--hostname H] <path> [extra...]". The hostname flag is
// added only for non-gitlab.com hosts so gitlab.com uses glab's default host.
func apiArgs(ref forge.PullRequestRef, path string, extra ...string) []string {
	args := []string{"api"}
	if ref.Host != "" && ref.Host != "gitlab.com" {
		args = append(args, "--hostname", ref.Host)
	}
	args = append(args, path)
	args = append(args, extra...)
	return args
}

// projectPath returns the URL-encoded "owner/repo" project id used in GitLab
// REST paths (slashes become %2F, covering nested subgroups).
func projectPath(ref forge.PullRequestRef) string {
	return url.PathEscape(fmt.Sprintf("%s/%s", ref.Owner, ref.Repo))
}
