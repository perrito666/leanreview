// Package ghcli implements the forge.Forge interface by shelling out to the
// GitHub CLI (`gh`). Wrapping `gh` means authentication, enterprise hosts, and
// token storage are already solved by a tool the user has configured. The
// command runner is injectable so the client is unit-testable without network
// access or a real `gh` binary.
package ghcli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/perrito666/leanreview/internal/forge"
)

// Runner executes a `gh` invocation and returns its stdout. stdin, when
// non-nil, is fed to the process (used to send JSON request bodies). Tests
// substitute a fake; production uses ExecRunner.
type Runner func(ctx context.Context, stdin []byte, args ...string) ([]byte, error)

// Client is a forge.Forge backed by the `gh` CLI.
type Client struct {
	run Runner
}

// Ensure Client satisfies the interface.
var _ forge.Forge = (*Client)(nil)

// New returns a Client that shells out to the real `gh` binary.
func New() *Client {
	return &Client{run: ExecRunner}
}

// NewWithRunner returns a Client using a custom runner (for tests).
func NewWithRunner(r Runner) *Client {
	return &Client{run: r}
}

// ExecRunner runs `gh` with the given arguments, optionally feeding stdin.
func ExecRunner(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// apiArgs builds "api [--hostname H] <path> [extra...]". The hostname flag is
// added only for non-github.com hosts so github.com continues to use gh's
// default host resolution.
func apiArgs(ref forge.PullRequestRef, path string, extra ...string) []string {
	args := []string{"api"}
	if ref.Host != "" && ref.Host != "github.com" {
		args = append(args, "--hostname", ref.Host)
	}
	args = append(args, path)
	args = append(args, extra...)
	return args
}

// repoPath returns "owner/repo", the segment used in REST API paths.
func repoPath(ref forge.PullRequestRef) string {
	return fmt.Sprintf("%s/%s", ref.Owner, ref.Repo)
}
