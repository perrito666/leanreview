package main

import (
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/config"
)

func listCfg() config.Config {
	return config.Config{
		ListEngine: "gh",
		ListFilter: "is:open review-requested:@me",
		ListFilters: map[string]string{
			"bugs": "is:open label:bug",
			"mrs":  "state=opened&labels=urgent",
		},
	}
}

func TestResolveListQuery(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		engine  string
		filter  string
		errPart string
	}{
		{name: "nothing → fallback filter", args: nil, engine: "gh", filter: "is:open review-requested:@me"},
		{name: "engine only → fallback filter", args: []string{"glab"}, engine: "glab", filter: "is:open review-requested:@me"},
		{name: "raw filter", args: []string{"author:x is:open"}, engine: "gh", filter: "author:x is:open"},
		{name: "raw qualifier is not a selector", args: []string{"label:bug"}, engine: "gh", filter: "label:bug"},
		{name: "named on default engine", args: []string{":bugs"}, engine: "gh", filter: "is:open label:bug"},
		{name: "named on explicit engine", args: []string{"gh:bugs"}, engine: "gh", filter: "is:open label:bug"},
		{name: "named on glab", args: []string{"glab:mrs"}, engine: "glab", filter: "state=opened&labels=urgent"},
		{name: "named + refinement (gh, spaces)", args: []string{":bugs", "base:main"}, engine: "gh", filter: "is:open label:bug base:main"},
		{name: "named + refinement (glab, ampersand)", args: []string{"glab:mrs", "milestone_title=v2"}, engine: "glab", filter: "state=opened&labels=urgent&milestone_title=v2"},
		{name: "engine + raw filter", args: []string{"gh", "org:corp"}, engine: "gh", filter: "org:corp"},
		{name: "unknown name errors with available", args: []string{":nope"}, errPart: `available: bugs, mrs`},
		{name: "empty name errors", args: []string{":"}, errPart: "empty filter name"},
		{name: "unknown engine prefix is raw text", args: []string{"hg:bugs"}, engine: "gh", filter: "hg:bugs"},
	}
	for _, c := range cases {
		engine, filter, err := resolveListQuery(listCfg(), c.args)
		if c.errPart != "" {
			if err == nil || !strings.Contains(err.Error(), c.errPart) {
				t.Errorf("%s: err = %v, want containing %q", c.name, err, c.errPart)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
			continue
		}
		if engine != c.engine || filter != c.filter {
			t.Errorf("%s: got (%q, %q), want (%q, %q)", c.name, engine, filter, c.engine, c.filter)
		}
	}
}

func TestVersionFlagParsed(t *testing.T) {
	for _, f := range []string{"-v", "--version"} {
		if _, err := parseArgs([]string{f}); err != errVersion {
			t.Errorf("parseArgs(%q) err = %v, want errVersion", f, err)
		}
	}
}

func TestResolveListQueryNoNamedConfigured(t *testing.T) {
	cfg := listCfg()
	cfg.ListFilters = nil
	_, _, err := resolveListQuery(cfg, []string{":bugs"})
	if err == nil || !strings.Contains(err.Error(), "no named filters configured") {
		t.Errorf("err = %v, want a hint about list_filters", err)
	}
}
