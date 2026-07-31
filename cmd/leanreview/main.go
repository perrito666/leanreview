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
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"github.com/perrito666/leanreview/internal/app"
	"github.com/perrito666/leanreview/internal/config"
	"github.com/perrito666/leanreview/internal/diff"
	"github.com/perrito666/leanreview/internal/editor"
	"github.com/perrito666/leanreview/internal/filecache"
	"github.com/perrito666/leanreview/internal/forge"
	"github.com/perrito666/leanreview/internal/forge/ghcli"
	"github.com/perrito666/leanreview/internal/forge/glabcli"
	"github.com/perrito666/leanreview/internal/review"
	"github.com/perrito666/leanreview/internal/source"
	"github.com/perrito666/leanreview/internal/ui"
)

// version is the build version, overridden at release time via
// -ldflags "-X main.version=...".
var version = "dev"

// main is a thin shim over run so all error paths funnel through a single
// exit point with a consistent "leanreview:" prefix on stderr.
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "leanreview:", err)
		os.Exit(1)
	}
}

// options is the result of parseArgs: flag values plus the remaining
// positionals. contextSet distinguishes an explicit -U from the default so
// the config file's context value only applies when the flag was absent.
type options struct {
	base        string
	staged      bool
	contextN    int
	contextSet  bool
	exportPath  string
	discard     bool
	list        bool
	initConfig  bool
	checkConfig bool
	args        []string
}

