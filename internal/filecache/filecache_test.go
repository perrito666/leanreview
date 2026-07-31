package filecache

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPutGetRoundTrip(t *testing.T) {
	c, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put("key/with:odd chars", []byte("content")); err != nil {
		t.Fatal(err)
	}
	got, ok := c.Get("key/with:odd chars")
	if !ok || string(got) != "content" {
		t.Errorf("get = %q, %v", got, ok)
	}
	if _, ok := c.Get("absent"); ok {
		t.Errorf("absent key reported as hit")
	}
}

func TestCleanupByAge(t *testing.T) {
	dir := t.TempDir()
	c, _ := OpenAt(dir)
	c.Put("old", []byte("x"))
	c.Put("fresh", []byte("y"))
	old := time.Now().Add(-MaxAge - time.Hour)
	os.Chtimes(c.path("old"), old, old)

	// Reopening is app start: cleanup runs.
	c2, _ := OpenAt(dir)
	if _, ok := c2.Get("old"); ok {
		t.Errorf("entry older than MaxAge survived cleanup")
	}
	if _, ok := c2.Get("fresh"); !ok {
		t.Errorf("fresh entry evicted by age cleanup")
	}
}

func TestCleanupBySizeOldestFirst(t *testing.T) {
	dir := t.TempDir()
	c, _ := OpenAt(dir)
	// Three ~30MiB entries exceed the 50MiB budget; the two oldest must go.
	big := make([]byte, 30<<20)
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("f%d", i)
		if err := c.Put(key, big); err != nil {
			t.Fatal(err)
		}
		mod := time.Now().Add(time.Duration(i-3) * time.Hour)
		os.Chtimes(c.path(key), mod, mod)
	}
	c2, _ := OpenAt(dir)
	if _, ok := c2.Get("f2"); !ok {
		t.Errorf("newest entry evicted; size cleanup must drop oldest first")
	}
	if _, ok := c2.Get("f0"); ok {
		t.Errorf("oldest entry survived a size overflow")
	}
	// Total must now be under budget.
	var total int64
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		info, _ := e.Info()
		total += info.Size()
	}
	if total > MaxBytes {
		t.Errorf("cache still over budget after cleanup: %d", total)
	}
}

func TestGetRefreshesAge(t *testing.T) {
	dir := t.TempDir()
	c, _ := OpenAt(dir)
	c.Put("k", []byte("v"))
	old := time.Now().Add(-2 * time.Hour)
	os.Chtimes(c.path("k"), old, old)
	c.Get("k")
	info, err := os.Stat(filepath.Join(dir, filepath.Base(c.path("k"))))
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(info.ModTime()) > time.Minute {
		t.Errorf("Get did not refresh mtime; LRU ordering would evict hot entries")
	}
}
