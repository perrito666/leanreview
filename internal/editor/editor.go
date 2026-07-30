// Package editor launches the user's external editor to compose review
// comments. Editor variables routinely carry arguments (e.g. "nvim -f",
// "code --wait"), so the command line is parsed into a program plus arguments
// rather than handed to a shell. The actual process launch is performed by the
// caller (the TUI uses tea.ExecProcess so the alternate screen is released and
// restored correctly); this package builds the *exec.Cmd and manages the temp
// file and template stripping.
package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Editor is a resolved editor invocation: a program and any fixed arguments
// that precede the file path.
type Editor struct {
	Command string
	Args    []string
}

// Resolve picks an editor using the precedence:
//
//	configured (review.editor) → GIT_EDITOR → VISUAL → EDITOR →
//	`git var GIT_EDITOR` → platform fallback (vi / notepad).
//
// The first non-empty source wins. The chosen value is parsed as a command
// line, so arguments are preserved.
func Resolve(configured string) (Editor, error) {
	candidates := []string{
		configured,
		os.Getenv("GIT_EDITOR"),
		os.Getenv("VISUAL"),
		os.Getenv("EDITOR"),
		gitVarEditor(),
	}
	for _, c := range candidates {
		if strings.TrimSpace(c) == "" {
			continue
		}
		parts := SplitCommand(c)
		if len(parts) == 0 {
			continue
		}
		return Editor{Command: parts[0], Args: parts[1:]}, nil
	}
	return Editor{Command: platformFallback()}, nil
}

// Cmd builds (but does not start) the command that opens path in the editor,
// wired to the current terminal's stdio.
func (e Editor) Cmd(ctx context.Context, path string) *exec.Cmd {
	args := append(append([]string(nil), e.Args...), path)
	cmd := exec.CommandContext(ctx, e.Command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Session owns a temporary Markdown file for one editing round.
type Session struct {
	Path string
	dir  string
}

// NewSession writes initial content to a fresh temp .md file whose name carries
// context (so the editor's title bar is useful).
func NewSession(initial, name string) (*Session, error) {
	dir, err := os.MkdirTemp("", "leanreview-editor-*")
	if err != nil {
		return nil, fmt.Errorf("create editor directory: %w", err)
	}
	path := filepath.Join(dir, sanitizeFilename(name)+".md")
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("write editor file: %w", err)
	}
	return &Session{Path: path, dir: dir}, nil
}

// Result reads the edited file and strips the HTML-comment template, returning
// the reviewer's note. An empty result means the comment was abandoned.
func (s *Session) Result() (string, error) {
	body, err := os.ReadFile(s.Path)
	if err != nil {
		return "", fmt.Errorf("read editor file: %w", err)
	}
	return CleanTemplate(string(body)), nil
}

// Close removes the whole temporary directory, not just the file: the
// directory is per-session, and some editors drop swap/backup files next to
// the buffer that would otherwise be left behind. Safe to call on a
// zero-value Session.
func (s *Session) Close() error {
	if s.dir == "" {
		return nil
	}
	return os.RemoveAll(s.dir)
}

// Edit is a convenience blocking helper for non-TUI callers: it opens the
// editor, waits, and returns the cleaned result.
func Edit(ctx context.Context, e Editor, initial, name string) (string, error) {
	s, err := NewSession(initial, name)
	if err != nil {
		return "", err
	}
	defer s.Close()
	if err := e.Cmd(ctx, s.Path).Run(); err != nil {
		return "", fmt.Errorf("run editor: %w", err)
	}
	return s.Result()
}

// gitVarEditor asks `git var GIT_EDITOR` for git's own editor resolution,
// which folds in core.editor from git config — a source the environment
// checks above cannot see. Errors (git missing, not installed) yield "" so
// the chain simply moves on to the platform fallback.
func gitVarEditor() string {
	out, err := exec.Command("git", "var", "GIT_EDITOR").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// platformFallback is the editor of last resort when nothing is configured
// anywhere: the same defaults git itself assumes, chosen for being present
// on a stock install (vi on Unix, notepad on Windows).
func platformFallback() string {
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vi"
}

// sanitizeFilename makes a session name safe to use as a single file name.
// Names are built from review context ("file.go-L10:12"), so path separators,
// Windows-reserved punctuation, and spaces are flattened to '-' — the name
// only exists to make the editor's title bar informative, so lossy
// replacement is fine.
func sanitizeFilename(name string) string {
	if name == "" {
		return "comment"
	}
	repl := func(r rune) rune {
		switch r {
		case '/', '\\', ':', ' ', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return r
	}
	return strings.Map(repl, name)
}

// SplitCommand tokenises a command line with minimal shell semantics: it
// respects single and double quotes and backslash escapes, splitting on
// unquoted whitespace. It intentionally does not expand variables or globs.
func SplitCommand(s string) []string {
	var (
		tokens []string
		cur    strings.Builder
		inTok  bool
		quote  rune
	)
	flush := func() {
		if inTok {
			tokens = append(tokens, cur.String())
			cur.Reset()
			inTok = false
		}
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else if c == '\\' && quote == '"' && i+1 < len(runes) {
				i++
				cur.WriteRune(runes[i])
			} else {
				cur.WriteRune(c)
			}
			inTok = true
		case c == '\'' || c == '"':
			quote = c
			inTok = true
		case c == '\\' && i+1 < len(runes):
			i++
			cur.WriteRune(runes[i])
			inTok = true
		case c == ' ' || c == '\t' || c == '\n':
			flush()
		default:
			cur.WriteRune(c)
			inTok = true
		}
	}
	flush()
	return tokens
}