// run is the real entry point, returning errors instead of exiting so main
// stays testable and cleanup (deferred log file close) runs. It wires the
// whole pipeline: parse flags, load config, resolve the review source,
// load/relocate the persisted draft, then either handle a non-interactive
// mode (--list piped, --discard, --export) or start the TUI.
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
	if err == errVersion {
		fmt.Printf("leanreview %s\n", version)
		return nil
	}
	if err != nil {
		return err
	}

	cfg := config.Load()
	if cfg.Warning != "" {
		// Before the TUI takes the terminal, so it survives on the original
		// screen after exit — otherwise a config typo is silently defaults.
		fmt.Fprintln(os.Stderr, "leanreview: "+cfg.Warning)
	}
	logFile := setupLogging(cfg.LogPath)
	if logFile != nil {
		defer logFile.Close()
	}

	// Apply configuration that must be in place before any diff is parsed.
	diff.TabWidth = cfg.TabWidth
	if !opts.contextSet {
		opts.contextN = cfg.Context
	}

	// Config housekeeping modes: generate a baseline or validate the current
	// file, then exit — neither needs a review source.
	if opts.initConfig {
		return runInitConfig()
	}
	if opts.checkConfig {
		return runCheckConfig()
	}

	ctx := context.Background()

	// --list: discover requests; on a TTY, picking one reviews it.
	if opts.list {
		url, err := runList(ctx, cfg, opts)
		if err != nil || url == "" {
			return err
		}
		opts.args = []string{url}
	}

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

	// --discard: delete the saved draft for this source and exit.
	if opts.discard {
		existing, _ := store.Load(src.Key())
		if err := store.Delete(src.Key()); err != nil {
			return fmt.Errorf("discard draft: %w", err)
		}
		n := 0
		if existing != nil {
			n = len(existing.Comments)
		}
		fmt.Printf("discarded draft for %s (%d comment(s))\n", src.Title(), n)
		return nil
	}

	// A review-exchange source seeds its draft from the document itself: the
	// file is the medium both sides of the conversation share, so it wins
	// over anything the local store remembers for this key.
	exSrc, _ := src.(*source.ExchangeSource)

	// Load or create the draft for this source.
	var draft *review.DraftReview
	if exSrc != nil {
		draft = exSrc.Exchange().ToDraft(src.Key(), files)
		if draft.Title == "" {
			draft.Title = src.Title()
		}
	} else if draft, err = store.Load(src.Key()); err != nil {
		log.Printf("load draft: %v", err)
	}
	headOID := src.HeadOID(ctx)
	if draft == nil {
		draft = review.NewDraftReview(src.Key(), src.Title(), headOID)
	} else if exSrc == nil {
		draft.SourceKey = src.Key()
		if draft.Title == "" {
			draft.Title = src.Title()
		}
		// If the head moved since the draft was saved, re-anchor its comments
		// against the current diff and persist the result.
		if headOID != "" && draft.HeadOID != "" && draft.HeadOID != headOID && len(draft.Comments) > 0 {
			s := review.RelocateDrafts(draft, files, headOID)
			if s.Changed() {
				log.Printf("relocation: %d moved, %d orphaned", s.Moved, s.Orphaned)
				fmt.Printf("head changed: %d comment(s) moved, %d orphaned\n", s.Moved, s.Orphaned)
			}
			if err := store.Save(draft); err != nil {
				log.Printf("save relocated draft: %v", err)
			}
		}
	}

	// Full-file context fetcher: resolves content by identity through the
	// on-disk cache (opened here so its age/size cleanup runs at app start);
	// nil when the source cannot provide file contents.
	var fetchContext func(context.Context, string) ([]byte, error)
	if cc, ok := src.(source.ContextContenter); ok {
		fcache, ferr := filecache.Open()
		if ferr != nil {
			log.Printf("file cache disabled: %v", ferr)
		}
		fetchContext = func(ctx context.Context, path string) ([]byte, error) {
			key := cc.ContextKey(ctx, path)
			if key != "" && fcache != nil {
				if data, ok := fcache.Get(key); ok {
					return data, nil
				}
			}
			data, err := cc.ContextContent(ctx, path)
			if err != nil {
				return nil, err
			}
			if key != "" && fcache != nil {
				if err := fcache.Put(key, data); err != nil {
					log.Printf("file cache put: %v", err)
				}
			}
			return data, nil
		}
	}

	// rawPatch is the literal diff text, needed to make exchange exports
	// self-contained; nil when the source cannot produce one.
	var rawPatch []byte
	if rp, ok := src.(source.RawPatcher); ok {
		if rawPatch, err = rp.RawPatch(ctx); err != nil {
			log.Printf("raw patch: %v", err)
			rawPatch = nil
		}
	}

	// Non-interactive export: write drafts (Markdown, or a review-exchange
	// document for .json destinations) and exit.
	if opts.exportPath != "" {
		out, err := review.RenderExport(opts.exportPath, draft, rawPatch)
		if err != nil {
			return fmt.Errorf("export: %w", err)
		}
		if err := os.WriteFile(opts.exportPath, out, 0o644); err != nil {
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
		Files:        files,
		Title:        src.Title(),
		HeadOID:      headOID,
		Draft:        draft,
		Store:        store,
		Editor:       ed,
		Theme:        ui.ThemeByName(cfg.Theme),
		Highlighter:  ui.NewHighlighter(cfg.Syntax, cfg.SyntaxStyle),
		Keys:         cfg.Keys,
		Wrap:         cfg.Wrap,
		WrapWidth:    cfg.WrapWidth,
		PR:           prCtx,
		RawPatch:     rawPatch,
		Author:       cfg.Author,
		Images:       cfg.Images,
		FetchContext: fetchContext,
	})
	if exSrc != nil {
		// Every draft save also rewrites the conversation file, so quitting
		// the TUI leaves the exchange ready for the other side to read.
		model.SetExchangeWriteback(exSrc.Path())
	}

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
			prSrc, err := source.NewPRSource(ctx, forgeFor(ref), ref)
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

// fileExists reports whether p is an existing regular file (not a directory).
// It disambiguates the single-argument case in resolveSource: a real file on
// disk is always treated as a patch, even if its name would also parse as a
// PR reference.
func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// forgeFor picks the adapter matching the reference's host: GitLab hosts get
// the glab-backed client, everything else (github.com, GHE, empty) the
// gh-backed one. The TUI itself never knows which is in use.
func forgeFor(ref forge.PullRequestRef) forge.Forge {
	if forge.KindForHost(ref.Host) == forge.KindGitLab {
		return glabcli.New()
	}
	return ghcli.New()
}

