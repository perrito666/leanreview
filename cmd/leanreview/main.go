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
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"iter"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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
		_, _ = fmt.Fprintln(os.Stderr, "leanreview:", err)
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
	if errors.Is(err, errHelp) {
		fmt.Print(usage)
		return nil
	}
	if errors.Is(err, errVersion) {
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
		_, _ = fmt.Fprintln(os.Stderr, "leanreview: "+cfg.Warning)
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
	var fetchContext func(context.Context, string, diff.Side) ([]byte, error)
	if cc, ok := src.(source.ContextContenter); ok {
		fcache, ferr := filecache.Open()
		if ferr != nil {
			log.Printf("file cache disabled: %v", ferr)
		}
		fetchContext = func(ctx context.Context, path string, side diff.Side) ([]byte, error) {
			key := cc.ContextKey(ctx, path, side)
			if key != "" && fcache != nil {
				if data, ok := fcache.Get(key); ok {
					return data, nil
				}
			}
			data, err := cc.ContextContent(ctx, path, side)
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

	// Attachment fetcher: comment images (GitHub user-attachments, GitLab
	// uploads) resolved through the forge's authentication, cached on disk
	// only when the URL is content-addressed (see attachmentCacheKey).
	var fetchImage func(context.Context, string) ([]byte, error)
	if prSrc != nil {
		fcache, ferr := filecache.Open()
		if ferr != nil {
			log.Printf("file cache disabled: %v", ferr)
		}
		fetchImage = func(ctx context.Context, url string) ([]byte, error) {
			// Mutable URLs (branch-addressed raw files) get no cache entry at
			// all — see attachmentCacheKey for why.
			key := ""
			if ck := attachmentCacheKey(url); ck != "" {
				key = "attachment-" + ck
			}
			if key != "" && fcache != nil {
				if data, ok := fcache.Get(key); ok {
					return data, nil
				}
			}
			data, err := prSrc.Attachment(ctx, url)
			if err != nil {
				log.Printf("attachment %s: %v", url, err)
				return nil, err
			}
			if !looksLikeImage(data) {
				// An HTML viewer page or error body must never be cached and
				// served as an "image" forever after.
				log.Printf("attachment %s: response is not an image", url)
				return nil, fmt.Errorf("attachment is not an image")
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
		general, gerr := prSrc.GeneralComments(ctx)
		if gerr != nil {
			log.Printf("fetch general comments: %v", gerr)
		}
		prCtx = &app.PRContext{
			Forge:   prSrc.Forge(),
			Ref:     prSrc.Ref(),
			PR:      prSrc.PullRequest(),
			Threads: threads,
			General: general,
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
		Sequences:    appSequences(cfg.Sequences),
		Wrap:         cfg.Wrap,
		WrapWidth:    cfg.WrapWidth,
		PR:           prCtx,
		RawPatch:     rawPatch,
		Author:       cfg.Author,
		Images:       cfg.Images,
		ChangeColors: cfg.ChangeColors,
		ChangeTint:   cfg.ChangeTint,
		FetchContext: fetchContext,
		FetchImage:   fetchImage,
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
	if len(opts.args) == 1 {
		// A real file on disk is always a patch, even if its name would also
		// parse as a PR reference; directories do not count as files.
		info, statErr := os.Stat(opts.args[0])
		isFile := statErr == nil && !info.IsDir()
		if ref, ok := forge.ParseRef(opts.args[0]); ok && !isFile {
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
					names := slices.Sorted(maps.Keys(cfg.ListFilters))
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
		names := slices.Sorted(maps.Keys(listEngines))
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

// missingVforF formats generic error for missing value for flag where expected
func missingVforF(flag cmdFlag) error {
	return fmt.Errorf("a value is required when %q flag is provided", flagsToNames[flag])
}

// parseArgs performs a small hand-rolled parse so flags and positionals can be
// interleaved (the stdlib flag package stops at the first positional).
func parseArgs(argv []string) (options, error) {
	var o options
	o.contextN = 3
	for arg, err := range iterArgs(argv) {
		if err != nil {
			return o, err
		}
		switch arg.Name {
		case flagBase:
			if !arg.HasValue {
				return o, missingVforF(flagBase)
			}
			o.base = arg.Value
		case flagStaged:
			o.staged = true
		case flagContext:
			if !arg.HasValue {
				return o, missingVforF(flagContext)
			}
			n, err := strconv.ParseUint(arg.Value, 10, 64)
			if err != nil {
				return o, fmt.Errorf("%s: %w", arg.Value, err)
			}
			o.contextN = int(n)
			o.contextSet = true
		case flagExport:
			if !arg.HasValue {
				return o, missingVforF(flagExport)
			}
			o.exportPath = arg.Value
		case flagDiscard:
			o.discard = true
		case flagList:
			o.list = true
		case flagInitConfig:
			o.initConfig = true
		case flagCheckConfig:
			o.checkConfig = true
		case flagHelp:
			return o, errHelp
		case flagVersion:
			return o, errVersion
		default:
			o.args = append(o.args, arg.Value)
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

type cmdFlag int

const (
	flagNoFlag cmdFlag = iota
	flagUnknown
	flagBase
	flagStaged
	flagContext
	flagExport
	flagDiscard
	flagList
	flagHelp
	flagVersion
	flagInitConfig
	flagCheckConfig
)

var namesToFlags = map[string]cmdFlag{
	"base":         flagBase,
	"staged":       flagStaged,
	"context":      flagContext,
	"U":            flagContext,
	"export":       flagExport,
	"discard":      flagDiscard,
	"list":         flagList,
	"help":         flagHelp,
	"h":            flagHelp,
	"version":      flagVersion,
	"v":            flagVersion,
	"init-config":  flagInitConfig,
	"check-config": flagCheckConfig,
}

var flagsToNames = map[cmdFlag]string{
	flagBase:        "--base",
	flagStaged:      "--staged",
	flagContext:     "--context/-U",
	flagExport:      "--export",
	flagDiscard:     "--discard",
	flagList:        "--list",
	flagHelp:        "--help/-h",
	flagVersion:     "--version/-v",
	flagInitConfig:  "--init-config",
	flagCheckConfig: "--check-config",
}

// flagsWithValue marks the flags that consume the following argument; every
// other flag is boolean, so what follows it is a positional (--list's
// engine/filter and any review target arrive that way).
var flagsWithValue = map[cmdFlag]bool{
	flagBase:    true,
	flagContext: true,
	flagExport:  true,
}

// runInitConfig writes the baseline config — every setting at its default,
// the full default keymap spelled out, and a $schema reference for editor
// validation — refusing to touch an existing file: a generator must never be
// able to destroy the config it exists to help create.
func runInitConfig() error {
	path := config.Path()
	if path == "" {
		return fmt.Errorf("cannot resolve a config location (no home directory)")
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return fmt.Errorf("%s already exists — refusing to overwrite (edit it, or delete it first)", path)
	}
	var seqs []config.SequenceBinding
	for _, sb := range app.DefaultSequenceBindings() {
		seqs = append(seqs, config.SequenceBinding{Keys: []string{sb.First, sb.Second}, Action: sb.Action})
	}
	out, err := config.BaselineJSON(app.DefaultKeymap(), seqs)
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
	// Binding-overlap problems live above the config package: only the app
	// knows the grammar's dispatch order (counts, then prefixes, then keys).
	cfg := config.Load()
	problems = append(problems, app.ValidateBindings(cfg.Keys, appSequences(cfg.Sequences))...)
	if len(problems) == 0 {
		fmt.Printf("%s is valid\n", path)
		return nil
	}
	for _, p := range problems {
		fmt.Println("  - " + p)
	}
	return fmt.Errorf("%s: %d problem(s)", path, len(problems))
}

type argument struct {
	Name     cmdFlag
	Value    string
	HasValue bool
}

// strFlagToCmdFlag returns the actual flag stripping dashes
func strFlagToCmdFlag(name string) cmdFlag {
	cleanFlag := strings.TrimLeft(name, "-")
	if len(cleanFlag) == 0 {
		return flagNoFlag
	}
	flag, ok := namesToFlags[cleanFlag]
	if !ok {
		return flagUnknown
	}
	return flag
}

// iterArgs iterates argv and returns flags and their values if any or fails
// if an unknown flag was passed
func iterArgs(
	argv []string,
) iter.Seq2[argument, error] {
	return func(yield func(argument, error) bool) {
		skip := false
		positionalOnly := false
		for iA, a := range argv {
			if skip {
				skip = false
				continue
			}
			var flag cmdFlag
			var value string
			if a == "--" && !positionalOnly {
				// The conventional end-of-flags separator: consumed, and
				// everything after it is a positional even when it starts
				// with a dash (targets can be git revisions like "-foo").
				positionalOnly = true
				continue
			}
			isFlag := !positionalOnly && strings.HasPrefix(a, "-")
			if isFlag {
				flag = strFlagToCmdFlag(a)
				if flag == flagNoFlag {
					// A bare "-" is not a flag but the stdin target.
					isFlag = false
				}
				if flag == flagUnknown {
					if !yield(argument{
						Name:     flag,
						Value:    value,
						HasValue: false,
					}, fmt.Errorf("unknown flag %q", a)) {
						return
					}
					continue
				}
			}
			hasValue := isFlag && flagsWithValue[flag] && len(argv) > iA+1 && !strings.HasPrefix(argv[iA+1], "-")
			if hasValue {
				value = argv[iA+1]
				skip = true
			}
			if !isFlag {
				value = a
				hasValue = true
			}

			if !yield(argument{
				Name:     flag,
				Value:    value,
				HasValue: hasValue,
			}, nil) {
				return
			}
			continue
		}
		return
	}
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

// appSequences converts config sequence entries to the app's binding form,
// dropping structurally invalid entries (--check-config names them).
func appSequences(in []config.SequenceBinding) []app.SeqBinding {
	var out []app.SeqBinding
	for _, sb := range in {
		if len(sb.Keys) != 2 || sb.Keys[0] == "" || sb.Keys[1] == "" {
			continue
		}
		out = append(out, app.SeqBinding{First: sb.Keys[0], Second: sb.Keys[1], Action: sb.Action})
	}
	return out
}

// commitPinnedRe matches a 40-hex path segment — a URL addressing content at
// a specific commit, which can never change.
var commitPinnedRe = regexp.MustCompile(`/[0-9a-f]{40}/`)

// attachmentCacheKey derives a disk-cache key for an attachment URL, or ""
// when the URL must not be cached at all. Only content-addressed URLs are
// safe: forge asset stores (signed GitHub user-images, upload secrets) and
// commit-pinned paths. A branch-addressed raw URL keeps its path while the
// branch moves under it, so caching it by URL serves stale bytes from a
// previous session — the image the user reviewed last week, not this one.
// For the cacheable signed URLs the query is stripped: the token rotates on
// every rendering while the path identifies the asset, so caching by full
// URL would never hit.
func attachmentCacheKey(url string) string {
	base := url
	if i := strings.IndexByte(base, '?'); i >= 0 {
		base = base[:i]
	}
	switch {
	case strings.Contains(base, "private-user-images.githubusercontent.com/"),
		strings.Contains(base, "/user-attachments/assets/"),
		strings.Contains(base, "objects.githubusercontent.com/"),
		strings.Contains(base, "/uploads/"),
		commitPinnedRe.MatchString(base):
		return base
	}
	return ""
}

// looksLikeImage sniffs whether bytes decode as a supported raster image —
// the guard that keeps HTML error pages out of the attachment cache.
func looksLikeImage(data []byte) bool {
	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	return err == nil
}
