package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store persists draft reviews as JSON under an XDG state directory. Drafts are
// namespaced by the source key so a patch file, a local branch diff, and (later)
// a PR each keep independent drafts.
type Store struct {
	Dir string
}

// DefaultStore resolves the state directory ($XDG_STATE_HOME/leanreview/drafts,
// falling back to ~/.local/state/leanreview/drafts).
func DefaultStore() (*Store, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	return &Store{Dir: filepath.Join(base, "leanreview", "drafts")}, nil
}

// Path returns the on-disk file for a source key.
func (s *Store) Path(key string) string {
	return filepath.Join(s.Dir, key+".json")
}

// Load reads the draft for key. It returns (nil, nil) when none exists.
func (s *Store) Load(key string) (*DraftReview, error) {
	data, err := os.ReadFile(s.Path(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read draft: %w", err)
	}
	var d DraftReview
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("decode draft %s: %w", s.Path(key), err)
	}
	return &d, nil
}

// Save writes the draft atomically (temp file + rename).
func (s *Store) Save(d *DraftReview) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("encode draft: %w", err)
	}
	path := s.Path(d.SourceKey)
	tmp, err := os.CreateTemp(s.Dir, ".draft-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp draft: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp draft: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp draft: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("commit draft: %w", err)
	}
	return nil
}

// Delete removes a draft file if present.
func (s *Store) Delete(key string) error {
	err := os.Remove(s.Path(key))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