// listEngines maps discovery engine names to their listers.
var listEngines = map[string]func() forge.Lister{
	"gh":   func() forge.Lister { return ghcli.New() },
	"glab": func() forge.Lister { return glabcli.New() },
}

// resolveListQuery turns the --list positionals into an engine and a filter.
//
// Shapes: [engine] [filter...], engine:name, :name — where "name" selects a
// named filter from cfg.ListFilters (":name" keeps the default engine). A
// first positional is a selector only when it starts with ":" or its prefix
// names an engine, so raw qualifiers like "author:x" stay filter text. Extra
// args after a selector refine the named filter (joined engine-appropriately:
// spaces for search queries, & for glab's query strings). With nothing
// supplied, cfg.ListFilter is the fallback; empty falls through to the
// engine's built-in default.
func resolveListQuery(cfg config.Config, args []string) (engine, filter string, err error) {
	engine = cfg.ListEngine
	if len(args) > 0 {
		first := args[0]
		if i := strings.Index(first, ":"); i >= 0 {
			prefix, name := first[:i], first[i+1:]
			_, engineOK := listEngines[prefix]
			if prefix == "" || engineOK {
				if engineOK {
					engine = prefix
				}
				if name == "" {
					return "", "", fmt.Errorf("empty filter name in %q", first)
				}
				f, ok := cfg.ListFilters[name]
				if !ok {
					if len(cfg.ListFilters) == 0 {
						return "", "", fmt.Errorf("no named filters configured (add a %q map to the config file)", "list_filters")
					}
					names := make([]string, 0, len(cfg.ListFilters))
					for n := range cfg.ListFilters {
						names = append(names, n)
					}
					sort.Strings(names)
					return "", "", fmt.Errorf("unknown filter name %q (available: %s)", name, strings.Join(names, ", "))
				}
				filter = f
				args = args[1:]
			}
		} else if _, ok := listEngines[first]; ok {
			engine = first
			args = args[1:]
		}
	}

	if len(args) > 0 {
		extra := strings.Join(args, " ")
		if filter == "" {
			filter = extra
		} else if engine == "glab" {
			filter += "&" + extra
		} else {
			filter += " " + extra
		}
	}
	if filter == "" {
		filter = cfg.ListFilter
	}
	return engine, filter, nil
}

// runList discovers requests with the resolved engine and filter. On a TTY the
// results open an interactive picker and the chosen request's URL is returned
// for review; otherwise (piped) a plain table is printed and "" is returned.
func runList(ctx context.Context, cfg config.Config, opts options) (string, error) {
	engine, filter, err := resolveListQuery(cfg, opts.args)
	if err != nil {
		return "", err
	}

	mk, ok := listEngines[engine]
	if !ok {
		names := make([]string, 0, len(listEngines))
		for n := range listEngines {
			names = append(names, n)
		}
		sort.Strings(names)
		return "", fmt.Errorf("unknown list engine %q (available: %s)", engine, strings.Join(names, ", "))
	}

	entries, err := mk().List(ctx, filter)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		fmt.Println("No matching requests.")
		return "", nil
	}

	if !isatty.IsTerminal(os.Stdout.Fd()) {
		for _, e := range entries {
			fmt.Printf("%s\t%s\t@%s\t%s\n", e.Ref, e.Title, e.Author, e.UpdatedAt.Format("2006-01-02"))
		}
		return "", nil
	}

	idx, err := app.PickRequest(entries, ui.ThemeByName(cfg.Theme))
	if err != nil || idx < 0 {
		return "", err
	}
	return entries[idx].URL, nil
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
			o.contextSet = true
		case a == "--export":
			v, err := next(argv, &i, "--export")
			if err != nil {
				return o, err
			}
			o.exportPath = v
		case a == "--discard":
			o.discard = true
		case a == "--list":
			o.list = true
		case a == "--init-config":
			o.initConfig = true
		case a == "--check-config":
			o.checkConfig = true
		case a == "-h" || a == "--help":
			return o, errHelp
		case a == "-v" || a == "--version":
			return o, errVersion
		case len(a) > 1 && a[0] == '-' && a != "-":
			return o, fmt.Errorf("unknown flag: %s", a)
		default:
			o.args = append(o.args, a)
		}
	}
	return o, nil
}

