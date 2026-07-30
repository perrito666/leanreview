package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) []FileDiff {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer f.Close()
	files, err := ParsePatch(f)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return files
}

func ptr(n int) *int { return &n }

func TestParseSimple(t *testing.T) {
	files := loadFixture(t, "simple.diff")
	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
	f := files[0]
	if f.NewPath != "internal/api/handler.go" {
		t.Errorf("NewPath = %q", f.NewPath)
	}
	if f.Status != StatusModified {
		t.Errorf("Status = %v, want modified", f.Status)
	}
	if len(f.Hunks) != 2 {
		t.Fatalf("hunks = %d, want 2", len(f.Hunks))
	}

	// The deletion/addition pair in the first hunk.
	h0 := f.Hunks[0]
	var del, add *DiffLine
	for i := range h0.Lines {
		switch h0.Lines[i].Kind {
		case LineDeletion:
			if del == nil {
				del = &h0.Lines[i]
			}
		case LineAddition:
			if add == nil {
				add = &h0.Lines[i]
			}
		}
	}
	if del == nil || add == nil {
		t.Fatalf("expected a deletion and an addition in hunk 0")
	}
	if del.OldLine == nil || *del.OldLine != 72 {
		t.Errorf("deletion OldLine = %v, want 72", del.OldLine)
	}
	if del.NewLine != nil {
		t.Errorf("deletion NewLine = %v, want nil", del.NewLine)
	}
	if add.NewLine == nil || *add.NewLine != 72 {
		t.Errorf("addition NewLine = %v, want 72", add.NewLine)
	}
	if add.OldLine != nil {
		t.Errorf("addition OldLine = %v, want nil", add.OldLine)
	}
}

func TestPatchPositionMonotonic(t *testing.T) {
	f := loadFixture(t, "simple.diff")[0]
	last := 0
	first := true
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.PatchPosition == nil {
				t.Fatalf("line %q has nil PatchPosition", l.Text)
			}
			if first {
				if *l.PatchPosition != 1 {
					t.Errorf("first patch position = %d, want 1", *l.PatchPosition)
				}
				first = false
			}
			if *l.PatchPosition <= last {
				t.Errorf("patch position %d not increasing after %d", *l.PatchPosition, last)
			}
			last = *l.PatchPosition
		}
	}
}

func TestParseRename(t *testing.T) {
	f := loadFixture(t, "rename.diff")[0]
	if !f.IsRenamed || f.Status != StatusRenamed {
		t.Errorf("expected rename, got status %v renamed=%v", f.Status, f.IsRenamed)
	}
	if f.OldPath != "old/name.go" || f.NewPath != "new/name.go" {
		t.Errorf("paths = %q -> %q", f.OldPath, f.NewPath)
	}
}

func TestParseDeleted(t *testing.T) {
	f := loadFixture(t, "deleted-file.diff")[0]
	if f.Status != StatusDeleted {
		t.Errorf("status = %v, want deleted", f.Status)
	}
	if f.Path() != "gone.txt" {
		t.Errorf("path = %q, want gone.txt", f.Path())
	}
	for _, l := range f.Hunks[0].Lines {
		if l.Kind != LineDeletion {
			t.Errorf("line %q kind = %v, want deletion", l.Text, l.Kind)
		}
	}
}

func TestExpandTabs(t *testing.T) {
	cases := []struct{ in, want string }{
		{"no tabs", "no tabs"},
		{"\tx", "    x"},       // one leading tab -> 4 spaces
		{"a\tb", "a   b"},      // tab stop at column 4
		{"ab\tc", "ab  c"},     // partial column fill
		{"\t\tx", "        x"}, // two tabs
	}
	for _, c := range cases {
		if got := expandTabs(c.in, 4); got != c.want {
			t.Errorf("expandTabs(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParsedLinesHaveNoTabs(t *testing.T) {
	f := loadFixture(t, "simple.diff")[0]
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if strings.ContainsRune(l.Text, '\t') {
				t.Errorf("parsed line still contains a tab: %q", l.Text)
			}
		}
	}
}

func TestParseBinary(t *testing.T) {
	files := loadFixture(t, "binary.diff")
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2 (binary + following text file)", len(files))
	}
	bin := files[0]
	if !bin.IsBinary || bin.Status != StatusBinary || len(bin.Hunks) != 0 {
		t.Errorf("binary file = binary:%v status:%v hunks:%d", bin.IsBinary, bin.Status, len(bin.Hunks))
	}
	// A "Binary files differ" entry must not break the file that follows it —
	// local `git diff` emits these by default.
	txt := files[1]
	if txt.Path() != "after.txt" || txt.Status != StatusModified || len(txt.Hunks) != 1 {
		t.Errorf("file after binary mangled: %+v", txt)
	}

	// Binary files render as a one-row placeholder instead of a blank body.
	uni := RenderUnified(&bin)
	if len(uni) != 1 || uni[0].Source != nil || !strings.Contains(uni[0].Left.Text, "binary") {
		t.Errorf("unified binary placeholder = %+v", uni)
	}
	spl := RenderSplit(&bin)
	if len(spl) != 1 || spl[0].Source != nil {
		t.Errorf("split binary placeholder = %+v", spl)
	}
}

func TestParseNoNewline(t *testing.T) {
	f := loadFixture(t, "no-newline.diff")[0]
	var adds, dels int
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			switch l.Kind {
			case LineAddition:
				adds++
			case LineDeletion:
				dels++
			}
		}
	}
	if adds != 1 || dels != 1 {
		t.Errorf("no-newline fixture: %d adds, %d dels, want 1/1", adds, dels)
	}
}

func TestParseMultilineComment(t *testing.T) {
	f := loadFixture(t, "multiline-comment.diff")[0]
	if f.Path() != "doc.go" {
		t.Fatalf("path = %q", f.Path())
	}
	var adds, dels int
	for _, l := range f.Hunks[0].Lines {
		switch l.Kind {
		case LineAddition:
			adds++
		case LineDeletion:
			dels++
		}
	}
	if adds != 4 || dels != 2 {
		t.Errorf("multiline block: %d adds, %d dels, want 4/2", adds, dels)
	}
	// The contiguous added block must be selectable as one continuous range.
	hi, li, ok := f.FindBySideLine(SideRight, 3)
	if !ok {
		t.Fatalf("first added line not found")
	}
	if l := f.LineAt(hi, li); l == nil || l.Kind != LineAddition {
		t.Errorf("line at new-side 3 = %+v, want addition", l)
	}
}

func TestFindBySideLine(t *testing.T) {
	f := loadFixture(t, "simple.diff")[0]
	hi, li, ok := f.FindBySideLine(SideRight, 72)
	if !ok {
		t.Fatalf("did not find new-side line 72")
	}
	l := f.LineAt(hi, li)
	if l == nil || l.Kind != LineAddition {
		t.Errorf("line at (%d,%d) = %+v, want an addition", hi, li, l)
	}
}
