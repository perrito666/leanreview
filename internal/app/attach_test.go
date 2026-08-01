package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

// uploadingForge is a recordingForge that can also receive attachments.
type uploadingForge struct {
	recordingForge
	uploaded []string
}

func (f *uploadingForge) UploadAttachment(_ context.Context, _ forge.PullRequestRef, path string) (string, error) {
	f.uploaded = append(f.uploaded, path)
	return "/uploads/secret123/" + filepath.Base(path), nil
}

// attachModel returns a PR-mode model with one draft comment under the cursor.
func attachModel(t *testing.T, f forge.Forge) (*Model, string) {
	t.Helper()
	m := prModel(t, f, nil)
	m.cursor = mustFindRow(t, m, diff.SideRight, 72)
	loc, snip, err := m.buildLocation()
	if err != nil {
		t.Fatal(err)
	}
	id := m.draft.Add(loc, "see the glitch", snip)
	return m, id
}

// TestAttachPromptValidatesAndCorrects: a bad path keeps the prompt open
// with the error so it can be corrected in place; a valid image attaches as
// a Markdown reference on the draft body.
func TestAttachPromptValidatesAndCorrects(t *testing.T) {
	m, id := attachModel(t, &recordingForge{})
	png := appPNGFixture(t)

	m = key(m, "I")
	if !m.attachActive {
		t.Fatalf("I did not open the attach prompt")
	}
	// A wrong path: error shown, prompt still open.
	for _, r := range "/no/such.png" {
		m.handleAttachKey(string(r))
	}
	m.handleAttachKey("enter")
	if !m.attachActive || m.attachErr == "" || !strings.Contains(m.attachErr, "does not exist") {
		t.Fatalf("bad path not held for correction: active=%v err=%q", m.attachActive, m.attachErr)
	}
	if out := m.View(); !strings.Contains(out, "attach image path:") || !strings.Contains(out, "does not exist") {
		t.Errorf("prompt/error not visible in the status bar")
	}
	// A directory is named as such.
	m.attachInput = filepath.Dir(png)
	m.handleAttachKey("enter")
	if !strings.Contains(m.attachErr, "directory") {
		t.Errorf("directory not diagnosed: %q", m.attachErr)
	}
	// A non-image file is refused.
	txt := filepath.Join(t.TempDir(), "notes.txt")
	os.WriteFile(txt, []byte("hi"), 0o644)
	m.attachInput = txt
	m.handleAttachKey("enter")
	if !strings.Contains(m.attachErr, "not a recognised image") {
		t.Errorf("non-image not diagnosed: %q", m.attachErr)
	}
	// Correcting to a real image attaches and closes the prompt.
	m.attachInput = png
	m.handleAttachKey("enter")
	if m.attachActive {
		t.Fatalf("prompt should close on success: err=%q", m.attachErr)
	}
	body := m.draft.Get(id).Body
	if !strings.Contains(body, "![shot.png]("+png+")") {
		t.Errorf("reference not attached: %q", body)
	}
}

// TestAttachNeedsADraft: with no draft under the cursor the prompt refuses
// to open — there is nothing to attach to.
func TestAttachNeedsADraft(t *testing.T) {
	m := prModel(t, &recordingForge{}, nil)
	m = key(m, "I")
	if m.attachActive {
		t.Errorf("prompt opened with no draft under cursor")
	}
	if !strings.Contains(m.status, "draft comment") {
		t.Errorf("status = %q", m.status)
	}
}

// TestSubmitUploadsAndRewrites: local references upload once each (deduped)
// before anything posts, and every outgoing body carries the returned
// /uploads reference instead of the local path.
func TestSubmitUploadsAndRewrites(t *testing.T) {
	f := &uploadingForge{}
	m, id := attachModel(t, f)
	png := appPNGFixture(t)
	m.attachTarget = id
	m.attachImage(png)
	m.draft.AddGeneral("general with the same shot\n\n![x]("+png+")", nil)
	m.pendingEvent = forge.EventComment

	msg := m.doSubmit()().(submitResultMsg)
	m.onSubmitResult(msg)
	if msg.err != nil {
		t.Fatalf("submit: %v", msg.err)
	}
	if len(f.uploaded) != 1 || f.uploaded[0] != png {
		t.Errorf("uploads = %v, want the file exactly once", f.uploaded)
	}
	want := "/uploads/secret123/" + filepath.Base(png)
	if len(f.createdComments) != 1 || !strings.Contains(f.createdComments[0].Body, "("+want+")") {
		t.Errorf("comment body not rewritten: %+v", f.createdComments)
	}
	if strings.Contains(f.createdComments[0].Body, png) {
		t.Errorf("local path leaked to the host")
	}
	if len(f.generals) != 1 || !strings.Contains(f.generals[0], "("+want+")") {
		t.Errorf("general body not rewritten: %v", f.generals)
	}
}

// TestSubmitRefusesLocalImagesWithoutUploader: on a forge with no upload API
// (GitHub) submission stops before any network call, with the reason named.
func TestSubmitRefusesLocalImagesWithoutUploader(t *testing.T) {
	f := &recordingForge{}
	m, id := attachModel(t, f)
	m.attachTarget = id
	m.attachImage(appPNGFixture(t))
	m.pendingEvent = forge.EventComment

	if cmd := m.doSubmit(); cmd != nil {
		t.Fatalf("submission should stop before posting")
	}
	if m.err == nil || !strings.Contains(m.err.Error(), "image uploads") {
		t.Errorf("err = %v", m.err)
	}
	if f.createdComments != nil {
		t.Errorf("something posted despite the refusal")
	}
}
