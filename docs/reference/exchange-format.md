# Review exchange format

The review exchange is leanreview's file format for **offline review
conversations**: a single JSON document carrying a unified diff and the
comments on it, passed back and forth between tools. The canonical loop is an
LLM writing a review, a human editing it in the leanreview TUI, and the LLM
reading the result back — but any two parties that speak the format can hold
the conversation.

- **Media type**: JSON (UTF-8). Recommended filename: `*.review.json`.
- **Version**: `1` (this page). A machine-readable
  [JSON Schema](../schema/leanreview-review.schema.json) is published for
  validation and editor tooling.
- **Detection**: by content, not filename — the presence of the
  `"leanreview_review"` key near the start of a JSON object. `leanreview
  <file>` opens any file that sniffs as an exchange in conversation mode.

## Example

```json
{
  "leanreview_review": 1,
  "title": "auth refactor",
  "summary": "Two issues, one blocking.",
  "patch": [
    "diff --git a/internal/api/handler.go b/internal/api/handler.go",
    "--- a/internal/api/handler.go",
    "+++ b/internal/api/handler.go",
    "@@ -70,5 +70,6 @@ func Handle(input Input) (Output, error) {",
    "     ctx := context.Background()",
    "     defer cancel(ctx)",
    "-    result := calculate(input)",
    "+    result, err := calculate(input)",
    "+    _ = err",
    "     return result, nil"
  ],
  "comments": [
    {
      "id": "c1",
      "author": "assistant",
      "path": "internal/api/handler.go",
      "side": "RIGHT",
      "start_line": 72,
      "body": "`err` is assigned and discarded — return it instead.",
      "state": "active",
      "snippet": "result, err := calculate(input)",
      "replies": [
        { "author": "hduran", "body": "Agreed, but only log it: callers can't handle it." }
      ]
    }
  ]
}
```

## Document fields

| Field | Required | Meaning |
| --- | --- | --- |
| `leanreview_review` | yes | Format version, integer. Readers **must** reject unknown versions. Keep it the first key: it doubles as the sniffing marker. |
| `title` | no | Human-readable review title (shown in the TUI title bar). |
| `summary` | no | Review-level overview/verdict; round-trips into the submission summary. |
| `patch` | yes | The unified diff the comments anchor to (see below). |
| `comments` | yes | The conversation; may be empty. |

