package diff

// RelocateResult reports the outcome of trying to map a Location captured
// against one diff onto a newer diff.
type RelocateResult uint8

const (
	// RelocateExact: the location still resolves at the same place.
	RelocateExact RelocateResult = iota
	// RelocateMoved: the location was uniquely re-found elsewhere.
	RelocateMoved
	// RelocateOrphaned: no unique match exists; a human must reposition it.
	RelocateOrphaned
)

// anchorContext is how many surrounding lines the relocation matcher compares.
const anchorContext = 3

// Relocate attempts to map loc onto the given (newer) set of file diffs using
// the location's captured context anchor. It first checks whether the exact
// side+line still holds the same content; failing that, it searches the file
// (following a rename) for a line with the same anchor text and non-conflicting
// surrounding context, relocating only when the match is unique. When several
// candidates match — or none do — the location is reported as orphaned and left
// unchanged so the reviewer can decide.
func Relocate(files []FileDiff, loc Location) (Location, RelocateResult) {
	f := findFileFor(files, loc.Path)
	if f == nil {
		return loc, RelocateOrphaned
	}

	// 1. Still exactly here?
	if hi, li, ok := f.FindBySideLine(loc.Side, loc.StartLine); ok {
		if f.Hunks[hi].Lines[li].Text == loc.Anchor.AnchorText &&
			contextMatches(loc.Anchor, NewContextAnchor(f, hi, li, anchorContext)) {
			out := loc
			out.Path = f.Path()
			out.HunkIndex, out.LineIndex = hi, li
			return out, RelocateExact
		}
	}

	// 2. Unique context match elsewhere in the file.
	type cand struct{ hi, li, line int }
	var matches []cand
	for hi := range f.Hunks {
		for li := range f.Hunks[hi].Lines {
			l := &f.Hunks[hi].Lines[li]
			n, ok := l.LineNumber(loc.Side)
			if !ok || l.Text != loc.Anchor.AnchorText {
				continue
			}
			if contextMatches(loc.Anchor, NewContextAnchor(f, hi, li, anchorContext)) {
				matches = append(matches, cand{hi, li, n})
			}
		}
	}
	if len(matches) == 1 {
		m := matches[0]
		span := loc.EndLine - loc.StartLine
		out := loc
		out.Path = f.Path()
		out.StartLine = m.line
		out.EndLine = m.line + span
		out.HunkIndex, out.LineIndex = m.hi, m.li
		return out, RelocateMoved
	}
	return loc, RelocateOrphaned
}

// findFileFor returns the file whose old or new path matches path (so a comment
// made before a rename still finds its file).
func findFileFor(files []FileDiff, path string) *FileDiff {
	for i := range files {
		if files[i].NewPath == path || files[i].OldPath == path || files[i].Path() == path {
			return &files[i]
		}
	}
	return nil
}

// contextMatches reports whether two anchors are compatible: the anchor text is
// assumed equal by the caller, and every overlapping context line (aligning
// Before by its tail and After by its head) must agree. Non-overlapping context
// at hunk edges is not a conflict.
func contextMatches(a, b ContextAnchor) bool {
	return tailEqual(a.Before, b.Before) && headEqual(a.After, b.After)
}

func tailEqual(a, b []string) bool {
	n := min(len(a), len(b))
	for i := 1; i <= n; i++ {
		if a[len(a)-i] != b[len(b)-i] {
			return false
		}
	}
	return true
}

func headEqual(a, b []string) bool {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
