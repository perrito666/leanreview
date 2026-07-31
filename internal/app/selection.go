package app

import (
	"errors"
	"fmt"
	"slices"
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
		src := m.sourceForRow(r)
		if src == nil {
			continue // header / blank filler: skip, don't fail
		}
		if !haveSide {
			side = src.Side
			haveSide = true
			anchor = src
		} else if src.Side != side {
			return diff.Location{}, "", errors.New("cannot comment on this selection: it crosses both deleted and added sides")
		}
		nums = append(nums, src.StartLine)
		snippets = append(snippets, rowText(r, side))
	}

	if !haveSide {
		return diff.Location{}, "", errors.New("nothing to comment on here")
	}

	sorted := append([]int(nil), nums...)
	slices.Sort(sorted)
	if sorted[len(sorted)-1]-sorted[0]+1 != len(sorted) {
		return diff.Location{}, "", errors.New("cannot comment on this selection: the lines are not a continuous range")
	}

	loc := *anchor
	loc.StartLine = sorted[0]
	loc.EndLine = sorted[len(sorted)-1]
	loc.CommitOID = m.headOID
	return loc, strings.Join(snippets, "\n"), nil
}

// sourceForRow resolves which side's location a row contributes: in split
// layout with the left side active, a both-sided row yields its AltSource
// (the old side); otherwise the primary Source.
func (m *Model) sourceForRow(r *diff.DisplayRow) *diff.Location {
	if r == nil {
		return nil
	}
	if m.layout == LayoutSplit && m.activeSide == diff.SideLeft && r.AltSource != nil {
		return r.AltSource
	}
	return r.Source
}

// setActiveSide switches the split-view side the cursor targets.
func (m *Model) setActiveSide(side diff.Side) {
	if m.layout != LayoutSplit {
		m.setStatus("side selection applies to split view (press t)")
		return
	}
	m.activeSide = side
	m.setStatus("targeting %s side", side)
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

// commentIDsAt returns the draft comment ids anchored to the row at index i,
// checking both sides of a both-sided split row so left-side comments keep
// their markers.
func (m *Model) commentIDsAt(i int) []string {
	r := m.rowAt(i)
	if r == nil {
		return nil
	}
	var ids []string
	if r.Source != nil {
		ids = append(ids, m.draft.CommentsForLine(r.Source.Path, r.Source.Side, r.Source.StartLine)...)
	}
	if r.AltSource != nil {
		ids = append(ids, m.draft.CommentsForLine(r.AltSource.Path, r.AltSource.Side, r.AltSource.StartLine)...)
	}
	return ids
}
