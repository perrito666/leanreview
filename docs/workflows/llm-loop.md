# Reviewing with an LLM

leanreview can be the human half of an **offline review conversation** with
an LLM: the model reviews a diff and writes its comments into a single
self-contained file, you triage them in the TUI, and the model reads your
verdict back and acts on it. A typical setup: before a PR is submitted, an
LLM performs a first review pass; a human dismisses or refines those
comments; the LLM uses the result to improve the change.

The medium is the [review exchange format](../reference/exchange-format.md)
— a JSON document (`*.review.json`) carrying the diff and its comments.

## The loop

**1. The LLM writes the review.** It diffs the change, writes
`topic.review.json` with its comments anchored to diff lines, and validates
its own anchors non-interactively:

```bash
leanreview topic.review.json --export /tmp/check.md   # orphaned = bad anchor
```

**2. You triage in the TUI.**

```bash
leanreview topic.review.json
```

The model's comments appear boxed inline under the diff lines they discuss,
attributed (`@assistant`). Then:

- `x` — **dismiss** a comment you disagree with. It stays in the file with
  `state: "dismissed"` so the model knows not to raise it again; `x` again
  restores it.
- `r` — **reply** to a comment: answer a question, explain a dismissal, or
  give direction without rewriting the model's words. Replies are
  attributed to you (`author` in the config, `LEANREVIEW_AUTHOR`, or
  `$USER`) and shown inside the comment's box.
- `e` — edit a comment's body to refine or correct it.
- `c` — add your own comments, exactly as in any review.
- `dd` — delete outright (leaves no trace in the conversation).

Every change **rewrites the file in place** — quit whenever you are done;
there is no export step.

**3. The LLM acts on your verdict.** Dismissed comments are dropped (and
never re-raised), edited bodies are the new instruction, and your own added
comments carry the highest weight. For another round, the model regenerates
the diff and a fresh exchange file, carrying over only what is still under
discussion.

## Ready-made agent skill

The repository ships a skill for the LLM side at
[`skills/leanreview-loop/`](https://github.com/perrito666/leanreview/tree/main/skills/leanreview-loop)
— drop it into your agent's skill directory (for Claude Code:
`.claude/skills/leanreview-loop/`) and asking for "a review before I open
the PR" runs this loop end to end.

## Editing the file directly

You normally never open the JSON by hand — the TUI is the editor. But the
file is designed to be readable: the patch is stored line-by-line, so it
diffs cleanly in git and the repository ships
[editor support](https://github.com/perrito666/leanreview/tree/main/editors)
(Vim/Neovim, VS Code, JetBrains) with diff colors and schema validation for
when you do look inside.

## Exporting a conversation from any review

Any leanreview session can start a conversation: `:export topic.review.json`
(or `--export topic.review.json`) writes the current diff and your draft
comments as an exchange file — hand it to an LLM as structured review
feedback instead of the flat Markdown export.
