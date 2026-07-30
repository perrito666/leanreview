package ghcli

import (
	"context"
	"strings"
	"testing"
)

func TestListDefaultsAndParses(t *testing.T) {
	cap := &capture{response: map[string]string{
		"search prs": `[
		  {"number":418,"title":"Fix parser","author":{"login":"alice"},
		   "repository":{"nameWithOwner":"owner/repo"},
		   "updatedAt":"2026-07-29T10:00:00Z","url":"https://github.com/owner/repo/pull/418"},
		  {"number":7,"title":"Docs","author":{"login":"bob"},
		   "repository":{"nameWithOwner":"other/proj"},
		   "updatedAt":"2026-07-28T10:00:00Z","url":"https://github.com/other/proj/pull/7"}
		]`,
	}}
	c := NewWithRunner(cap.run)
	got, err := c.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("results = %d, want 2", len(got))
	}
	if got[0].Ref.Owner != "owner" || got[0].Ref.Repo != "repo" || got[0].Ref.Number != 418 {
		t.Errorf("ref = %+v", got[0].Ref)
	}
	if got[0].Author != "alice" || got[0].Title != "Fix parser" {
		t.Errorf("entry = %+v", got[0])
	}

	args := strings.Join(cap.calls[0], " ")
	// Default filter terms are passed as individual search arguments.
	if !strings.Contains(args, "is:open review-requested:@me") {
		t.Errorf("default filter missing: %s", args)
	}
	if !strings.Contains(args, "--json") || !strings.Contains(args, "--limit") {
		t.Errorf("json/limit flags missing: %s", args)
	}
}

func TestListCustomFilter(t *testing.T) {
	cap := &capture{response: map[string]string{"search prs": `[]`}}
	c := NewWithRunner(cap.run)
	if _, err := c.List(context.Background(), "repo:cli/cli author:alice"); err != nil {
		t.Fatalf("List: %v", err)
	}
	args := strings.Join(cap.calls[0], " ")
	if !strings.Contains(args, "repo:cli/cli author:alice") {
		t.Errorf("custom filter not passed: %s", args)
	}
	if strings.Contains(args, "review-requested") {
		t.Errorf("default filter should be replaced, not appended: %s", args)
	}
}
