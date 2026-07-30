// Command leanreview is a terminal code-review client. It reviews a patch/diff
// file or a local git comparison (PR mode arrives in a later milestone),
// letting you navigate the diff, attach draft comments anchored to semantic
// locations, and export them as Markdown for prompt feedback.
//
// Usage:
//
//	leanreview [flags] [target]
//	leanreview review [flags] [target]   (the "review" verb is optional)
//
// target is one of:
//
//	<file.diff>     a patch/diff file ("-" reads stdin)
//	.  (or empty)   the local working tree vs HEAD
//	<revA> <revB>   an explicit git revision range
//
// Flags:
//
//	--base <ref>     compare <ref>...HEAD (merge-base) instead of the working tree
//	--staged         compare the index against HEAD
//	-U, --context N  unified context lines (default 3)
//	--export FILE    non-interactive: write existing draft comments as Markdown and exit
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/perrito666/leanreview/internal/app"
	"github.com/perrito666/leanreview/internal/config"
	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/forge/ghcli"
	"github.com/perrito666/leanreview/internal/review"
	"github.com/perrito666/leanreview/internal/source"
	"github.com/perrito666/leanreview/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "leanreview:", err)
		os.Exit(1)
	}
}

type options struct {
	base       string
	staged     bool
	contextN   int
	exportPath string
	args       []string
}

func run(argv []string) error {
	// Strip an optional leading "review" verb for ergonomic invocation.
	if len(argv) > 0 && argv[0] == "review" {
		argv = argv[1:]
	}

	opts, err := parseArgs(argv)
	if err == errHelp {
		fmt.Print(usage)
		return nil
	}
	if err != nil {
		return err
	}

	cfg := config.Load()
	logFile := setupLogging(cfg.LogPath)
	if logFile != nil {
		defer logFile.Close()
	}

	ctx := context.Background()

	src, prSrc, err := resolveSource(ctx, opts)
	if err != nil {
		return err
	}

	files, err := src.Files(ctx)
	if err != nil {
		return err
	}

	store, err := review.DefaultStore()
	if err != nil {
		return err
	}

	// Load or create the draft for this source.
	draft, err := store.Load(src.Key())
	if err != nil {
		log.Printf("load draft: %v", err)
	}
	headOID := src.HeadOID(ctx)
	if draft == nil {
		draft = review.NewDraftReview(src.Key(), src.Title(), headOID)
	} else {
		draft.SourceKey = src.Key()
		if draft.Title == "" {
			draft.Title = src.Title()
		}
		markStaleIfHeadChanged(draft, headOID)
	}

	// Non-interactive export: write drafts as Markdown and exit.
	if opts.exportPath != "" {
		md := review.ExportMarkdown(draft)
		if err := os.WriteFile(opts.exportPath, []byte(md), 0o644); err != nil {
			return fmt.Errorf("export: %w", err)
		}
		abs, _ := filepath.Abs(opts.exportPath)
		fmt.Printf("exported %d comment(s) to %s\n", len(draft.Comments), abs)
		return nil
	}

	if len(files) == 0 {
		fmt.Println("No changes to review.")
		return nil
	}

	ed, err := editor.Resolve(cfg.Editor)
	if err != nil {
		return err
	}

	var prCtx *app.PRContext
	if prSrc != nil {
		threads, err := prSrc.Threads(ctx)
		if err != nil {
			log.Printf("fetch threads: %v", err)
		}
		prCtx = &app.PRContext{
			Forge:   prSrc.Forge(),
			Ref:     prSrc.Ref(),
			PR:      prSrc.PullRequest(),
			Threads: threads,
		}
	}

	model := app.New(app.Config{
		Files:   files,
		Title:   src.Title(),
		HeadOID: headOID,
		Draft:   draft,
		Store:   store,
		Editor:  ed,
		Theme:   ui.DefaultTheme(),
		PR:      prCtx,
	})

	prog := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}

// resolveSource picks between a GitHub pull request and the local/patch
// resolver. A single argument that parses as a PR reference and is not an
// existing file is treated as PR mode; owner/repo are inferred from origin when
// only a number was given. Returns the generic source plus the concrete
// *PRSource (nil in local/patch mode) for the TUI to use in PR mode.
func resolveSource(ctx context.Context, opts options) (source.ReviewSource, *source.PRSource, error) {
	if len(opts.args) == 1 && !fileExists(opts.args[0]) {
		if ref, ok := forge.ParseRef(opts.args[0]); ok {
			ref, err := source.InferPRRef(ctx, "", ref)
			if err != nil {
				return nil, nil, err
			}
			prSrc, err := source.NewPRSource(ctx, ghcli.New(), ref)
			if err != nil {
				return nil, nil, err
			}
			return prSrc, prSrc, nil
		}
	}

	src, err := source.Resolve(ctx, source.Options{
		Args:    opts.args,
		Base:    opts.base,
		Staged:  opts.staged,
		Context: opts.contextN,
		Stdin:   os.Stdin,
	})
	return src, nil, err
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// parseArgs performs a small hand-rolled parse so flags and positionals can be
// interleaved (the stdlib flag package stops at the first positional).
func parseArgs(argv []string) (options, error) {
	var o options
	o.contextN = 3
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--base":
			v, err := next(argv, &i, "--base")
			if err != nil {
				return o, err
			}
			o.base = v
		case a == "--staged":
			o.staged = true
		case a == "-U" || a == "--context":
			v, err := next(argv, &i, a)
			if err != nil {
				return o, err
			}
			n, err := atoi(v)
			if err != nil {
				return o, fmt.Errorf("%s: %w", a, err)
			}
			o.contextN = n
		case a == "--export":
			v, err := next(argv, &i, "--export")
			if err != nil {
				return o, err
			}
			o.exportPath = v
		case a == "-h" || a == "--help":
			return o, errHelp
		case len(a) > 1 && a[0] == '-' && a != "-":
			return o, fmt.Errorf("unknown flag: %s", a)
		default:
			o.args = append(o.args, a)
		}
	}
	return o, nil
}

var errHelp = fmt.Errorf("help requested")

const usage = `leanreview — terminal code-review client

Usage:
  leanreview [flags] [target]
  leanreview review [flags] [target]

Target:
  <file.diff>     a patch/diff file ("-" reads stdin)
  .  (or empty)   the local working tree vs HEAD
  <revA> <revB>   an explicit git revision range

Flags:
  --base <ref>       compare <ref>...HEAD instead of the working tree
  --staged           compare the index against HEAD
  -U, --context N    unified context lines (default 3)
  --export FILE      write existing draft comments as Markdown and exit
  -h, --help         show this help

In the TUI, press ? for the key reference.
`

func next(argv []string, i *int, flag string) (string, error) {
	if *i+1 >= len(argv) {
		return "", fmt.Errorf("%s requires a value", flag)
	}
	*i++
	return argv[*i], nil
}

func atoi(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// markStaleIfHeadChanged flags all comments as potentially stale when the head
// commit the draft was captured against no longer matches. Full relocation
// arrives in Milestone 3; for now this just surfaces the risk.
func markStaleIfHeadChanged(d *review.DraftReview, headOID string) {
	if headOID == "" || d.HeadOID == "" || d.HeadOID == headOID {
		return
	}
	for i := range d.Comments {
		if d.Comments[i].State == review.DraftActive {
			d.Comments[i].State = review.DraftStale
		}
	}
	d.HeadOID = headOID
}

func setupLogging(path string) *os.File {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	log.SetOutput(f)
	return f
}
