# Command line

```text
leanreview [flags] [target]        (the "review" verb is also accepted)
```

## Targets

| Target | Meaning |
| --- | --- |
| `<file.diff>` / `-` | a patch file / stdin |
| `.` (or nothing) | working tree vs HEAD |
| `<revA> <revB>` | explicit git revision range |
| `418`, `#418`, `!42` | PR/MR number (owner/repo inferred from origin) |
| `owner/repo#418`, `group/repo!42`, full URL | explicit PR/MR reference |

## Flags

| Flag | Meaning |
| --- | --- |
| `--base <ref>` | compare `<ref>...HEAD` (merge-base) instead of the working tree |
| `--staged` | compare the index against HEAD |
| `-U, --context N` | unified context lines (default 3, configurable) |
| `--export FILE` | write draft comments as Markdown and exit |
| `--discard` | delete the saved draft for this source and exit |
| `--list [engine] [filter]` | discover open PRs/MRs and pick one to review (plain table when piped) |
| `-h, --help` | show help |
| `-v, --version` | print the version and exit |

See [Discovering what to review](../workflows/discovery.md) for the full
`--list` selector syntax (engines, named filters, refinements).
