// Package filecache stores fetched full-file contents (the context view's
// raw material) under the XDG cache directory. Entries are keyed by content
// identity — a git blob hash, or a PR head commit plus path — so a file that
// changes upstream simply produces a new key and the stale entry ages out;
// nothing ever needs to be invalidated in place.
package filecache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Cleanup limits. Both are enforced on every Open: entries older than MaxAge
// are dropped, then the oldest survivors are dropped until the total is under
// MaxBytes — whichever constraint bites first, bites.
const (
	MaxAge   = 14 * 24 * time.Hour
	MaxBytes = 50 << 20 // 50 MiB
)

// Cache is a directory of content-addressed files.
type Cache struct {
	dir string
}

// Open resolves the default cache directory ($XDG_CACHE_HOME/leanreview/files,
// ~/.cache fallback) and runs cleanup. Errors are returned rather than
// tolerated: a broken cache dir should disable caching loudly at the one
// caller, not fail on every Put.
func Open() (*Cache, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, ".cache")
	}
	return OpenAt(filepath.Join(base, "leanreview", "files"))
}

// OpenAt opens (creating if needed) a cache rooted at dir and runs cleanup.
// Split from Open so tests can point it at a temp directory.
func OpenAt(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	c := &Cache{dir: dir}
	c.cleanup(time.Now())
	return c, nil
}

// path maps a key to its file: a hash, so keys can carry any characters
// (paths, refs) without filesystem constraints leaking into key design.
func (c *Cache) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:16]))
}

// Get returns the cached content for key. A hit refreshes the entry's mtime,
// so cleanup's age/size ordering approximates least-recently-used rather than
// least-recently-written.
func (c *Cache) Get(key string) ([]byte, bool) {
	p := c.path(key)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(p, now, now)
	return data, true
}

// Put stores content under key, atomically (write + rename) so a crash can
// never leave a truncated entry that Get would serve as a real file.
func (c *Cache) Put(key string, data []byte) error {
	p := c.path(key)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// cleanup enforces MaxAge then MaxBytes, oldest first. It runs at Open — app
// start — because that is the one moment cleanup cost is invisible and the
// cache is guaranteed quiescent.
func (c *Cache) cleanup(now time.Time) {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type entry struct {
		path string
		mod  time.Time
		size int64
	}
	var files []entry
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || e.IsDir() {
			continue
		}
		p := filepath.Join(c.dir, e.Name())
		if now.Sub(info.ModTime()) > MaxAge {
			_ = os.Remove(p)
			continue
		}
		files = append(files, entry{p, info.ModTime(), info.Size()})
		total += info.Size()
	}
	if total <= MaxBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for _, f := range files {
		if total <= MaxBytes {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}
