package ghcli

import (
	"strings"
	"testing"
)

// TestResolveAttachments maps session-gated asset URLs to the signed URLs in
// body_html by UUID — the only programmatic route to attachment bytes.
func TestResolveAttachments(t *testing.T) {
	body := `look <img alt="Image" src="https://github.com/user-attachments/assets/25addea2-357a-42c0-a1c8-fba0163382cc" /> and ![x](https://github.com/user-attachments/assets/4ac2ccd6-338a-44e0-b75c-41155aa431d4)`
	html := `<p>look <img src="https://private-user-images.githubusercontent.com/1/604-25addea2-357a-42c0-a1c8-fba0163382cc.png?jwt=AAA" alt="Image"> and <img src="https://private-user-images.githubusercontent.com/1/605-4ac2ccd6-338a-44e0-b75c-41155aa431d4.png?jwt=BBB"></p>`
	got := resolveAttachments(body, html)
	if !strings.Contains(got, "604-25addea2-357a-42c0-a1c8-fba0163382cc.png?jwt=AAA") ||
		!strings.Contains(got, "605-4ac2ccd6-338a-44e0-b75c-41155aa431d4.png?jwt=BBB") {
		t.Errorf("assets not resolved to signed URLs:\n%s", got)
	}
	if strings.Contains(got, "github.com/user-attachments") {
		t.Errorf("session-gated URL survived resolution:\n%s", got)
	}
	// Unmatched assets stay put; empty html is a no-op.
	keep := "see https://github.com/user-attachments/assets/ffffffff-0000-0000-0000-000000000000"
	if resolveAttachments(keep, html) != keep {
		t.Errorf("unmatched asset was rewritten")
	}
	if resolveAttachments(body, "") != body {
		t.Errorf("empty body_html should be a no-op")
	}
}
