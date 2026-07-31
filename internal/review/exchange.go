package review

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/perrito666/leanreview/internal/diff"
)

// ExchangeVersion is the current exchange-format version. It is bumped only on
// incompatible changes; readers must reject versions they do not know rather
// than guess.
const ExchangeVersion = 1

// Exchange is the on-disk review-conversation format: a self-contained JSON
// document carrying the diff under review and the comments on it. It exists so
// a review can travel between tools — typically an LLM writes one, a human
// edits it in the leanreview TUI (dismissing, rewording, adding comments), and
// the LLM reads the result back to act on it. Embedding the patch (rather than
// referencing a repo state) keeps the round trip fully offline and
// self-describing.
type Exchange struct {
	// Version gates parsing; see ExchangeVersion. The field name doubles as
	// the sniff marker that distinguishes an exchange file from a plain patch
	// or any other JSON.
	Version int    `json:"leanreview_review"`
	Title   string `json:"title,omitempty"`
	// Summary is the review-level verdict or overview, round-tripped into
	// DraftReview.Summary.
	Summary string `json:"summary,omitempty"`
	// Patch is the unified diff the comments anchor to. Required: without it
	// the comments' line numbers are meaningless to the next reader. On the
	// wire it is an array of lines (see PatchText) so exchange files diff
	// cleanly and editors can highlight the embedded diff.
	Patch    PatchText         `json:"patch"`
	Comments []ExchangeComment `json:"comments"`
}

// PatchText carries the unified diff. Canonically it marshals as a JSON array
// of lines — one diff line per array element — because a single escaped
// string is unreadable to humans, undiffable across round trips, and opaque
// to editor highlighting. Readers are lenient and also accept a plain string,
// so a hand-rolled or older writer still parses.
type PatchText string

// MarshalJSON emits the canonical line-array form. The trailing newline is
// implicit (restored by UnmarshalJSON), so the array never ends with an empty
// element.
func (p PatchText) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.Split(strings.TrimRight(string(p), "\n"), "\n"))
}

// UnmarshalJSON accepts the canonical array-of-lines or a single string.
func (p *PatchText) UnmarshalJSON(b []byte) error {
	var lines []string
	if err := json.Unmarshal(b, &lines); err == nil {
		*p = PatchText(strings.Join(lines, "\n") + "\n")
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("patch must be an array of lines or a string")
	}
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	*p = PatchText(s)
	return nil
}

// ExchangeComment is one comment in the conversation. Line numbers follow the
// same semantics as forge review comments: side LEFT/RIGHT, 1-based lines in
// the pre-/post-image of the embedded patch.
type ExchangeComment struct {
	// ID identifies the comment across round trips so an editor can correlate
	// what changed. Preserved as the draft's local id; assigned when absent.
	ID string `json:"id,omitempty"`
	// Author is a free-form attribution ("assistant", a username); shown in
	// the TUI next to imported comments.
	Author    string `json:"author,omitempty"`
	Path      string `json:"path"`
	Side      string `json:"side"` // "LEFT" or "RIGHT" (case-insensitive)
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line,omitempty"` // defaults to start_line
	Body      string `json:"body"`
	// State carries the human's verdict back: "active" (default),
	// "dismissed" (rejected — do not act on it), or "orphaned" (no longer
	// anchors to the diff).
	State string `json:"state,omitempty"`
	// Snippet is the diff line the comment anchors to; informational for
	// readers, recomputed from the patch on import when empty.
	Snippet string `json:"snippet,omitempty"`
	// At is an optional RFC 3339 creation timestamp (see ReviewReply.At for
	// why it is part of version 1).
	At string `json:"at,omitempty"`
	// Replies is the running conversation on this comment, oldest first.
	Replies []ReviewReply `json:"replies,omitempty"`
}

// ReviewReply is one follow-up message on a comment. At is an optional
// RFC 3339 timestamp: replies accumulate across rounds, and chronology is
// part of the conversation. It exists in version 1 deliberately —
// intermediaries do not preserve unknown fields, so anything a round trip
// must carry has to be in the format from the start.
type ReviewReply struct {
	Author string `json:"author,omitempty"`
	Body   string `json:"body"`
	At     string `json:"at,omitempty"` // RFC 3339
}

// IsExchange sniffs whether data looks like an exchange document rather than a
// patch: JSON object syntax plus the version key near the start. It must be
// cheap and false-negative-free for real exchange files, because source
// resolution uses it to decide how to open a path.
func IsExchange(data []byte) bool {
	head := bytes.TrimLeft(data, " \t\r\n")
	if len(head) == 0 || head[0] != '{' {
		return false
	}
	if len(head) > 4096 {
		head = head[:4096]
	}
	return bytes.Contains(head, []byte(`"leanreview_review"`))
}

