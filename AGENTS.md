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
