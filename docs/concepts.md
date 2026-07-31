# Concepts

## Semantic anchoring

The core design rule: **rendered screen rows are never canonical comment
locations**. A comment anchors to a semantic location — path, side
(old/new), line range, and the surrounding context lines — not to a terminal
coordinate. That anchor survives resizing, hunk folding, line wrapping, and
unified ↔ split toggles, because those only change the *projection* of the
diff, not the diff itself.

## Drafts

Every comment starts as a **draft**, saved locally and never sent anywhere
until you explicitly submit. Drafts live in
`$XDG_STATE_HOME/leanreview/drafts/` (`~/.local/state/…`), one JSON file per
source, written atomically.

Each review source has a **stable identity key** — a hash of the patch
content for files/stdin, the diff spec for git comparisons, host/repo/number
for PRs and MRs — so reopening the same source reloads its drafts, and two
different sources never share state. `--discard` deletes the saved draft for
a source.

## Relocation and orphans

In pull-request mode the diff can change under your saved drafts: someone
pushes new commits between your sessions. On load, leanreview re-anchors each
draft by matching its captured surrounding context against the new diff
(following file renames), and it relocates a comment **only on a unique
match** — a context that matches twice is ambiguous, and guessing would put
your note on the wrong line.

Comments that cannot be placed are marked **orphaned**: they stay in your
draft list, render with an `[orphaned]` tag, are excluded from submission
(the confirmation screen warns about them), and wait for you to reposition or
delete them. Nothing is silently dropped.

## Submission

Submission is explicit and atomic where the host allows it:

- **GitHub** — all line comments go up as one review with your chosen event
  (comment / approve / request changes); staged thread replies post
  afterwards.
- **GitLab** — has no atomic review endpoint, so comments become positioned
  diff discussions posted in order, and the event maps onto approval or a
  note.

Either way, whatever the host has accepted is immediately cleared from your
drafts — a retry after a partial failure sends only what is still pending,
never a duplicate.

## The editor

`c`, `e`, and `r` open your editor, resolved in this order: `editor` in the
config file / `LEANREVIEW_EDITOR` → `GIT_EDITOR` → `VISUAL` → `EDITOR` →
`git var GIT_EDITOR` → `vi`. Values are parsed as command lines (`code
--wait`, `nvim -f`). The buffer is a Markdown file with an HTML-comment
header carrying context; the header is stripped automatically, and saving an
empty body discards the comment.