// errHelp and errVersion are sentinels from parseArgs: they signal "print and
// exit 0" rather than a failure, so run intercepts them before generic error
// handling would turn them into a nonzero exit.
var errHelp = fmt.Errorf("help requested")
var errVersion = fmt.Errorf("version requested")

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
  --discard          delete the saved draft for this source and exit
  --list [engine|engine:name|:name] [filter]
                     discover open PRs/MRs: pick one to review (TTY) or print
                     a table (piped). Engine: gh or glab (default from config).
                     Filter: engine-specific search text, or a named filter
                     from the config's list_filters map (":name" keeps the
                     default engine; extra text refines it). With nothing
                     supplied, list_filter applies, falling back to "review
                     requested from me"
  --init-config      write a baseline config (defaults + full keymap) and exit
  --check-config     validate the config file, report problems, and exit
  -h, --help         show this help
  -v, --version      print the version and exit

In the TUI, press ? for the key reference.
`

// runInitConfig writes the baseline config — every setting at its default,
// the full default keymap spelled out, and a $schema reference for editor
// validation — refusing to touch an existing file: a generator must never be
// able to destroy the config it exists to help create.
func runInitConfig() error {
	path := config.Path()
	if path == "" {
		return fmt.Errorf("cannot resolve a config location (no home directory)")
	}
	if fileExists(path) {
		return fmt.Errorf("%s already exists — refusing to overwrite (edit it, or delete it first)", path)
	}
	out, err := config.BaselineJSON(app.DefaultKeymap())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote baseline config to %s\n", path)
	return nil
}

// runCheckConfig validates the config file and reports every problem — the
// counterpart to Load's tolerance: Load must start the TUI despite a typo,
// while this mode exists to find the typo. A missing file is not a problem
// (defaults apply); any reported problem makes the exit code non-zero so the
// check is scriptable.
func runCheckConfig() error {
	path := config.Path()
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("no config file at %s (defaults apply)\n", path)
		return nil
	}
	problems := config.Validate(data, app.KnownActions())
	if len(problems) == 0 {
		fmt.Printf("%s is valid\n", path)
		return nil
	}
	for _, p := range problems {
		fmt.Println("  - " + p)
	}
	return fmt.Errorf("%s: %d problem(s)", path, len(problems))
}

// next consumes the value following a flag in argv, advancing the caller's
// loop index past it so value-taking flags work in the hand-rolled parser.
// flag is only used to name the offender in the error.
func next(argv []string, i *int, flag string) (string, error) {
	if *i+1 >= len(argv) {
		return "", fmt.Errorf("%s requires a value", flag)
	}
	*i++
	return argv[*i], nil
}

// atoi parses a non-negative decimal integer, rejecting anything strconv.Atoi
// would tolerate that makes no sense for a context-line count ("+3", "-1",
// whitespace). Digits-only keeps the -U error message simple and uniform.
func atoi(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("not a number: %q", s)
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// setupLogging redirects the standard logger to an append-only file at path,
// because the TUI owns the terminal and any log output to stdout/stderr would
// corrupt the alternate screen. Failures are swallowed (returning nil) but
// still silence the logger: losing diagnostics is preferable to refusing to
// start — or to log.Print scribbling over the alternate screen via stderr.
func setupLogging(path string) *os.File {
	f, err := openLogFile(path)
	if err != nil {
		log.SetOutput(io.Discard)
		return nil
	}
	log.SetOutput(f)
	return f
}

// openLogFile creates the log directory and opens the file for appending.
func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}
