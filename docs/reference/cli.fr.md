# Ligne de commande

```text
leanreview [flags] [target]        (the "review" verb is also accepted)
```

## Cibles

| Cible | Signification |
| --- | --- |
| `<file.diff>` / `-` | un fichier patch / l'entrée standard |
| `.` (ou rien) | l'arbre de travail par rapport à HEAD |
| `<revA> <revB>` | plage de révisions git explicite |
| `418`, `#418`, `!42` | numéro de PR/MR (owner/repo déduit de l'origine) |
| `owner/repo#418`, `group/repo!42`, URL complète | référence de PR/MR explicite |

## Options

| Option | Signification |
| --- | --- |
| `--base <ref>` | comparer `<ref>...HEAD` (merge-base) plutôt que l'arbre de travail |
| `--staged` | comparer l'index à HEAD |
| `-U, --context N` | lignes de contexte unifié (3 par défaut, configurable) |
| `--export FILE` | écrire les commentaires en brouillon en Markdown et quitter |
| `--discard` | supprimer le brouillon enregistré pour cette source et quitter |
| `--list [engine] [filter]` | découvrir les PR/MR ouvertes et en choisir une à revoir (table brute si redirigé via un tube) |
| `-h, --help` | afficher l'aide |
| `-v, --version` | afficher la version et quitter |

Voir [Découvrir quoi revoir](../workflows/discovery.md) pour la syntaxe
complète du sélecteur `--list` (moteurs, filtres nommés, affinements).
