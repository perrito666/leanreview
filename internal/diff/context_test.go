package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contextFixture builds the full new-side file that matches simple.diff's
// hunks: line numbers and text must agree with the hunk lines, and the gaps
// are synthesized filler.
func contextFixture(t *testing.T) (*FileDiff, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "simple.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	files, err := ParsePatchBytes(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := &files[0]

	// Reconstruct the new file: filler everywhere, hunk text where known.
	max := 0
	byLine := map[int]string{}
	for hi := range f.Hunks {
		for _, l := range f.Hunks[hi].Lines {
			if l.NewLine != nil {
				byLine[*l.NewLine] = l.Text
				if *l.NewLine > max {
					max = *l.NewLine
				}
			}
		}
	}
	total := max + 5 // trailing context beyond the last hunk
	var b strings.Builder
	for i := 1; i <= total; i++ {
		if text, ok := byLine[i]; ok {
			b.WriteString(text)
		} else {
			b.WriteString("filler line")
		}
		b.WriteByte('\n')
	}
	return f, []byte(b.String())
}

func TestRenderUnifiedContextCoversWholeFile(t *testing.T) {
	f, content := contextFixture(t)
	rows, err := RenderUnifiedContext(f, content)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	total := len(strings.Split(strings.TrimRight(string(content), "\n"), "\n"))

	// Every new-side line appears exactly once and in order.
	seen := 0
	last := 0
	for _, r := range rows {
		if r.Right != nil && r.Right.LineNumber != nil {
			if *r.Right.LineNumber != last+1 {
				t.Fatalf("new lines out of order: %d after %d", *r.Right.LineNumber, last)
			}
			last = *r.Right.LineNumber
			seen++
		}
	}
	if seen != total {
		t.Errorf("covered %d new lines, file has %d", seen, total)
	}

	// Hunk rows keep Sources; gap rows have none. Deletions are present.
	var hunkRows, gapRows, dels int
	for _, r := range rows {
		switch {
		case r.Source != nil:
			hunkRows++
			if r.Left != nil && r.Left.Kind == LineDeletion {
				dels++
			}
		default:
			gapRows++
		}
	}
	if hunkRows == 0 || gapRows == 0 || dels == 0 {
		t.Errorf("rows: hunk=%d gap=%d dels=%d — context view must contain all three", hunkRows, gapRows, dels)
	}

	// Every hunk is bracketed: a rule + its @@ header on entry, a rule on
	// exit — the reviewed excerpt's extent stays visible inside the file.
	headers, seps := 0, 0
	for _, r := range rows {
		if r.Separator {
			seps++
		}
		if isHeaderLike(&r) {
			headers++
		}
	}
	if headers != len(f.Hunks) {
		t.Errorf("headers = %d, want one per hunk (%d)", headers, len(f.Hunks))
	}
	if seps != 2*len(f.Hunks) {
		t.Errorf("boundary rules = %d, want %d (entry+exit per hunk)", seps, 2*len(f.Hunks))
	}
	// Structure: the row before each header is a rule (this fixture's hunks
	// start mid-file), and each hunk's last sourced row is followed by one.
	for i, r := range rows {
		if isHeaderLike(&r) && (i == 0 || !rows[i-1].Separator) {
			t.Errorf("header at %d not preceded by a boundary rule", i)
		}
	}

	// Old numbering in the tail gap reflects the net line delta.
	adds, delcount := 0, 0
	for hi := range f.Hunks {
		for _, l := range f.Hunks[hi].Lines {
			switch l.Kind {
			case LineAddition:
				adds++
			case LineDeletion:
				delcount++
			}
		}
	}
	lastRow := rows[len(rows)-1]
	wantOld := *lastRow.Right.LineNumber - (adds - delcount)
	if *lastRow.Left.LineNumber != wantOld {
		t.Errorf("tail old number = %d, want %d", *lastRow.Left.LineNumber, wantOld)
	}
}

func TestRenderUnifiedContextRejectsWrongContent(t *testing.T) {
	f, content := contextFixture(t)
	bad := strings.Replace(string(content), "filler", "FILLER", 1)
	// Corrupt a line the hunks actually cover.
	badHunk := []byte(strings.Replace(string(content), "ctx := context.Background()", "tampered", 1))
	if _, err := RenderUnifiedContext(f, badHunk); err == nil {
		t.Errorf("mismatched hunk content must be rejected, not rendered")
	}
	_ = bad
}

func isHeaderLike(r *DisplayRow) bool {
	return r.Left != nil && r.Left.Kind == LineMetadata && r.Right == nil
}

// TestRenderUnifiedContextExpandsTabs is the regression for the guard
// misfiring on tab-indented files: the parser stores hunk text tab-expanded,
// so raw fetched content must be normalized before comparison — otherwise
// every indented line reports "different revision".
func TestRenderUnifiedContextExpandsTabs(t *testing.T) {
	patch := "diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n@@ -1,3 +1,3 @@\n func f() {\n-\tolder()\n+\tnewer()\n }\n"
	files, err := ParsePatchBytes([]byte(patch))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Raw file content with REAL tabs, as git show would return it.
	content := []byte("func f() {\n\tnewer()\n}\n")
	rows, err := RenderUnifiedContext(&files[0], content)
	if err != nil {
		t.Fatalf("tab-indented content rejected: %v", err)
	}
	for _, r := range rows {
		if r.Left != nil && strings.Contains(r.Left.Text, "\t") {
			t.Errorf("row text carries a raw tab: %q", r.Left.Text)
		}
	}
}
