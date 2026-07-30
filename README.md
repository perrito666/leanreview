# leanreview

A terminal code-review client. It reviews a **patch/diff file** or a **local git
comparison**, lets you navigate the diff and attach draft comments anchored to
semantic diff locations, and **exports those comments as Markdown** so they can
be fed back as prompt feedback. Pull-request (GitHub) mode is planned; see
[Milestones](#milestones).

It is a review client, not a git client: the installed `git` handles repository
semantics, and this tool focuses on navigation, rendering, review state, and
comments.

## Install

```bash
go build -o leanreview ./cmd/leanreview
```

## Usage

```bash
leanreview change.diff          # review a patch file
git diff | leanreview -         # review a patch from stdin
leanreview .                    # working tree vs HEAD
leanreview --base main          # main...HEAD (merge-base)
leanreview --staged             # index vs HEAD
leanreview HEAD~3 HEAD          # explicit revision range
```

The optional `review` verb is accepted too: `leanreview review .`.

Non-interactive export (useful in scripts / CI):

```bash
leanreview --export out.md change.diff
```

## Keys

| Key | Action |
| --- | --- |
| `j` / `k` | down / up a line |
| `J` / `K` | next / previous change |
| `]c` / `[c` | next / previous hunk |
| `]f` / `[f` | next / previous file |
| `gg` / `G` | first / last line |
| `Ctrl-d` / `Ctrl-u` | half page |
| `t` | toggle unified / split |
| `za` / `zR` / `zM` | fold current hunk / expand all / collapse all |
| `/`, `n`, `N` | search diff text, next/previous match |
| `f` | file picker |
| `C` / `Enter` | comment list |
| `v` / `V` | select lines / changed block |
| `c` | comment on line or selection (opens `$EDITOR`) |
| `e` | edit draft comment under cursor |
| `dd` | delete comment under cursor |
| `r` | reply to thread under cursor (PR mode) |
| `s` | submit review (PR mode) |
| `:w` | save drafts |
| `:export FILE` | export comments as Markdown |
| `:q` / `q` | quit |
| `?` | help |

Comments open your editor (`LEANREVIEW_EDITOR` → `GIT_EDITOR` → `VISUAL` →
`EDITOR` → `git var GIT_EDITOR` → `vi`). A selection must map onto one continuous
range on one side (GitHub semantics); cross-side selections are rejected.

Source is syntax-highlighted in the unified view (Chroma). Drafts persist under
`$XDG_STATE_HOME/leanreview/drafts/` and reload on the next run of the same
source. Logs go to `$XDG_STATE_HOME/leanreview/leanreview.log` (never stdout,
which the TUI owns).

Environment:

- `LEANREVIEW_EDITOR` — editor override (highest precedence).
- `LEANREVIEW_SYNTAX=0` — disable syntax highlighting.
- `NO_COLOR` — disable all color (highlighting and styling).
- `LEANREVIEW_LOG` — log file path.

## Export format

Comments export as Markdown grouped by file:

````markdown
## internal/api/handler.go

### L72 (RIGHT)
```go
result, err := calculate(input)
```
> This ignores the error from calculate().
````

## GitHub pull-request mode

Point leanreview at a pull request and it fetches the canonical diff through
`gh`:

```bash
leanreview 418                                   # infers owner/repo from origin
leanreview owner/repo#418
leanreview https://github.com/owner/repo/pull/418
```

Existing review threads are marked on the diff (`◆N`) and listed in the comment
overlay. Press `r` on a line to reply, and `s` (or `:comment` / `:approve` /
`:request`) to open the submission screen — your draft line comments are sent as
one atomic review and staged replies are posted. Nothing is sent until you
confirm. Requires the [`gh` CLI](https://cli.github.com/) to be installed and
authenticated (`gh auth login`); enterprise hosts are supported.

When the PR head moves between sessions, saved draft comments are re-anchored to
the new diff: each is relocated by matching its captured surrounding context
(following renames), uniquely or not at all. Comments with no unique match are
marked orphaned, excluded from submission, and kept as drafts for you to
reposition.

## Milestones

- **M1 (done)** — shared diff core, patch/local viewer, unified/split layouts,
  draft comments via `$EDITOR`, persistence, Markdown export.
- **M3 (done)** — GitHub PR mode (via `gh`): canonical PR diff, threads, replies,
  atomic review submission, and context-based comment relocation on head change.
- **Ergonomics (done)** — edit drafts (`e`), in-diff search (`/`, `n`, `N`),
  collapsible hunks (`za`/`zR`/`zM`), and Chroma syntax highlighting.
- **Remaining polish** — persistent side panels / file sidebar, configurable
  keys & themes.
- **M5** — other forges (GitLab/Forgejo) behind the same `Forge` seam.

## Architecture

```
cmd/leanreview      CLI entrypoint, logging, tea.Program wiring
internal/diff       canonical diff model, parser, unified/split projections, Location
internal/source     resolve args -> ReviewSource (patch file, stdin, local git, PR)
internal/git        wrapper over the git executable (incl. origin inference)
internal/forge      host-agnostic Forge seam + PR-ref parsing
internal/forge/ghcli  gh-backed Forge implementation (PR, diff, threads, reviews)
internal/review     draft comments, XDG persistence, Markdown export
internal/editor     $EDITOR resolution/launch, temp file, template stripping
internal/app        Bubble Tea model/update/view, key grammar, selection
internal/ui         theme, help text
internal/config     environment-sourced config
```

The core design rule: rendered screen rows are never canonical comment
locations. Comments anchor to a semantic `diff.Location` (path, side, line
range, context) that survives resize, folding, and unified↔split toggles.
```
