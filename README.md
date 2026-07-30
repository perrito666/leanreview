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
| `f` | file picker |
| `C` / `Enter` | comment list |
| `v` / `V` | select lines / changed block |
| `c` | comment on line or selection (opens `$EDITOR`) |
| `dd` | delete comment under cursor |
| `:w` | save drafts |
| `:export FILE` | export comments as Markdown |
| `:q` / `q` | quit |
| `?` | help |

Comments open your editor (`LEANREVIEW_EDITOR` → `GIT_EDITOR` → `VISUAL` →
`EDITOR` → `git var GIT_EDITOR` → `vi`). A selection must map onto one continuous
range on one side (GitHub semantics); cross-side selections are rejected.

Drafts persist under `$XDG_STATE_HOME/leanreview/drafts/` and reload on the next
run of the same source. Logs go to `$XDG_STATE_HOME/leanreview/leanreview.log`
(never stdout, which the TUI owns). `NO_COLOR` is honored.

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

## Milestones

- **M1 (done)** — shared diff core, patch/local viewer, unified/split layouts,
  draft comments via `$EDITOR`, persistence, Markdown export.
- **M2** — in-diff comment markers polish, side panels, file sidebar.
- **M3** — GitHub PR mode (via `gh`): threads, replies, atomic review submission,
  head-commit change detection and comment relocation.
- **M4** — search, collapsible context, syntax highlighting, config & themes.
- **M5** — other forges (GitLab/Forgejo) behind the same `Forge` seam.

## Architecture

```
cmd/leanreview      CLI entrypoint, logging, tea.Program wiring
internal/diff       canonical diff model, parser, unified/split projections, Location
internal/source     resolve args -> ReviewSource (patch file, stdin, local git)
internal/git        wrapper over the git executable
internal/forge      host-agnostic Forge seam + PR-ref parsing (impls land in M3)
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
