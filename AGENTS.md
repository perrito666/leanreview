# AGENTS.md

Context and decisions for coding agents (and new contributors) working on
leanreview — a Bubble Tea terminal code-review client for patch files, local
git comparisons, GitHub PRs, and GitLab MRs.

## Commands

```bash
make            # build ./leanreview
go test ./...   # full suite (fast, no network; forges are faked)
make race       # suite with the race detector (what CI runs)
make vet fmt    # go vet / gofmt -w
make screens    # regenerate docs/screens/*.svg from the real UI (needs freeze)
```

CI (`ci.yml`) enforces gofmt, vet, build, and `go test -race`. **Release is
automatic**: `release.yml` cuts a tagged release on every merge to `main`
(`#minor`/`#major` in the merge commit bump those; `[skip release]` skips).
Treat merging as shipping.

## Package map

```
cmd/leanreview      CLI entrypoint, hand-rolled flag parsing, logging, tea.Program wiring
internal/diff       canonical diff model, parser, unified/split projections, Location
internal/source     resolve args -> ReviewSource (patch file, stdin, local git, PR/MR)
internal/git        wrapper over the git executable (incl. origin inference)
internal/forge      host-agnostic Forge seam + PR/MR-ref parsing + host dispatch
internal/forge/ghcli    gh-backed adapter (GitHub)   — shells out, no API SDK
internal/forge/glabcli  glab-backed adapter (GitLab) — shells out, no API SDK
internal/review     draft comments, XDG persistence, context relocation, MD export
internal/editor     $EDITOR resolution/launch, temp file, template stripping
internal/app        Bubble Tea model/update/view, key grammar, selection, overlays
internal/ui         theme, syntax highlighting, markdown styling, help text
internal/config     defaults <- config file <- environment
```

## Load-bearing design decisions

- **Semantic anchoring.** Rendered screen rows are never canonical comment
  locations. Comments anchor to `diff.Location` (path, side, line range,
  context anchor); rows are projections. Any feature that maps "cursor row →
  comment" must go through `Location`, never a row index.
- **One row = one screen line.** `internal/app` scrolling assumes it.
  Display-only rows (wrapped continuations, annotation boxes) are injected as
  rows with `Source == nil` and are skipped by navigation; annotation-box
  borders are rows with `diff.AnnotationEdge` set. If you add a new
  display-only row type, navigation, `contentHeight`, and the width
  invariants in tests must all still hold.
- **Height budget.** `contentHeight() = height - 3`: two title lines + one
  status line. Change the chrome and you must change it here and in the tests
  that skip title/status lines.
- **Forge seam.** The UI depends only on `forge.Forge`; adapters shell out to
  `gh`/`glab` so authentication and enterprise hosts are their problem.
  Adding a forge = new adapter package + `Kind` entry + wiring in
  `cmd/leanreview` (`forgeFor`, `listEngines`). Forgejo/Gitea is an open slot
  behind the same seam.
- **Canonical diffs from the host.** PR mode reviews the diff the host
  serves (never a locally recomputed one) so comment positions match the web
  UI. glabcli reconstructs unified patches from GitLab's changes endpoint.
- **Drafts are local until submitted.** Persistence is per-source-key JSON
  under XDG state, atomic writes. Submission clears exactly what the host
  accepted — on partial failure the accepted part must be removed from the
  draft (retry-safety; see `submitResultMsg.done`).
- **Relocation is conservative.** On head change, drafts re-anchor only on a
  unique context match; ambiguous or missing → orphaned, excluded from
  submission, never silently dropped.
- **Hand-rolled markdown.** `internal/ui/markdown.go` deliberately renders a
  small subset; do not add glamour or another renderer dependency for it.
- **Review exchange format.** `internal/review/exchange.go` implements the
  versioned `*.review.json` conversation format (spec:
  `docs/reference/exchange-format.md`, schema: `docs/schema/`). The patch is
  an array of lines on the wire (readability/diffability); readers accept a
  string too. Dismissed comments are kept, never submitted, never dropped.
  Format changes are spec changes: update the spec, the schema, the editor
  grammars in `editors/`, and the skill in `skills/leanreview-loop/`
  together, and bump the version only for incompatible changes.
- **No TUI writes to stdout/stderr while running.** Logs go to the XDG log
  file; if it cannot open, the logger is discarded. Startup warnings print
  before the TUI takes the terminal.
- **Context view.** T overlays hunks on the full file
  (`diff.RenderUnifiedContext`): gap rows are deliberately uncommentable
  (hosts anchor to diff positions), content is fetched lazily through
  `Config.FetchContext` (never on file switch), and hunk identity in
  context comes from row Sources — there are no header rows to count.
  Content that fails verification against the hunks is rejected, never
  rendered. The file cache (`internal/filecache`) keys by content identity
  (blob id / head+path); there is no in-place invalidation by design.
- **Thread box.** All comments/threads on a line share one box, oldest
  first (`at` RFC 3339 sorts lexically). Image references render as
  preformatted rows (`DisplayRow.Pre`) — kitty Unicode placeholders, chafa
  fallback, tag otherwise. Comment bodies are scanned for both Markdown and
  HTML <img> image syntax (GitHub pastes the latter); forge attachments
  fetch through the adapter's auth (Forge.Attachment) once per URL into
  session files. The bodies scanned include the PR description and general
  comments (the `p` overlay), not just inline threads.
