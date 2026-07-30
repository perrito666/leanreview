package app

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/perrito666/leanreview/internal/diff"
)

// selectionRange returns the inclusive row range currently targeted: the
// selection span in visual mode, or the single cursor row otherwise.
func (m *Model) selectionRange() (lo, hi int) {
	if m.mode == ModeVisual && m.selAnchor >= 0 {
		lo, hi = m.selAnchor, m.cursor
		if lo > hi {
			lo, hi = hi, lo
		}
		return lo, hi
	}
	return m.cursor, m.cursor
}

// buildLocation resolves the current selection into a single semantic Location
// plus the diff snippet it covers. It enforces GitHub's constraint that a
// comment maps onto one continuous range on one side; ambiguous selections are
// rejected before the editor opens.
func (m *Model) buildLocation() (diff.Location, string, error) {
	rows := m.rows()
	lo, hi := m.selectionRange()

	var (
		side     diff.Side
		haveSide bool
		nums     []int
		snippets []string
		anchor   *diff.Location
	)
	for i := lo; i <= hi && i < len(rows); i++ {
		r := &rows[i]
		if r.Source == nil {
			continue // header / blank filler: skip, don't fail
		}
		if !haveSide {
			side = r.Source.Side
			haveSide = true
			anchor = r.Source
		} else if r.Source.Side != side {
			return diff.Location{}, "", errors.New("cannot comment on this selection: it crosses both deleted and added sides")
		}
		nums = append(nums, r.Source.StartLine)
		snippets = append(snippets, rowText(r, side))
	}

	if !haveSide {
		return diff.Location{}, "", errors.New("nothing to comment on here")
	}

	sorted := append([]int(nil), nums...)
	sort.Ints(sorted)
	if sorted[len(sorted)-1]-sorted[0]+1 != len(sorted) {
		return diff.Location{}, "", errors.New("cannot comment on this selection: the lines are not a continuous range")
	}

	loc := *anchor
	loc.StartLine = sorted[0]
	loc.EndLine = sorted[len(sorted)-1]
	loc.CommitOID = m.headOID
	return loc, strings.Join(snippets, "\n"), nil
}

// rowText returns the text of the row on the given side.
func rowText(r *diff.DisplayRow, side diff.Side) string {
	if side == diff.SideLeft && r.Left != nil {
		return r.Left.Text
	}
	if side == diff.SideRight && r.Right != nil {
		return r.Right.Text
	}
	// Fall back to whichever side carries text (unified rows share text).
	if r.Right != nil {
		return r.Right.Text
	}
	if r.Left != nil {
		return r.Left.Text
	}
	return ""
}

// lineRefString renders a compact "L72" or "L72-76" reference for a location.
func lineRefString(loc diff.Location) string {
	if loc.Single() {
		return fmt.Sprintf("L%d", loc.StartLine)
	}
	return fmt.Sprintf("L%d-%d", loc.StartLine, loc.EndLine)
}

// commentIDsAt returns the draft comment ids anchored to the row at index i.
func (m *Model) commentIDsAt(i int) []string {
	r := m.rowAt(i)
	if r == nil || r.Source == nil {
		return nil
	}
	return m.draft.CommentsForLine(r.Source.Path, r.Source.Side, r.Source.StartLine)
}
