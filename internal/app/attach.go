package app

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	// The prompt validates that a path decodes as an image; registering the
	// formats here keeps the capability self-contained.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// startAttachImage opens the attach prompt for a draft comment (the one
// under the cursor in normal mode, a selected general draft in the
// conversation screen). The mechanism is deliberately a plain path prompt —
// paste a path, get it validated — no picker.
func (m *Model) startAttachImage(targetID string) {
	if targetID == "" {
		m.setStatus("attach needs a draft comment (add one with c first)")
		return
	}
	m.attachActive = true
	m.attachTarget = targetID
	m.attachInput = ""
	m.attachErr = ""
}

// handleAttachKey drives the attach prompt: type or paste a path, enter
// validates it (must exist, be a file, and decode as an image) and attaches
// on success; a failed validation keeps the prompt open with the error so
// the path can be corrected in place. esc cancels.
func (m *Model) handleAttachKey(key string) {
	switch key {
	case "esc":
		m.attachActive = false
		m.attachInput = ""
		m.attachErr = ""
	case "enter":
		path := strings.TrimSpace(m.attachInput)
		if err := validateImagePath(path); err != nil {
			m.attachErr = err.Error()
			return
		}
		m.attachImage(path)
		m.attachActive = false
		m.attachInput = ""
		m.attachErr = ""
	case "backspace":
		if r := []rune(m.attachInput); len(r) > 0 {
			m.attachInput = string(r[:len(r)-1])
		}
		m.attachErr = ""
	default:
		// One rune = typed text; pasted paths arrive as their runes.
		if utf8.RuneCountInString(key) == 1 {
			m.attachInput += key
			m.attachErr = ""
		}
	}
}

// attachImage appends the image reference to the target draft's body. The
// local path renders immediately in the thread box; on submission the file
// is uploaded through the forge and the reference rewritten (or, on forges
// without an upload API, submission stops with a clear error).
func (m *Model) attachImage(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	ref := fmt.Sprintf("![%s](%s)", filepath.Base(abs), abs)
	if c := m.draft.Get(m.attachTarget); c != nil {
		c.Body = strings.TrimRight(c.Body, "\n") + "\n\n" + ref
	} else if g := m.draft.GetGeneral(m.attachTarget); g != nil {
		g.Body = strings.TrimRight(g.Body, "\n") + "\n\n" + ref
	} else {
		m.setStatus("the draft disappeared before the image was attached")
		return
	}
	m.saveDraft()
	m.setStatus("image attached — it uploads on submit")
}

// validateImagePath is the prompt's gate: the path must exist, be a regular
// file, and decode as an image — each failure named so the path can be fixed
// rather than guessed at.
func validateImagePath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s does not exist", path)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not an image file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %v", path, err)
	}
	defer f.Close()
	if _, _, err := image.DecodeConfig(f); err != nil {
		return fmt.Errorf("%s is not a recognised image (png/jpeg/gif)", path)
	}
	return nil
}

// localImageRefs returns the image references in body that point at local
// files — the ones submission must upload (or refuse).
func localImageRefs(body string) []string {
	var out []string
	for _, ref := range imageRefs(body) {
		if isRemoteRef(ref) {
			continue
		}
		if _, err := os.Stat(ref); err == nil {
			out = append(out, ref)
		}
	}
	return out
}
