package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// ParsePatch reads a unified/Git patch and adapts it into our canonical model.
//
// It computes per-line old/new numbers by walking each fragment from its
// starting positions, and assigns each line a GitHub-style PatchPosition: the
// 1-based offset counted down from the file's first "@@" header, where every
// content line and every subsequent hunk header advances the position.
func ParsePatch(r io.Reader) ([]FileDiff, error) {
	files, _, err := gitdiff.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse patch: %w", err)
	}

	out := make([]FileDiff, 0, len(files))
	for _, f := range files {
		out = append(out, convertFile(f))
	}
	return out, nil
}

// ParsePatchBytes is a convenience wrapper over ParsePatch for in-memory patches.
func ParsePatchBytes(b []byte) ([]FileDiff, error) {
	return ParsePatch(strings.NewReader(string(b)))
}

func convertFile(f *gitdiff.File) FileDiff {
	fd := FileDiff{
		OldPath:   f.OldName,
		NewPath:   f.NewName,
		Status:    statusOf(f),
		IsBinary:  f.IsBinary,
		IsRenamed: f.IsRename,
		RawPatch:  f.String(),
	}

	// GitHub patch position counts from the first hunk header downward.
	pos := 0
	for hi, frag := range f.TextFragments {
		if hi > 0 {
			// A subsequent "@@" header itself consumes a diff position.
			pos++
		}
		hunk := Hunk{Header: strings.TrimRight(frag.Header(), "\n")}

		oldLine := int(frag.OldPosition)
		newLine := int(frag.NewPosition)

		for _, ln := range frag.Lines {
			pos++
			p := pos
			dl := DiffLine{
				Text:          expandTabs(strings.TrimRight(ln.Line, "\n"), 4),
				PatchPosition: &p,
			}
			switch ln.Op {
			case gitdiff.OpContext:
				dl.Kind = LineContext
				o, n := oldLine, newLine
				dl.OldLine, dl.NewLine = &o, &n
				oldLine++
				newLine++
			case gitdiff.OpDelete:
				dl.Kind = LineDeletion
				o := oldLine
				dl.OldLine = &o
				oldLine++
			case gitdiff.OpAdd:
				dl.Kind = LineAddition
				n := newLine
				dl.NewLine = &n
				newLine++
			}
			hunk.Lines = append(hunk.Lines, dl)
		}
		fd.Hunks = append(fd.Hunks, hunk)
	}

	return fd
}

// expandTabs replaces tabs with spaces using tab-stop columns, so display width
// is predictable (terminals and lipgloss otherwise expand tabs unpredictably
// relative to our padding). Column counting starts at the beginning of the line
// text (the number gutter is added separately during rendering).
func expandTabs(s string, tab int) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := tab - col%tab
			for i := 0; i < n; i++ {
				b.WriteByte(' ')
			}
			col += n
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}

func statusOf(f *gitdiff.File) FileStatus {
	switch {
	case f.IsBinary:
		return StatusBinary
	case f.IsNew:
		return StatusAdded
	case f.IsDelete:
		return StatusDeleted
	case f.IsRename:
		return StatusRenamed
	case f.IsCopy:
		return StatusCopied
	default:
		return StatusModified
	}
}
