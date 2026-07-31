package app

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// imageFetchedMsg delivers a forge attachment's bytes (or the failure).
type imageFetchedMsg struct {
	url  string
	data []byte
	err  error
}

// isRemoteRef reports whether an image reference needs fetching rather than
// a local read: absolute URLs, and GitLab's project-relative /uploads paths.
func isRemoteRef(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "/uploads/")
}

// maybeFetchImages scans every comment and thread body for remote image
// references and requests each one once, asynchronously, through the forge
// attachment fetcher. Discovery is eager (bodies are all known up front)
// precisely so the render path never has to trigger network work.
func (m *Model) maybeFetchImages() tea.Cmd {
	if m.fetchImage == nil || !m.images.Enabled() {
		return nil
	}
	want := map[string]bool{}
	collect := func(body string) {
		for _, ref := range imageRefs(body) {
			if isRemoteRef(ref) && m.imageFiles[ref] == "" && !m.imagePending[ref] && !m.imageFailed[ref] {
				want[ref] = true
			}
		}
	}
	if m.draft != nil {
		for i := range m.draft.Comments {
			collect(m.draft.Comments[i].Body)
			for _, rp := range m.draft.Comments[i].Replies {
				collect(rp.Body)
			}
		}
	}
	if m.pr != nil {
		for _, th := range m.pr.Threads {
			collect(th.Root.Body)
			for _, rp := range th.Replies {
				collect(rp.Body)
			}
		}
	}
	var cmds []tea.Cmd
	for url := range want {
		m.imagePending[url] = true
		u := url
		cmds = append(cmds, func() tea.Msg {
			data, err := m.fetchImage(m.ctx, u)
			return imageFetchedMsg{url: u, data: data, err: err}
		})
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// onImageFetched lands attachment bytes in a session-scoped file the
// renderer can read. Failures are remembered so a broken URL is asked for
// exactly once and keeps rendering as a tag.
func (m *Model) onImageFetched(msg imageFetchedMsg) {
	delete(m.imagePending, msg.url)
	if msg.err != nil || len(msg.data) == 0 {
		m.imageFailed[msg.url] = true
		return
	}
	if m.imageDir == "" {
		dir, err := os.MkdirTemp("", "leanreview-img-*")
		if err != nil {
			m.imageFailed[msg.url] = true
			return
		}
		m.imageDir = dir
	}
	sum := sha256.Sum256([]byte(msg.url))
	name := hex.EncodeToString(sum[:8])
	if ext := filepath.Ext(msg.url); len(ext) > 1 && len(ext) <= 5 && !strings.ContainsAny(ext, "/?&=") {
		name += ext
	}
	path := filepath.Join(m.imageDir, name)
	if err := os.WriteFile(path, msg.data, 0o600); err != nil {
		m.imageFailed[msg.url] = true
		return
	}
	m.imageFiles[msg.url] = path
}
