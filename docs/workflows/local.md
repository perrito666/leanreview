# Reviewing without a forge

This is leanreview's foundation workflow: no host, no account, no network —
just a diff and your notes. Everything the forge modes add (threads,
submission) layers on top of it, so everything here applies to those modes
too.

## Opening a diff

Any of these puts you in the same review TUI:

```bash
leanreview change.diff          # a patch file
git diff | leanreview -         # stdin
leanreview .                    # working tree vs HEAD (also: no argument)
leanreview --staged             # index vs HEAD
leanreview --base main          # main...HEAD (merge-base comparison)
leanreview HEAD~3 HEAD          # explicit revision range
```

`-U`/`--context` controls the number of unified context lines (default 3,
configurable).

## Navigating

`j`/`k` move by line, `J`/`K` jump between changes, `]c`/`[c` between hunks,
and `Tab`/`Shift-Tab` (or `]f`/`[f`) between files. `t` toggles unified ↔
split layout, `\` toggles the changed-files sidebar, `za` folds a hunk,
`/` searches the diff text.

The title bar always shows the review title on its first line and the current
file, position, and layout on the second.

![Split layout](../screens/split.svg)

*Split layout: deletions styled on the left panel, additions on the right,
and a draft comment boxed over the right panel.*

## Commenting

Press `c` on a line — or `v` to select a range (`V` grabs the whole changed
block), then `c`. Your `$EDITOR` opens with a Markdown template; write the
note, save, quit. The comment becomes a **draft**: the line gets a `●` in the
left gutter and the note is previewed inline in a bordered box right under it
(`i` hides/shows the previews). `e` edits it, `dd` deletes it.

In split layout, `h`/`l` choose which side of a paired change you are
commenting on — the status bar shows `[LEFT]`/`[RIGHT]`.

A selection must map onto one continuous range on one side; cross-side
selections are rejected before the editor opens.

## Reviewing your notes

`C` lists every draft; `Enter` jumps to one, `e` edits, `d` deletes.

![Comment list](../screens/comments.svg)

## Exporting

`:export notes.md` — or non-interactively `leanreview --export notes.md
change.diff` — writes Markdown grouped by file, with the commented snippet
quoted above each note:

````markdown
## internal/api/handler.go

### L72 (RIGHT)
```go
result, err := calculate(input)
```
> This still ignores the error from calculate().
````

This output is designed to be pasted straight back into an AI prompt as
review feedback, or into a plain-text review email.

## Draft persistence

Drafts save automatically (`:w` forces a save; quitting saves too) and reload
the next time you open the **same source** — each source has a stable
identity key (see [Concepts](../concepts.md)). `leanreview --discard <target>`
deletes the saved draft for a source.
