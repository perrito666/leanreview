package editor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestEditRoundTrip exercises the real exec path with a script standing in for
// $EDITOR: it appends a note to the file, and Edit must return the cleaned body.
func TestEditRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script editor not portable to windows")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-editor.sh")
	// The editor receives the temp file path as its last argument; append a note.
	content := "#!/bin/sh\nprintf 'This is my note.\\n' >> \"$1\"\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	e := Editor{Command: "sh", Args: []string{script}}
	initial := BuildTemplate(TemplateContext{Repository: "owner/repo", File: "x.go", Lines: "10"}, "")
	got, err := Edit(context.Background(), e, initial, "x.go-L10")
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got != "This is my note." {
		t.Errorf("edit result = %q, want %q", got, "This is my note.")
	}
}
