# JetBrains IDE support for review-exchange files

IntelliJ-platform IDEs (IDEA, GoLand, PyCharm, WebStorm, …) get the two
useful behaviours without a custom plugin:

## 1. Schema validation and completions

Settings → **Languages & Frameworks → Schemas and DTDs → JSON Schema
Mappings** → add:

- **Schema URL**:
  `https://perrito666.github.io/leanreview/schema/leanreview-review.schema.json`
- **Schema version**: JSON Schema version 2020-12
- **File path pattern**: `*.review.json`

You now get structure validation, enum completion for `side`/`state`, and
field documentation on hover.

## 2. Diff colors in the embedded patch

JetBrains IDEs can run TextMate grammars for languages they do not know
natively. Import the grammar shipped for VS Code:

Settings → **Editor → TextMate Bundles** → **+** → select this repository's
`editors/vscode` directory.

Note: TextMate bundles apply only to files not already claimed by a native
language. To route `*.review.json` to the bundle instead of the built-in JSON
support, remove that pattern from the JSON file type (Settings → **Editor →
File Types → JSON**) — or simply keep native JSON plus the schema mapping
from step 1, which is the recommended setup: validation matters more than
the diff colors here.

A full custom-language plugin would add little beyond the above and carries a
real maintenance cost, so it is intentionally not provided.
