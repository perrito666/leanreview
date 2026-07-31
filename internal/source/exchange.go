package source

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/review"
)

// ExchangeSource is a ReviewSource backed by a review-exchange file: the diff
// under review is the patch embedded in the document, and the document's
// comments seed the draft. It keeps the file path so the session can write the
// updated conversation back to the same place — the file, not the draft
// store, is the medium the two sides of the conversation share.
type ExchangeSource struct {
	path     string
	title    string
	key      string
	exchange *review.Exchange
}

// newExchangeSource wraps an already-sniffed exchange document. The key hashes
// the absolute path only — the file's content changes on every round trip by
// design, and hashing it would detach the draft from its source each time.
func newExchangeSource(path string, data []byte) (*ExchangeSource, error) {
	e, err := review.ParseExchange(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	abs, _ := filepath.Abs(path)
	title := e.Title
	if title == "" {
		title = filepath.Base(path)
	}
	return &ExchangeSource{
		path:     abs,
		title:    title,
		key:      "exchange-" + hashString(abs),
		exchange: e,
	}, nil
}

// Files parses the embedded patch — the only diff the conversation's line
// numbers are meaningful against.
func (s *ExchangeSource) Files(context.Context) ([]diff.FileDiff, error) {
	return diff.ParsePatchBytes([]byte(s.exchange.Patch))
}

// Title is the document's title, or the filename when it has none.
func (s *ExchangeSource) Title() string { return s.title }

// Key seeds the draft store's filename; path-based so round trips resume.
func (s *ExchangeSource) Key() string { return s.key }

// HeadOID is empty: the embedded patch carries no commit identity, and
// relocation across patch versions happens on import instead.
func (s *ExchangeSource) HeadOID(context.Context) string { return "" }

// RawPatch returns the embedded patch for re-embedding on writeback.
func (s *ExchangeSource) RawPatch(context.Context) ([]byte, error) {
	return []byte(s.exchange.Patch), nil
}

// Exchange exposes the parsed document so the CLI can seed the draft from it.
func (s *ExchangeSource) Exchange() *review.Exchange { return s.exchange }

// Path is where the conversation lives on disk — the writeback target.
func (s *ExchangeSource) Path() string { return s.path }

// RawPatcher is implemented by sources that can hand out the raw unified
// patch they were built from. The review-exchange export needs the literal
// patch text (parsed FileDiffs cannot be serialised back verbatim), so
// exchange export is available exactly where this interface is.
type RawPatcher interface {
	RawPatch(ctx context.Context) ([]byte, error)
}