- **Kitty payload delivery.** The one-shot kitty payload transmission never
  rides on rendered rows: rows are also built during update processing
  (cursor clamping, layout math) where the strings are discarded, and a
  payload emitted there dies unseen — images then only appear after a width
  change. Renders enqueue; `ImageRenderer.TakeTransmissions` drains at the
  top of `Model.View`, the only output guaranteed to reach the terminal.
- **Attachment disk cache is content-addressed only.** `attachmentCacheKey`
  returns "" (no caching) unless the URL is content-addressed: signed
  GitHub user-images, user-attachments assets, GitLab `/uploads/` secrets,
  or a 40-hex commit-pinned path. A branch-addressed raw URL keeps its path
  while the branch moves, so caching it by URL serves last session's image.
  Signed URLs cache by path only — their query token rotates per render.
- **Two-pass syntax highlighting.** Whole-file passes per side
  (`Highlighter.ContentLines`): deletions index the old-file pass,
  everything else the new — that is what makes multi-line constructs color
  correctly. Fallback: per-hunk side stitching, then legacy per-line. The
  passes emit fg-only SGR (never a full reset) so the change_tint
  background survives; keep that discipline. S cycles
  red/green-changes → syntax-everywhere → off; content arrives through the
  same fetch/cache seam as the context view.
- **Bindings are data.** Single keys (Keymap), two-key sequences
  (Sequences — object form in config, so non-US layouts can express them),
  and named presets all resolve in app.resolveKeymap: preset/user base,
  then top-level overrides. Structural validation lives in config;
  dispatch-order overlap checks (dead bindings) live in app.ValidateBindings
  because only the app knows the grammar order (counts → prefixes → keys).
  Presets are supersets of the defaults, never clones — terminals cannot
  report most editor chords.
- **General comments are flat.** Both forges model the PR conversation as a
  flat list, so "reply" means a new comment carrying a Markdown quote
  (GeneralDraft.QuoteOf is context, not threading). General drafts post
  individually on submit with record-as-you-go bookkeeping, like replies.
  The review summary (draft.Summary) is the submission's own general
  comment, edited from the confirmation screen.
- **Themes resolve by metadata name.** Theme files in themes/ next to the
  config are referenced by their "name" field, never the filename; built-in
  names (default, default-light, default-dark, mono) are reserved; files
  overlay only the roles they name on the default palette. Broken themes
  degrade to default with a stderr note — never block a review.
- **Config fill is never destructive.** --fill-config adds missing
  defaults; it must not touch existing values or drop unknown keys, even
  ones the validator flags.
- **Attachment upload is an optional capability.**
  forge.AttachmentUploader exists because GitHub's REST API has no upload
  surface while GitLab's does; submission stops before any network call
  when local image refs exist and the forge cannot upload. Uploads happen
  before the review posts, so a failure aborts with every draft intact.
- **ANSI-aware geometry.** Any string measuring/clipping must use
  `lipgloss.Width` / the `clip`/`pad` helpers, never `len()`. Input handling
  counts runes, never bytes.

## Conventions

- **Docs explain the why.** Every function carries a doc comment stating the
  rationale/constraint, not a restatement of the signature. Match the
  existing voice.
- **Tests are headless.** Models are driven directly (`m.width/height` set by
  hand, `key(m, "x")` helpers); forges are faked in-package
  (`recordingForge`). Color assertions are unreliable headless — test
  structure, not ANSI. `t.Setenv("LEANREVIEW_SYNTAX", "0")` keeps renders
  deterministic.
- **Keymap.** Single normal-mode keys live in `DefaultKeymap` and are
  user-remappable; two-key sequences (`gg`, `]c`, `dd`, `z*`) live in the
  grammar (`commands.go`) and are fixed. New actions need: keymap entry,
  `execute` case, help text (`internal/ui/help.go`), README key table, and
  `docs/reference/keys.md` (all three languages).
- **Docs site.** `docs/` is a MkDocs Material site (config in `mkdocs.yml`)
  deployed to GitHub Pages by `docs.yml` on push to `main`. i18n uses
  mkdocs-static-i18n suffix files: `page.md` (English, default),
  `page.es.md`, `page.fr.md`. **Any docs change must update all three
  languages**; untranslated pages fall back to English at build time, so a
  missing translation is silent — check.
- **Screenshots regenerate from code.** `docs/screens/*.svg` come from
  `TestGenerateScreens` (internal/app/screens_test.go) via `make screens`
  (charmbracelet `freeze`). Never hand-edit the SVGs; extend the test when a
  new view needs a screenshot.
- **Commits.** Imperative subject with an area prefix (`app:`, `forge:`,
  `cli/config:`, `docs:`, `chore:`); body explains the why. Keep commits
  scoped — this repo's history is deliberately reviewable.

## Gotchas

- `rows()` output is cached per (file, layout); mutate anything that feeds it
  and call `invalidateRows()`.
- The split-panel width math is shared via `splitHalf` — `plainSplit` and
  `renderSplitStyled` must never diverge or cursor rows misalign.
- `forge.PullRequestRef.String()` prints `#` for GitHub and `!` for GitLab;
  `Kind.String()` (`gh`/`glab`) doubles as the discovery engine name and the
  title-bar badge.
- The `test` status check name in the `protect-main` ruleset must keep
  matching the CI job name if you rename CI jobs.
