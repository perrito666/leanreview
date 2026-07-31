# Keys

Press `?` inside the TUI for this reference at any time.

![Help overlay](../screens/help.svg)

## Navigation

| Key | Action |
| --- | --- |
| `j` / `k`, `↓` / `↑` | down / up a line (counts work: `3j`) |
| `J` / `K` | next / previous change |
| `]c` / `[c` | next / previous hunk |
| `Tab` / `Shift-Tab`, `]f` / `[f` | next / previous file |
| `gg` / `G` | first / last line |
| `Ctrl-d` / `Ctrl-u` | half page |
| `PgDn` / `PgUp` | full page |
| `h` / `l`, `←` / `→` | scroll long lines (unified) / target side (split) |
| `0` / `$` | scroll to line start / end |

## View

| Key | Action |
| --- | --- |
| `t` | toggle unified / split |
| `T` | toggle full-file context around the diff (lazy fetch, cached; `]c`/`[c` still jump hunks) |
| `S` | cycle syntax coloring: red/green changes · syntax everywhere (tinted) · off |
| `w` | toggle wrapping of long lines and comment previews |
| `i` | toggle inline comment previews |
| `\` | toggle changed-files sidebar |
| `za` / `zR` / `zM` | fold current hunk / expand all / collapse all |
| `/`, `n`, `N` | search diff text, next / previous match |
| `f` | file picker |
| `C` | comment list |
| `Enter` | open conversation (`●` line: edit/delete replies) / thread (`◆` line), else comment list |

## Review

| Key | Action |
| --- | --- |
| `v` / `V` | select lines / changed block |
| `c` | comment on line or selection (opens `$EDITOR`) |
| `e` | edit draft comment under cursor |
| `x` | dismiss / restore comment under cursor (kept, never submitted) |
| `r` | reply to the comment under cursor ([exchange conversations](exchange-format.md)) or its thread (PR mode) |
| `dd` | delete comment under cursor |

## Pull-request mode

| Key | Action |
| --- | --- |
| `p` (or `:pr`) | PR details: title, description, link |
| `s` | submit review (confirmation screen) |

## Commands

| Command | Action |
| --- | --- |
| `:w` | save drafts |
| `:export FILE` | export comments (`.json`: [review exchange](exchange-format.md), else Markdown) |
| `:comment` / `:approve` / `:request` | open submission with that event |
| `:q` / `q` | quit |

## Remapping

Single-key normal-mode bindings can be remapped via the `keys` map in the
[configuration](configuration.md). An empty action unbinds a key; single keys
may bind two-key actions (`next-hunk`, `delete-comment`, …). The two-key
sequences themselves (`gg`, `]c`, …) and numeric count prefixes are fixed.
