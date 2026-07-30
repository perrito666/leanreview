package editor

import (
	"reflect"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"vi", []string{"vi"}},
		{"nvim -f", []string{"nvim", "-f"}},
		{"code --wait", []string{"code", "--wait"}},
		{`"/Applications/My Editor" --wait`, []string{"/Applications/My Editor", "--wait"}},
		{`emacs -nw 'a b'`, []string{"emacs", "-nw", "a b"}},
		{"  spaced   out  ", []string{"spaced", "out"}},
	}
	for _, c := range cases {
		got := SplitCommand(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitCommand(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestResolvePrecedence(t *testing.T) {
	t.Setenv("GIT_EDITOR", "")
	t.Setenv("VISUAL", "visual-ed")
	t.Setenv("EDITOR", "editor-ed")

	// configured wins over everything.
	e, _ := Resolve("configured-ed --wait")
	if e.Command != "configured-ed" || len(e.Args) != 1 || e.Args[0] != "--wait" {
		t.Errorf("configured not chosen: %+v", e)
	}

	// with no config, VISUAL beats EDITOR.
	e, _ = Resolve("")
	if e.Command != "visual-ed" {
		t.Errorf("expected visual-ed, got %+v", e)
	}
}

func TestCleanTemplate(t *testing.T) {
	in := "<!--\nRepository: owner/repo\nFile: x.go\n-->\n\nThis is my note.\n"
	if got := CleanTemplate(in); got != "This is my note." {
		t.Errorf("CleanTemplate = %q", got)
	}
	if got := CleanTemplate("<!-- only header -->\n\n"); got != "" {
		t.Errorf("empty body should clean to empty, got %q", got)
	}
}

func TestBuildTemplateOmitsEmpty(t *testing.T) {
	out := BuildTemplate(TemplateContext{Repository: "owner/repo", File: "x.go", Lines: "72"}, "")
	if want := "Repository: owner/repo"; !contains(out, want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
	if contains(out, "Pull request:") {
		t.Errorf("empty PR field should be omitted:\n%s", out)
	}
	// Round-trips to empty body.
	if CleanTemplate(out) != "" {
		t.Errorf("template with empty body should clean to empty")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
