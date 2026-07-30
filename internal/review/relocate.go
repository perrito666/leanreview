package review

import "github.com/perrito666/leanreview/internal/diff"

// RelocationSummary reports how a draft's comments fared when re-anchored to a
// newer diff.
type RelocationSummary struct {
	Unchanged int
	Moved     int
	Orphaned  int
}

// Changed reports whether any comment moved or was orphaned.
func (s RelocationSummary) Changed() bool { return s.Moved > 0 || s.Orphaned > 0 }

// RelocateDrafts re-anchors every line comment in the draft against the current
// diff and records the new head. Line comments that still resolve (exactly or
// uniquely) are marked active with their updated location; those with no unique
// match are marked orphaned and left for the reviewer to reposition. Replies are
// keyed by the parent comment's identity, not a line, so they are left alone.
func RelocateDrafts(d *DraftReview, files []diff.FileDiff, newHead string) RelocationSummary {
	var s RelocationSummary
	for i := range d.Comments {
		c := &d.Comments[i]
		if c.ReplyTo != nil {
			continue
		}
		newLoc, res := diff.Relocate(files, c.Location)
		switch res {
		case diff.RelocateExact:
			c.Location = newLoc
			c.State = DraftActive
			s.Unchanged++
		case diff.RelocateMoved:
			c.Location = newLoc
			c.State = DraftActive
			s.Moved++
		default:
			c.State = DraftOrphaned
			s.Orphaned++
		}
	}
	if newHead != "" {
		d.HeadOID = newHead
	}
	return s
}
