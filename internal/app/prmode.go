package app

import (
	"fmt"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

// PRContext carries everything the TUI needs to operate in pull-request mode:
// the forge to reply/submit through, the PR reference and metadata, and the
// existing review threads. It is nil in local/patch mode.
type PRContext struct {
	Forge   forge.Forge
	Ref     forge.PullRequestRef
	PR      *forge.PullRequest
	Threads []forge.Thread
}

// buildThreadIndex maps a location key to the indices of threads anchored there,
// so the diff view can mark lines that already carry review discussion.
func (m *Model) buildThreadIndex() {
	m.threadIndex = map[string][]int{}
	if m.pr == nil {
		return
	}
	for i, th := range m.pr.Threads {
		if th.Location == nil {
			continue
		}
		key := locKey(th.Location.Path, th.Location.Side, th.Location.StartLine)
		m.threadIndex[key] = append(m.threadIndex[key], i)
	}
}

func locKey(path string, side diff.Side, line int) string {
	return fmt.Sprintf("%s|%d|%d", path, side, line)
}

// threadsAt returns the thread indices anchored to the row at index i.
func (m *Model) threadsAt(i int) []int {
	if m.pr == nil {
		return nil
	}
	r := m.rowAt(i)
	if r == nil || r.Source == nil {
		return nil
	}
	return m.threadIndex[locKey(r.Source.Path, r.Source.Side, r.Source.StartLine)]
}

// prActive reports whether the model is in pull-request mode.
func (m *Model) prActive() bool { return m.pr != nil }
