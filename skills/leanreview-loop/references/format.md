# Review-exchange format quick reference (version 1)

Full specification: https://perrito666.github.io/leanreview/reference/exchange-format/
JSON Schema: https://perrito666.github.io/leanreview/schema/leanreview-review.schema.json

A review exchange is one JSON document (UTF-8), conventionally named
`*.review.json`. It is detected by content: the `"leanreview_review"` key
near the start.

## Document

| Field | Required | Notes |
| --- | --- | --- |
| `leanreview_review` | yes | Integer version, currently `1`. Keep it the first key. |
| `title` | no | Shown in the TUI title bar. |
| `summary` | no | Review-level verdict/overview, Markdown. |
| `patch` | yes | The unified diff as an **array of lines** (writers must emit the array form; a plain string is accepted on read only). No trailing empty element. |
| `comments` | yes | Array; may be empty. |

## Comment

| Field | Required | Notes |
| --- | --- | --- |
| `id` | recommended | Stable across round trips; assign `c1`, `c2`, … Readers preserve it. |
| `author` | no | `"assistant"` for LLM comments; humans get their own name. |
| `path` | yes | Exactly as spelled in the patch. |
| `side` | yes | `RIGHT` = post-image (added + context lines), `LEFT` = pre-image (deleted + context). |
| `start_line` | yes | 1-based line number **in that image of the patch**, not in your memory of the file. |
| `end_line` | no | Inclusive span end; defaults to `start_line`. |
| `body` | yes | Markdown. |
| `state` | no | `active` (default) / `dismissed` / `orphaned` / `stale`. You write `active`; humans produce `dismissed`. |
| `snippet` | no | The anchored line's exact text. Fill it: it is the error-detector for wrong line numbers. |
| `at` | no | RFC 3339 creation timestamp. Set it — chronology helps across rounds. |
| `replies` | no | `[{"author": "...", "body": "...", "at": "..."}]`, oldest first; `at` optional. |

## Semantics that matter

- **`dismissed` is final.** Keep the comment in the file, never act on it,
  never re-raise it.
- **Orphaned on import**: if `(path, side, start_line)` does not exist in
  the embedded patch, leanreview imports the comment as `orphaned` — a
  visible flag, not data loss. Validate before handoff to keep this at zero.
- **Round trips are lossless** for `id`, `author`, `replies`, `patch`,
  `title`, `summary`. Unknown fields are ignored and NOT preserved — do not
  stash data in custom fields.
- **Multi-line comments**: `start_line`..`end_line` on one side only.

## Worked example

```json
{
  "leanreview_review": 1,
  "title": "auth refactor",
  "summary": "One blocking issue, one nit.",
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
      "body": "`err` is assigned and discarded — return it instead of `_ = err`.",
      "snippet": "    result, err := calculate(input)"
    }
  ]
}
```

Line-number arithmetic for the example: the hunk header `@@ -70,5 +70,6 @@`
says the post-image starts at line 70. Counting post-image lines (context
and `+` lines only): 70 = `ctx := ...`, 71 = `defer cancel(ctx)`, 72 =
`result, err := calculate(input)` (the first `+` line), 73 = `_ = err`,
74 = `return result, nil`. The deleted line (`-`) does not count in the
post-image; it would be LEFT line 72.
