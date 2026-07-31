package glabcli

import (
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/forge"
)

// TestAdaptSuggestion: GitHub's implicit-range fence becomes GitLab's ranged
// form for multi-line comments; single-line and pre-ranged bodies pass
// through untouched.
func TestAdaptSuggestion(t *testing.T) {
	multi := forge.ReviewComment{Body: "fix:\n```suggestion\na\nb\nc\n```", StartLine: 10, Line: 12}
	got := adaptSuggestion(multi)
	if !strings.Contains(got, "```suggestion:-2+0") {
		t.Errorf("multi-line fence not ranged:\n%s", got)
	}
	single := forge.ReviewComment{Body: "```suggestion\nx\n```", Line: 5}
	if adaptSuggestion(single) != single.Body {
		t.Errorf("single-line fence must pass through")
	}
	ranged := forge.ReviewComment{Body: "```suggestion:-1+0\nx\n```", StartLine: 4, Line: 5}
	if adaptSuggestion(ranged) != ranged.Body {
		t.Errorf("already-ranged fence must pass through")
	}
}