Unknown fields anywhere in the document are **tolerated**: readers ignore
them, so additive evolution happens without a version bump — but
intermediaries (including leanreview's writeback) do not preserve fields they
do not know, so writers must not rely on extra fields surviving a round trip.
Incompatible changes bump `leanreview_review`.

## The patch

The patch is the complete unified diff (git style) the conversation is about.
Embedding it — rather than referencing a commit — is what makes the file
self-contained and the conversation possible offline.

Writers **must** emit it as a **JSON array of lines**, one diff line per
element, without a trailing empty element (the final newline is implicit).
Readers **must** also accept a single string (with embedded `\n`) for
leniency toward hand-rolled writers.

The array form is deliberate: it keeps the file readable to humans, makes
round trips produce meaningful line-level text diffs (the conversation file
itself is often kept in git), and lets editors highlight the embedded diff —
see [editor support](#editor-support).

## Comments

| Field | Required | Meaning |
| --- | --- | --- |
| `id` | recommended | Stable identity across round trips. leanreview preserves ids and assigns random ones when absent. |
| `author` | no | Free-form attribution (`"assistant"`, a username). Shown in the TUI. |
| `path` | yes | File path as it appears in the patch. |
| `side` | yes | `LEFT` (old image: deleted lines) or `RIGHT` (new image: added/context lines). Case-insensitive. |
| `start_line` | yes | 1-based line in that image of the patch. |
| `end_line` | no | Inclusive multi-line span end; defaults to `start_line`. |
| `body` | yes | Markdown comment text. |
| `state` | no | `active` (default), `dismissed`, `orphaned`, or `stale`. |
| `snippet` | no | The anchored diff line(s), informational. Recomputed when empty. |
| `at` | no | RFC 3339 creation timestamp. |
| `replies` | no | Follow-ups, oldest first: `{ "author", "body", "at" }` (`at` optional). |

Timestamps are optional but part of version 1 on purpose: intermediaries do
not preserve unknown fields, so anything a round trip must carry has to
exist from the start. Replies accumulate across rounds, making their
chronology part of the conversation.

### States — the conversation protocol

`state` is how the human's verdict travels back to the review's author:

- **`active`** — the comment stands. The next actor should address it.
- **`dismissed`** — a human rejected it. It is kept in the file (the verdict
  itself is information) but must **not** be acted on or resubmitted.
  In the TUI, `x` toggles dismissal.
- **`orphaned`** — the anchor no longer resolves against the patch (or the
  next diff). leanreview sets this on import when `(path, side, start_line)`
  does not exist in the embedded patch; orphaned comments are never
  submitted.
- **`stale`** — the anchor may have moved; a re-anchoring pass is pending.

A `dismissed` comment keeps its state even when it no longer anchors:
the human decision outranks the anchoring failure.

### Anchoring rules

Line numbers are interpreted against the **embedded patch**, with the same
semantics as forge review comments: `side` picks the pre-/post-image, and
`start_line` is the 1-based line number in that image. On import, leanreview
resolves each comment against the parsed patch:

- resolvable → the comment gets a context anchor (surrounding lines), so it
  can survive later diff revisions by content matching;
- not resolvable → the comment imports as `orphaned` rather than being
  dropped or guessed: an off-by-one from a generator is a visible, fixable
  condition, never silent data loss.

Writers should double-check line numbers against the patch they embed; the
`snippet` field exists so readers (human or LLM) can spot mismatches at a
glance.

## Round-trip guarantees

When leanreview opens an exchange file, edits happen in the TUI and **every
draft save rewrites the file in place** (quitting saves). The following
survive a round trip untouched: comment `id`s, `author`s, `at` timestamps,
`replies`, the `patch`, `title`, and `summary`. The TUI changes only what
the human changed: bodies, states, replies they authored (`r` on a comment),
and the set of comments (added/deleted ones).

Output is deterministic: two-space indentation, stable key order, one diff
line per patch element, trailing newline. Successive round trips of an
unchanged review are byte-identical.

## Design notes

The format went through deliberate iteration before being pinned:

1. **JSON over a bespoke text format.** An interleaved "diff with comment
   blocks" format was considered — it reads nicely and anchors comments
   implicitly. It lost because the primary writers and readers are programs
   (LLMs emit valid JSON far more reliably than bespoke syntax), the human
   edits in the TUI rather than a text editor, and a custom parser is a
   lifetime of escaping bugs.
2. **Patch as line array, not string.** The first draft embedded the patch as
   one JSON string; a diff full of `\n` escapes on a single line is
   unreadable, undiffable, and un-highlightable. The array form fixes all
   three at no parsing cost, with string leniency kept for minimal writers.
3. **States instead of deletion.** Dismissed comments stay in the file
   because the rejection is what the other side of the conversation most
   needs to know.

## Editor support

The [`editors/`](https://github.com/perrito666/leanreview/tree/main/editors)
directory in the repository ships ready-made support:

- **Vim / Neovim** — a filetype (`lreview`) for `*.review.json` that layers
  diff colors (added/removed/hunk lines inside the `patch` array) on top of
  JSON highlighting.
- **VS Code** — a minimal extension with the same layered grammar, plus a
  JSON Schema association for validation and completions.
- **JetBrains IDEs** — a JSON Schema mapping (validation + completions) via
  the published schema URL; the TextMate grammar from the VS Code extension
  can be imported for the diff colors.

All of them key off the `*.review.json` naming convention — another reason to
follow it.