// ParseExchange decodes and validates an exchange document. Unknown fields are
// tolerated (forward compatibility for additive changes); an unknown version
// or a missing patch is an error, because silently misreading a conversation
// is worse than refusing it.
func ParseExchange(data []byte) (*Exchange, error) {
	var e Exchange
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse review exchange: %w", err)
	}
	if e.Version != ExchangeVersion {
		return nil, fmt.Errorf("unsupported review exchange version %d (this build reads %d)", e.Version, ExchangeVersion)
	}
	if strings.TrimSpace(string(e.Patch)) == "" {
		return nil, fmt.Errorf("review exchange carries no patch")
	}
	return &e, nil
}

// ToDraft converts the exchange comments into a draft anchored against the
// parsed files of the embedded patch. Comments whose (path, side, line) do not
// resolve in the diff import as orphaned rather than failing the load — an
// LLM-supplied line number can be slightly off, and the reviewer should see
// and fix that, not lose the comment.
func (e *Exchange) ToDraft(sourceKey string, files []diff.FileDiff) *DraftReview {
	d := NewDraftReview(sourceKey, e.Title, "")
	d.Summary = e.Summary
	for _, c := range e.Comments {
		side := parseSide(c.Side)
		end := c.EndLine
		if end == 0 {
			end = c.StartLine
		}
		loc := diff.Location{Path: c.Path, Side: side, StartLine: c.StartLine, EndLine: end}
		snippet := c.Snippet
		state := parseState(c.State)

		if f := diff.FindFileFor(files, c.Path); f != nil {
			if hi, li, ok := f.FindBySideLine(side, c.StartLine); ok {
				loc.Path = f.Path()
				loc.HunkIndex, loc.LineIndex = hi, li
				loc.Anchor = diff.NewContextAnchor(f, hi, li, 3)
				if snippet == "" {
					snippet = f.Hunks[hi].Lines[li].Text
				}
			} else if state != DraftDismissed {
				state = DraftOrphaned
			}
		} else if state != DraftDismissed {
			state = DraftOrphaned
		}

		id := c.ID
		if id == "" {
			id = newLocalID()
		}
		d.Comments = append(d.Comments, DraftComment{
			LocalID:  id,
			Location: loc,
			Body:     c.Body,
			Snippet:  snippet,
			State:    state,
			Author:   c.Author,
			At:       c.At,
			Replies:  c.Replies,
		})
	}
	return d
}

// FromDraft builds the exchange document for a draft, embedding the patch the
// review was made against so the file remains self-contained for the next
// reader in the conversation.
func FromDraft(d *DraftReview, patch []byte) *Exchange {
	e := &Exchange{
		Version: ExchangeVersion,
		Title:   d.Title,
		Summary: d.Summary,
		Patch:   PatchText(patch),
	}
	for _, c := range d.Comments {
		e.Comments = append(e.Comments, ExchangeComment{
			ID:        c.LocalID,
			Author:    c.Author,
			Path:      c.Location.Path,
			Side:      c.Location.Side.String(),
			StartLine: c.Location.StartLine,
			EndLine:   c.Location.EndLine,
			Body:      c.Body,
			State:     c.State.String(),
			Snippet:   c.Snippet,
			At:        c.At,
			Replies:   c.Replies,
		})
	}
	return e
}

// MarshalExchange renders the document with stable indentation so successive
// round trips produce reviewable text diffs — the conversation itself is
// often kept in version control or pasted into chat.
func MarshalExchange(e *Exchange) ([]byte, error) {
	out, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// RenderExport renders the draft for the given destination path: the exchange
// document when the extension is .json (patch required — it is what makes the
// file self-contained), Markdown otherwise. One dispatch point so the CLI
// --export and the TUI :export cannot disagree on the format rule.
func RenderExport(path string, d *DraftReview, patch []byte) ([]byte, error) {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		if len(patch) == 0 {
			return nil, fmt.Errorf("exchange export needs the raw diff, which this source cannot provide")
		}
		return MarshalExchange(FromDraft(d, patch))
	}
	return []byte(ExportMarkdown(d)), nil
}

// parseSide maps the wire spelling onto diff.Side, defaulting to the new side
// — the overwhelmingly common case for review comments — when the value is
// missing or unrecognised.
func parseSide(s string) diff.Side {
	if strings.EqualFold(strings.TrimSpace(s), "LEFT") {
		return diff.SideLeft
	}
	return diff.SideRight
}

// parseState maps the wire state onto DraftState, defaulting to active so a
// minimal hand-written exchange file needs no state field at all.
func parseState(s string) DraftState {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "dismissed":
		return DraftDismissed
	case "orphaned":
		return DraftOrphaned
	case "stale":
		return DraftStale
	default:
		return DraftActive
	}
}
