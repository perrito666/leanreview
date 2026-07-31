# Configuration

Settings resolve in increasing precedence: built-in defaults → config file →
environment → command-line flags. The config file lives at
`$XDG_CONFIG_HOME/leanreview/config.json`
(`~/.config/leanreview/config.json`); a malformed file is ignored with a
warning printed on startup.

```json
{
  "editor": "nvim -f",
  "syntax": true,
  "syntax_style": "github",
  "theme": "default",
  "tab_width": 4,
  "context": 3,
  "keys": { " ": "down", "m": "next-hunk" },
  "list_engine": "gh",
  "list_filter": "is:open review-requested:@me",
  "list_filters": { "bugs": "is:open label:bug" },
  "wrap": true,
  "wrap_width": 120
}
```

## Settings

- `editor` — editor command (parsed as a command line, e.g. `code --wait`).
- `syntax` / `syntax_style` — enable highlighting and pick a Chroma style.
  The default, `auto`, matches your terminal background (`monokai` on dark,
  `github` on light); any
  [Chroma style name](https://xyproto.github.io/splash/docs/) can be set
  explicitly.
- `theme` — TUI palette: `default` or `mono`.
- `tab_width` — columns a tab expands to.
- `context` — default unified context lines when `-U` is not passed.
- `keys` — remap normal-mode bindings, `{ "<key>": "<action>" }`. See
  [Keys](keys.md#remapping).
- `list_engine` / `list_filter` / `list_filters` — defaults for `--list`:
  the discovery engine (`gh` or `glab`), the fallback search filter, and a
  map of named filters selectable as `--list :name` / `--list engine:name`.
- `author` — the name attributed to your replies in
  [review-exchange conversations](exchange-format.md) (default: `$USER`).
- `wrap` / `wrap_width` — wrapping of long diff lines and comment previews
  (default on, `w` toggles). Code wraps hard at the column edge, comments at
  word boundaries; in unified layout the wrap point is
  `min(wrap_width, view)` (default 120), in split layout the side panel's
  width. With wrapping off, long lines clip and `h`/`l` scroll them.

## Environment

| Variable | Effect |
| --- | --- |
| `LEANREVIEW_EDITOR` | editor command (overrides the config file) |
| `LEANREVIEW_SYNTAX=0` | disable syntax highlighting |
| `NO_COLOR` | disable all color (mono theme) |
| `LEANREVIEW_AUTHOR` | reply attribution name (overrides `author`) |
| `LEANREVIEW_LOG` | log file path |

## State on disk

- Drafts: `$XDG_STATE_HOME/leanreview/drafts/` (`~/.local/state/…`), one
  JSON file per source, atomic writes.
- Logs: `$XDG_STATE_HOME/leanreview/leanreview.log` — never stdout, which
  the TUI owns.
