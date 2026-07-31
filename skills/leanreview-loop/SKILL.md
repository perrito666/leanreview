---
name: leanreview-loop
description: Run an iterative code-review conversation with a human through leanreview's review-exchange files. Use this whenever the user asks you to review changes before a PR, to "write up a review" of a diff or branch, to iterate on review feedback, or to act on an edited *.review.json file — and whenever you have just finished a sizeable change and the user wants a human-vetted review pass before submitting. If a *.review.json file appears in the conversation or the working tree, this skill is how to produce and consume it.
---

# The leanreview review loop

You are one side of a review conversation. The other side is a human using
the leanreview TUI. The medium is a single self-contained JSON file (the
"review exchange"): you write your review into it, the human edits it in the
TUI — dismissing what they disagree with, rewording what they half-agree
with, adding notes of their own — and you read the result back and act on
it. Nothing goes over the network; the file is the whole conversation.

Why this shape: the human gets a purpose-built diff UI instead of raw JSON,
you get a machine-readable verdict instead of prose, and every comment keeps
a stable id so you can tell exactly what the human changed.

## Prerequisites

`leanreview` must be installed (`go install
github.com/perrito666/leanreview/cmd/leanreview@latest`). Verify with
`leanreview --version`.

## Step 1 — produce the diff and the review

Generate the unified diff of whatever is under review. For pre-PR review of
a branch, diff against the merge base so only the branch's own changes
appear:

```bash
git diff --merge-base main > /tmp/changes.diff   # or: git diff HEAD, git diff --staged
```

Read the diff and the relevant source files, form your review, then write
`<topic>.review.json` — the `*.review.json` suffix matters: editors and the
TUI key off it. The format essentials:

- `"leanreview_review": 1` first, then `title`, `summary`, `patch`,
  `comments`.
- `patch` is the diff **as an array of lines** (split on newline, no
  trailing empty element).
- Each comment: `id` (assign short stable ids: `c1`, `c2`, …), `author`
  (use `"assistant"` unless told otherwise), `path`, `side`, `start_line`,
  `body` (Markdown), `snippet`.

Read [references/format.md](references/format.md) before writing your first
exchange file — it has the full field semantics and a worked example.

**Line numbers are where reviews go wrong.** `start_line` counts lines in
the chosen image of the patch: `RIGHT` = the file as it looks after the
change (added and context lines), `LEFT` = before it (deleted and context
lines). Derive the number by walking the hunk header math in the patch you
just wrote — never from your memory of the file. Always fill `snippet` with
the exact line text: it is how mistakes get caught.

## Step 2 — validate before handing off

leanreview itself is the validator. Export the file non-interactively; any
comment whose anchor does not resolve comes back marked orphaned:

```bash
leanreview mytopic.review.json --export /tmp/check.md
grep -c "orphaned" /tmp/check.md   # 0 means every comment anchored
```

If anything is orphaned, fix the line numbers (compare `snippet` against the
patch) and re-validate. Handing the human a review with dangling comments
wastes their goodwill on your bookkeeping.

## Step 3 — hand off to the human

Tell the user the file is ready and how to review it:

```bash
leanreview mytopic.review.json
```

In the TUI they will see your comments boxed inline under the diff lines,
attributed to you. Mention the verbs that matter: `x` dismisses a comment
(rejects it), `e` edits its body, `c` adds their own comment, `dd` deletes
outright, `q` quits — **the file is rewritten automatically on every
change**, so when they are done, it is done. Do not poll or wait; resume
when the user says so.

## Step 4 — read the verdict and act

Re-read the file and diff it against what you wrote (ids are stable):

- `"state": "dismissed"` — the human rejected it. Do **not** act on it, do
  not re-raise it in later rounds, and keep it in the file: the recorded
  verdict is what prevents you from suggesting the same thing twice.
- still `active`, body unchanged — confirmed; act on it.
- still `active`, body edited or a reply added — the human refined or
  answered it; the edited text is the instruction, not your original.
- new comments (ids you did not assign, or a different `author`) — direct
  instructions from the human; treat them with the highest priority.
- `summary` may also have been edited — re-read it.

Then implement the accepted feedback.

## Step 5 — close the loop

After changing code, the old patch no longer matches reality. For another
round, regenerate the diff and write a **fresh** exchange file (new patch,
new anchors). Carry forward only what is still worth discussing: unresolved
comments keep their ids and gain a reply from you explaining what you did or
why you disagree; addressed comments are simply left out — the human asked,
you fixed, done. Dismissed comments from earlier rounds stay out too.

One round is often enough. Stop looping when the human has no active
comments left, and say what you changed in your final summary.

## Failure modes to avoid

- Inventing line numbers from the source file instead of the patch image —
  the reviewer sees orphaned comments and trusts the rest of the review
  less.
- Acting on dismissed comments "because they still seem right". The human
  saw them and said no; the conversation depends on that being final.
- Overwriting the human's edits by regenerating the file before reading it.
  Always read the current file first; your copy is stale the moment the TUI
  opens.
- Padding the review. Five comments the human mostly accepts beat twenty
  they must triage; dismissals are expensive human attention.
