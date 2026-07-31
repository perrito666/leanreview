// Package review holds review state: the draft comments a reviewer accumulates
// before submitting, their persistence, and their export as Markdown for the
// prompt-feedback workflow. Comments anchor to semantic diff locations
// (diff.Location), never to terminal rows.
package review

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/forge"
)

// DraftState tracks whether a draft comment still maps cleanly onto the diff.
type DraftState uint8

const (
	// DraftActive: the location resolves exactly in the current diff.
	DraftActive DraftState = iota
	// DraftStale: the head changed; the comment may need relocation.
	DraftStale
	// DraftOrphaned: the location no longer resolves and needs manual repositioning.
	DraftOrphaned
	// DraftDismissed: a human rejected the comment. It is kept (so the other
	// side of a review conversation can see the verdict) but never submitted.
	DraftDismissed
)

// String returns the lowercase state label shown next to a comment in the
// annotation list and the Markdown export, flagging drafts that need attention
// before submission.
func (s DraftState) String() string {
	switch s {
	case DraftStale:
		return "stale"
	case DraftOrphaned:
		return "orphaned"
	case DraftDismissed:
		return "dismissed"
	default:
		return "active"
	}
}

// DraftComment is one unsubmitted comment.
type DraftComment struct {
	LocalID  string        `json:"local_id"`
	Location diff.Location `json:"location"`
	Body     string        `json:"body"`

	// ReplyTo, when set, is the host comment id this draft replies to (PR mode).
	ReplyTo *int64 `json:"reply_to,omitempty"`

	// Snippet is the diff text the comment was made on, captured at creation so
	// export does not depend on the diff still being loaded.
	Snippet string     `json:"snippet"`
	State   DraftState `json:"state"`

	// Author attributes an imported comment (review-exchange conversations:
	// "assistant", a username). Empty for comments created in this TUI.
	Author string `json:"author,omitempty"`
	// Replies is the running exchange conversation on this comment. Distinct
	// from ReplyTo (a PR-mode reply to a host thread): these travel with the
	// comment through the exchange file.
	Replies []ReviewReply `json:"replies,omitempty"`
}

// DraftReview is the full set of pending comments for one source, plus the
// summary and event that will accompany submission (PR mode).
type DraftReview struct {
	SourceKey string            `json:"source_key"`
	Title     string            `json:"title"`
	HeadOID   string            `json:"head_oid"`
	Comments  []DraftComment    `json:"comments"`
	Summary   string            `json:"summary"`
	Event     forge.ReviewEvent `json:"event"`
}

// NewDraftReview creates an empty draft for a source.
func NewDraftReview(sourceKey, title, headOID string) *DraftReview {
	return &DraftReview{
		SourceKey: sourceKey,
		Title:     title,
		HeadOID:   headOID,
		Event:     forge.EventComment,
	}
}

// Add appends a comment, assigning it a fresh local id, and returns the id.
func (d *DraftReview) Add(loc diff.Location, body, snippet string) string {
	id := newLocalID()
	d.Comments = append(d.Comments, DraftComment{
		LocalID:  id,
		Location: loc,
		Body:     body,
		Snippet:  snippet,
		State:    DraftActive,
	})
	return id
}

// Get returns a pointer to the comment with the given id, or nil.
func (d *DraftReview) Get(localID string) *DraftComment {
	for i := range d.Comments {
		if d.Comments[i].LocalID == localID {
			return &d.Comments[i]
		}
	}
	return nil
}

// Remove deletes the comment with the given id; it reports whether one was removed.
func (d *DraftReview) Remove(localID string) bool {
	for i := range d.Comments {
		if d.Comments[i].LocalID == localID {
			d.Comments = append(d.Comments[:i], d.Comments[i+1:]...)
			return true
		}
	}
	return false
}

// CommentsForLine returns the ids of comments anchored to a given path+side+line.
func (d *DraftReview) CommentsForLine(path string, side diff.Side, line int) []string {
	var ids []string
	for i := range d.Comments {
		c := &d.Comments[i]
		if c.Location.Path == path && c.Location.Side == side && line >= c.Location.StartLine && line <= c.Location.EndLine {
			ids = append(ids, c.LocalID)
		}
	}
	return ids
}

// newLocalID returns a random 16-hex-digit id. Drafts exist before the host
// knows about them, so comments need an identity of their own that survives
// persistence and relocation; randomness (rather than a counter) keeps ids
// unique even across separately edited draft files. rand.Read is documented
// never to fail, hence the ignored error.
func newLocalID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
