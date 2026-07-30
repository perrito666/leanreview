package forge

import "testing"

func TestParseRef(t *testing.T) {
	cases := []struct {
		in    string
		want  PullRequestRef
		notOK bool
	}{
		{in: "https://github.com/owner/repo/pull/418", want: PullRequestRef{Host: "github.com", Owner: "owner", Repo: "repo", Number: 418}},
		{in: "https://ghe.example.com/team/proj/pull/7", want: PullRequestRef{Host: "ghe.example.com", Owner: "team", Repo: "proj", Number: 7}},
		{in: "https://gitlab.com/group/repo/-/merge_requests/42", want: PullRequestRef{Host: "gitlab.com", Owner: "group", Repo: "repo", Number: 42}},
		{in: "https://gitlab.com/group/subgroup/repo/-/merge_requests/42", want: PullRequestRef{Host: "gitlab.com", Owner: "group/subgroup", Repo: "repo", Number: 42}},
		{in: "https://gitlab.example.com/team/proj/-/merge_requests/9", want: PullRequestRef{Host: "gitlab.example.com", Owner: "team", Repo: "proj", Number: 9}},
		{in: "owner/repo#418", want: PullRequestRef{Host: "github.com", Owner: "owner", Repo: "repo", Number: 418}},
		{in: "group/repo!42", want: PullRequestRef{Host: "gitlab.com", Owner: "group", Repo: "repo", Number: 42}},
		{in: "group/sub/repo!42", want: PullRequestRef{Host: "gitlab.com", Owner: "group/sub", Repo: "repo", Number: 42}},
		// Bare numbers leave host/owner/repo empty for origin inference.
		{in: "418", want: PullRequestRef{Number: 418}},
		{in: "#418", want: PullRequestRef{Number: 418}},
		{in: "!42", want: PullRequestRef{Number: 42}},
		// Non-references.
		{in: "some/file.diff", notOK: true},
		{in: "not a ref", notOK: true},
		{in: "", notOK: true},
	}
	for _, c := range cases {
		got, ok := ParseRef(c.in)
		if c.notOK {
			if ok {
				t.Errorf("ParseRef(%q) unexpectedly ok: %+v", c.in, got)
			}
			continue
		}
		if !ok {
			t.Errorf("ParseRef(%q) not recognised", c.in)
			continue
		}
		if got != c.want {
			t.Errorf("ParseRef(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestKindForHost(t *testing.T) {
	cases := []struct {
		host string
		want Kind
	}{
		{"github.com", KindGitHub},
		{"", KindGitHub},
		{"ghe.example.com", KindGitHub},
		{"gitlab.com", KindGitLab},
		{"gitlab.example.com", KindGitLab},
		{"my-gitlab.corp.net", KindGitLab},
	}
	for _, c := range cases {
		if got := KindForHost(c.host); got != c.want {
			t.Errorf("KindForHost(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

func TestRefString(t *testing.T) {
	gh := PullRequestRef{Host: "github.com", Owner: "o", Repo: "r", Number: 1}
	if got := gh.String(); got != "github.com/o/r#1" {
		t.Errorf("github ref string = %q", got)
	}
	gl := PullRequestRef{Host: "gitlab.com", Owner: "g/s", Repo: "r", Number: 2}
	if got := gl.String(); got != "gitlab.com/g/s/r!2" {
		t.Errorf("gitlab ref string = %q", got)
	}
}
