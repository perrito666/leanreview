package diff

// LineNumber returns the line number a DiffLine carries on the given side, or
// (0, false) if the line does not exist on that side (e.g. the new side of a
// deletion).
func (l *DiffLine) LineNumber(side Side) (int, bool) {
	if side == SideLeft {
		if l.OldLine != nil {
			return *l.OldLine, true
		}
		return 0, false
	}
	if l.NewLine != nil {
		return *l.NewLine, true
	}
	return 0, false
}

// FindBySideLine locates the (hunkIndex, lineIndex) of the line that occupies
// the given number on the given side. It returns (-1, -1, false) when no such
// line is present in the diff.
func (f *FileDiff) FindBySideLine(side Side, number int) (hunkIndex, lineIndex int, ok bool) {
	for hi := range f.Hunks {
		for li := range f.Hunks[hi].Lines {
			if n, present := f.Hunks[hi].Lines[li].LineNumber(side); present && n == number {
				return hi, li, true
			}
		}
	}
	return -1, -1, false
}

// Changed reports whether the line is an addition or deletion.
func (l *DiffLine) Changed() bool {
	return l.Kind == LineAddition || l.Kind == LineDeletion
}
