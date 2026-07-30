package glabcli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/perrito666/leanreview/internal/forge"
)

// mrJSON is the subset of the GitLab merge-request payload we consume.
type mrJSON struct {
	IID    int    `json:"iid"`
	Title  string `json:"title"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	SHA      string `json:"sha"`
	DiffRefs struct {
		BaseSHA  string `json:"base_sha"`
		StartSHA string `json:"start_sha"`
		HeadSHA  string `json:"head_sha"`
	} `json:"diff_refs"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	WebURL       string `json:"web_url"`
}

func (c *Client) mr(ctx context.Context, ref forge.PullRequestRef) (*mrJSON, error) {
	path := fmt.Sprintf("projects/%s/merge_requests/%d", projectPath(ref), ref.Number)
	out, err := c.run(ctx, nil, apiArgs(ref, path)...)
	if err != nil {
		return nil, err
	}
	var m mrJSON
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, fmt.Errorf("decode merge request: %w", err)
	}
	return &m, nil
}

// PullRequest fetches merge-request metadata via `glab api`.
func (c *Client) PullRequest(ctx context.Context, ref forge.PullRequestRef) (*forge.PullRequest, error) {
	m, err := c.mr(ctx, ref)
	if err != nil {
		return nil, err
	}
	return &forge.PullRequest{
		Ref:     ref,
		Title:   m.Title,
		Author:  m.Author.Username,
		HeadOID: m.SHA,
		BaseRef: m.TargetBranch,
		HeadRef: m.SourceBranch,
	}, nil
}

// changeJSON is one file's diff as returned by the changes endpoint.
type changeJSON struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	NewFile     bool   `json:"new_file"`
	DeletedFile bool   `json:"deleted_file"`
	RenamedFile bool   `json:"renamed_file"`
	Diff        string `json:"diff"`
}

// Diff fetches the merge request's changes and synthesizes a unified git-style
// patch from them (GitLab's `diff` field carries only the hunks, so the
// per-file headers are reconstructed here). This is GitLab's canonical diff
// representation — the one review-comment positions must align with.
func (c *Client) Diff(ctx context.Context, ref forge.PullRequestRef) ([]byte, error) {
	path := fmt.Sprintf("projects/%s/merge_requests/%d/changes", projectPath(ref), ref.Number)
	out, err := c.run(ctx, nil, apiArgs(ref, path)...)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Changes []changeJSON `json:"changes"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("decode merge-request changes: %w", err)
	}

	var b strings.Builder
	for _, ch := range payload.Changes {
		writeChange(&b, ch)
	}
	return []byte(b.String()), nil
}

func writeChange(b *strings.Builder, ch changeJSON) {
	fmt.Fprintf(b, "diff --git a/%s b/%s\n", ch.OldPath, ch.NewPath)
	switch {
	case ch.NewFile:
		b.WriteString("new file mode 100644\n")
	case ch.DeletedFile:
		b.WriteString("deleted file mode 100644\n")
	case ch.RenamedFile:
		fmt.Fprintf(b, "rename from %s\nrename to %s\n", ch.OldPath, ch.NewPath)
	}

	diff := ch.Diff
	// Some GitLab versions include the ---/+++ header lines in the diff field;
	// emit our own only when they are absent.
	if !strings.HasPrefix(diff, "--- ") {
		if ch.NewFile {
			b.WriteString("--- /dev/null\n")
		} else {
			fmt.Fprintf(b, "--- a/%s\n", ch.OldPath)
		}
		if ch.DeletedFile {
			b.WriteString("+++ /dev/null\n")
		} else {
			fmt.Fprintf(b, "+++ b/%s\n", ch.NewPath)
		}
	}
	b.WriteString(diff)
	if diff != "" && !strings.HasSuffix(diff, "\n") {
		b.WriteByte('\n')
	}
}
