# Configuration

Les réglages se résolvent par ordre de priorité croissante : valeurs par
défaut intégrées → fichier de configuration → environnement → options de
ligne de commande. Le fichier de configuration se trouve à
`$XDG_CONFIG_HOME/leanreview/config.json`
(`~/.config/leanreview/config.json`) ; un fichier malformé est ignoré avec
un avertissement affiché au démarrage.

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

## Réglages

- `editor` — commande de l'éditeur (analysée comme une ligne de commande,
  p. ex. `code --wait`).
- `syntax` / `syntax_style` — active la coloration syntaxique et choisit un
  style Chroma. La valeur par défaut, `auto`, s'adapte à l'arrière-plan de
  votre terminal (`monokai` sur fond sombre, `github` sur fond clair) ; tout
  [nom de style Chroma](https://xyproto.github.io/splash/docs/) peut être
  défini explicitement.
- `theme` — palette de la TUI : `default` ou `mono`.
- `tab_width` — nombre de colonnes vers lesquelles une tabulation s'étend.
- `context` — nombre de lignes de contexte unifié par défaut quand `-U`
  n'est pas passé.
- `keys` — remappe les liaisons du mode normal, `{ "<key>": "<action>" }`.
  Voir [Touches](keys.md#remapping).
- `list_engine` / `list_filter` / `list_filters` — valeurs par défaut pour
  `--list` : le moteur de découverte (`gh` ou `glab`), le filtre de
  recherche de secours, et une table de filtres nommés sélectionnables sous
  la forme `--list :name` / `--list engine:name`.
- `author` — le nom attribué à vos réponses dans les
  [conversations d'échange de revue](exchange-format.md)
  (par défaut : `$USER`).
- `wrap` / `wrap_width` — retour à la ligne des lignes de diff longues et
  des aperçus de commentaires (activé par défaut, `w` bascule). Le code
  revient à la ligne strictement au bord de la colonne, les commentaires
  aux limites de mots ; en disposition unifiée, le point de retour à la
  ligne est `min(wrap_width, view)` (120 par défaut), en disposition scindée
  c'est la largeur du panneau latéral. Retour à la ligne désactivé, les
  lignes longues sont tronquées et `h`/`l` les font défiler.

## Environnement

| Variable | Effet |
| --- | --- |
| `LEANREVIEW_EDITOR` | commande de l'éditeur (remplace le fichier de configuration) |
| `LEANREVIEW_SYNTAX=0` | désactive la coloration syntaxique |
| `NO_COLOR` | désactive toute couleur (thème mono) |
| `LEANREVIEW_AUTHOR` | nom d'attribution des réponses (remplace `author`) |
| `LEANREVIEW_LOG` | chemin du fichier journal |

## État sur le disque

- Brouillons : `$XDG_STATE_HOME/leanreview/drafts/` (`~/.local/state/…`),
  un fichier JSON par source, écritures atomiques.
- Journaux : `$XDG_STATE_HOME/leanreview/leanreview.log` — jamais la sortie
  standard, dont la TUI est propriétaire.
