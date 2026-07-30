package glabcli

import (
	"context"
	"strings"
	"testing"
)

func TestListSubstitutesMeAndParses(t *testing.T) {
	cap := &capture{response: map[string]string{
		"api user": `{"username":"hduran"}`,
		"merge_requests?": `[
		  {"iid":4,"title":"Fix pipeline","author":{"username":"carol"},
		   "updated_at":"2026-07-29T09:00:00Z",
		   "web_url":"https://gitlab.com/group/sub/proj/-/merge_requests/4"}
		]`,
	}}
	c := NewWithRunner(cap.run)
	got, err := c.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("results = %d, want 1", len(got))
	}
	ref := got[0].Ref
	if ref.Host != "gitlab.com" || ref.Owner != "group/sub" || ref.Repo != "proj" || ref.Number != 4 {
		t.Errorf("ref = %+v", ref)
	}

	// The listing call must carry the substituted username, not "@me".
	last := strings.Join(cap.calls[len(cap.calls)-1], " ")
	if !strings.Contains(last, "reviewer_username=hduran") || strings.Contains(last, "@me") {
		t.Errorf("@me not substituted: %s", last)
	}
}

func TestListCustomFilterSkipsUserLookup(t *testing.T) {
	cap := &capture{response: map[string]string{"merge_requests?": `[]`}}
	c := NewWithRunner(cap.run)
	if _, err := c.List(context.Background(), "state=opened&labels=bug"); err != nil {
		t.Fatalf("List: %v", err)
	}
	// No @me in the filter: exactly one call, no user lookup.
	if len(cap.calls) != 1 {
		t.Fatalf("calls = %d, want 1 (no user lookup)", len(cap.calls))
	}
	if got := strings.Join(cap.calls[0], " "); !strings.Contains(got, "state=opened&labels=bug&per_page=") {
		t.Errorf("filter not passed through: %s", got)
	}
}
