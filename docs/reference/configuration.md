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

## Generating and validating

Start from a complete baseline instead of a blank file:

```bash
leanreview --init-config     # writes the file above with every default,
                             # the full keymap, and a $schema reference
```

The generated `keys` map lists **every** default binding, so remapping is an
edit, not a guessing game. The generator refuses to overwrite an existing
file — for a config that already exists, fill it instead:

```bash
leanreview --fill-config     # adds every missing setting, binding, and
                             # sequence at its default; existing values and
                             # unknown keys are never touched
```

Validate at any time:

```bash
leanreview --check-config    # reports typos, unknown actions, bad values;
                             # non-zero exit when problems exist
```

Editors validate too: the `$schema` reference points at the published
[config schema](../schema/leanreview-config.schema.json), giving validation
and completion (including action names for `keys`) in VS Code and JetBrains
out of the box — see
[`editors/`](https://github.com/perrito666/leanreview/tree/main/editors)
for Vim/Neovim setup.

## Settings

- `editor` — editor command (parsed as a command line, e.g. `code --wait`).
- `syntax` / `syntax_style` — enable highlighting and pick a Chroma style.
  The default, `auto`, matches your terminal background (`monokai` on dark,
  `github` on light); any
  [Chroma style name](https://xyproto.github.io/splash/docs/) can be set
  explicitly.
- `theme` — TUI palette: a built-in (`default`, `default-light`,
  `default-dark`, `mono`) or the name of a [theme file](#themes).
- `tab_width` — columns a tab expands to.
- `context` — default unified context lines when `-U` is not passed.
- `keymap` — base binding set: a built-in preset (`default`, `vim`,
  `vscode`, `sublime`, `intellij`) or the name of an entry in `keymaps`.
  Presets layer familiar chords over the defaults (terminals cannot report
  every editor chord, so they are supersets, not clones); `keys` and
  `sequences` still override on top.
- `keymaps` — user-defined named keymaps,
  `{ "<name>": { "keys": {…}, "sequences": […] } }`, each layered on the
  defaults when selected. Built-in preset names are reserved.
- `keys` — remap normal-mode bindings, `{ "<key>": "<action>" }`. See
  [Keys](keys.md#remapping).
- `sequences` — remap two-key sequences, a list of
  `{ "keys": ["<k1>", "<k2>"], "action": "<action>" }` objects. See
  [Keys](keys.md#remapping).
- `list_engine` / `list_filter` / `list_filters` — defaults for `--list`:
  the discovery engine (`gh` or `glab`), the fallback search filter, and a
  map of named filters selectable as `--list :name` / `--list engine:name`.
- `change_colors` — how `+`/`-` lines are colored when syntax highlighting
  is on: `diff` (default: classic red/green, syntax reserved for context
  lines) or `syntax` (syntax colors everywhere).
- `change_tint` — with `change_colors: syntax`, back changed lines with a
  faint red/green background so the diff stays legible at a glance
  (default `true`).
- `images` — comment-image rendering: `auto` (kitty graphics on
  kitty/ghostty, `chafa` when installed, off otherwise), `kitty`, `chafa`,
  or `off`. In PR/MR mode, comment attachments (GitHub `user-attachments`,
  GitLab `/uploads`, including HTML `<img>` embeds) are fetched through the
  forge CLI's authentication and cached; other remote URLs render as tags.
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
| `LEANREVIEW_IMAGES` | image rendering backend (overrides `images`) |
| `LEANREVIEW_LOG` | log file path |

## State on disk

- Drafts: `$XDG_STATE_HOME/leanreview/drafts/` (`~/.local/state/…`), one
  JSON file per source, atomic writes.
- Logs: `$XDG_STATE_HOME/leanreview/leanreview.log` — never stdout, which
  the TUI owns.

## Themes

`default` adapts to the terminal background; `default-light` and
`default-dark` pin that choice. Beyond the built-ins, drop JSON theme files
into a `themes/` folder next to the config. A theme is referenced by its
**metadata name** (not its filename), restyles only the roles it names
(everything else keeps the default palette), and may not claim a built-in
name:

```json
{
  "$schema": "https://perrito666.github.io/leanreview/schema/leanreview-theme.schema.json",
  "name": "dusk",
  "description": "muted greens, magenta accents",
  "styles": {
    "addition":      { "fg": "108" },
    "deletion":      { "fg": "#d75f5f", "bold": true },
    "title":         { "fg": "15", "bg": "53" },
    "addition_tint": { "bg": "236" }
  }
}
```

Roles: `addition`, `deletion`, `context`, `metadata`, `gutter`, `cursor`,
`select`, `search`, `marker`, `title`, `status`, `error`, `key`, `faint`,
`comment`, `addition_tint`, `deletion_tint`. Colors are ANSI palette
indices or hex. `leanreview --check-config` validates the folder: broken
files, duplicate or reserved names, and unknown roles are all reported.
