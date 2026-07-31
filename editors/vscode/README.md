# VS Code support for review-exchange files

A minimal extension for `*.review.json`: JSON highlighting with diff colors
over the embedded `patch` array, plus JSON Schema validation and completions
via the published schema.

## Install

No marketplace needed — copy the folder into your extensions directory:

```bash
cp -r editors/vscode ~/.vscode/extensions/leanreview-exchange-1.0.0
```

then reload VS Code (`Developer: Reload Window`).

## Schema-only alternative

If you only want validation (keeping the stock JSON language), skip the
extension and add to your `settings.json`:

```json
"json.schemas": [
  {
    "fileMatch": ["*.review.json"],
    "url": "https://perrito666.github.io/leanreview/schema/leanreview-review.schema.json"
  }
]
```
