package git

import "testing"

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		in                string
		host, owner, repo string
		wantErr           bool
	}{
		{in: "git@github.com:perrito666/leanreview.git", host: "github.com", owner: "perrito666", repo: "leanreview"},
		{in: "git@github.com:perrito666/leanreview", host: "github.com", owner: "perrito666", repo: "leanreview"},
		{in: "https://github.com/perrito666/leanreview.git", host: "github.com", owner: "perrito666", repo: "leanreview"},
		{in: "https://github.com/perrito666/leanreview", host: "github.com", owner: "perrito666", repo: "leanreview"},
		{in: "ssh://git@github.com/perrito666/leanreview.git", host: "github.com", owner: "perrito666", repo: "leanreview"},
		{in: "ssh://git@github.com:22/perrito666/leanreview.git", host: "github.com", owner: "perrito666", repo: "leanreview"},
		{in: "https://gitlab.com/group/subgroup/proj.git", host: "gitlab.com", owner: "group/subgroup", repo: "proj"},
		{in: "https://ghe.example.com/team/repo", host: "ghe.example.com", owner: "team", repo: "repo"},
		{in: "", wantErr: true},
		{in: "not a url", wantErr: true},
	}
	for _, c := range cases {
		got, err := ParseRemoteURL(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRemoteURL(%q) expected error, got %+v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRemoteURL(%q) error: %v", c.in, err)
			continue
		}
		if got.Host != c.host || got.Owner != c.owner || got.Repo != c.repo {
			t.Errorf("ParseRemoteURL(%q) = %+v, want %s/%s/%s", c.in, got, c.host, c.owner, c.repo)
		}
	}
}
