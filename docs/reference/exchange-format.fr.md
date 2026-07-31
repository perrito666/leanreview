# Format d'échange de revue

L'échange de revue (« review exchange ») est le format de fichier de
leanreview pour les **conversations de revue hors ligne** : un document JSON
unique portant un diff unifié et les commentaires qui s'y rapportent, passé
d'un outil à l'autre. La boucle canonique est un LLM qui écrit une revue, un
humain qui l'édite dans la TUI leanreview, et le LLM qui relit le résultat —
mais deux parties quelconques parlant ce format peuvent tenir la
conversation.

- **Type de média** : JSON (UTF-8). Nom de fichier recommandé :
  `*.review.json`.
- **Version** : `1` (cette page). Un
  [schéma JSON](../schema/leanreview-review.schema.json) lisible par machine
  est publié pour la validation et l'outillage éditeur.
- **Détection** : par le contenu, pas par le nom de fichier — la présence de
  la clé `"leanreview_review"` près du début d'un objet JSON. `leanreview
  <file>` ouvre tout fichier qui se reconnaît comme un échange en mode
  conversation.

## Exemple

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

## Champs du document

| Champ | Requis | Signification |
| --- | --- | --- |
| `leanreview_review` | oui | Version du format, entier. Les lecteurs **doivent** rejeter les versions inconnues. À garder comme première clé : elle sert aussi de marqueur de détection. |
| `title` | non | Titre de la revue lisible par un humain (affiché dans la barre de titre de la TUI). |
| `summary` | non | Aperçu/verdict au niveau de la revue ; fait l'aller-retour vers le résumé de soumission. |
| `patch` | oui | Le diff unifié auquel les commentaires s'ancrent (voir ci-dessous). |
| `comments` | oui | La conversation ; peut être vide. |

Les champs inconnus, où qu'ils soient dans le document, sont **tolérés** :
les lecteurs les ignorent, ce qui permet une évolution additive sans
incrémenter la version — mais les intermédiaires (y compris la réécriture de
leanreview) ne préservent pas les champs qu'ils ne connaissent pas, donc les
générateurs ne doivent pas compter sur la survie de champs supplémentaires à
un aller-retour. Les changements incompatibles incrémentent
`leanreview_review`.

## Le patch

Le patch est le diff unifié complet (style git) dont traite la conversation.
L'intégrer — plutôt que de référencer un commit — est ce qui rend le fichier
autonome et rend la conversation possible hors ligne.

Les générateurs **doivent** l'émettre comme un **tableau JSON de lignes**,
une ligne de diff par élément, sans élément vide final (le saut de ligne
final est implicite). Les lecteurs **doivent** aussi accepter une chaîne
unique (avec des `\n` intégrés) par tolérance envers les générateurs
rudimentaires.

La forme en tableau est délibérée : elle garde le fichier lisible pour les
humains, fait que les allers-retours produisent des diffs texte
significatifs ligne par ligne (le fichier de conversation lui-même est
souvent conservé dans git), et permet aux éditeurs de surligner le diff
intégré — voir [support éditeur](#editor-support).

## Commentaires

| Champ | Requis | Signification |
| --- | --- | --- |
| `id` | recommandé | Identité stable à travers les allers-retours. leanreview préserve les ids et en assigne des aléatoires quand ils sont absents. |
| `author` | non | Attribution libre (`"assistant"`, un nom d'utilisateur). Affichée dans la TUI. |
| `path` | oui | Chemin du fichier tel qu'il apparaît dans le patch. |
| `side` | oui | `LEFT` (image ancienne : lignes supprimées) ou `RIGHT` (image nouvelle : lignes ajoutées/de contexte). Insensible à la casse. |
| `start_line` | oui | Ligne 1-indexée dans cette image du patch. |
| `end_line` | non | Fin de la portée multi-lignes incluse ; par défaut `start_line`. |
| `body` | oui | Texte du commentaire en Markdown. |
| `state` | non | `active` (par défaut), `dismissed`, `orphaned`, ou `stale`. |
| `snippet` | non | La ou les lignes de diff ancrées, à titre informatif. Recalculé si vide. |
| `at` | non | Horodatage de création au format RFC 3339. |
| `replies` | non | Réponses, de la plus ancienne à la plus récente : `{ "author", "body", "at" }` (`at` optionnel). |

Les horodatages sont optionnels mais font partie de la version 1 à dessein :
les intermédiaires ne conservent pas les champs inconnus, donc tout ce qu'un
aller-retour doit transporter doit exister dès le départ. Les réponses
s'accumulent au fil des rondes, et leur chronologie fait partie de la
conversation.

### États — le protocole de conversation

`state` est la façon dont le verdict de l'humain voyage jusqu'à l'auteur de
la revue :

- **`active`** — le commentaire tient. Le prochain acteur doit le traiter.
- **`dismissed`** — un humain l'a rejeté. Il est conservé dans le fichier (le
  verdict lui-même est une information) mais ne doit **pas** être traité ni
  soumis à nouveau. Dans la TUI, `x` bascule le rejet.
- **`orphaned`** — l'ancrage ne se résout plus contre le patch (ou le diff
  suivant). leanreview positionne cet état à l'import quand
  `(path, side, start_line)` n'existe pas dans le patch intégré ; les
  commentaires orphelins ne sont jamais soumis.
- **`stale`** — l'ancrage a peut-être bougé ; une passe de réancrage est en
  attente.

Un commentaire `dismissed` garde son état même quand il ne s'ancre plus : la
décision humaine prime sur l'échec d'ancrage.

### Règles d'ancrage

Les numéros de ligne sont interprétés par rapport au **patch intégré**, avec
la même sémantique que les commentaires de revue des forges : `side` choisit
l'image pré-/post-, et `start_line` est le numéro de ligne 1-indexé dans
cette image. À l'import, leanreview résout chaque commentaire par rapport au
patch analysé :

- résoluble → le commentaire reçoit un ancrage de contexte (lignes
  environnantes), pour pouvoir survivre à de futures révisions du diff par
  correspondance de contenu ;
- non résoluble → le commentaire est importé comme `orphaned` plutôt que
  d'être abandonné ou deviné : un décalage d'un générateur est une condition
  visible et corrigible, jamais une perte de données silencieuse.

Les générateurs devraient revérifier les numéros de ligne par rapport au
patch qu'ils intègrent ; le champ `snippet` existe pour que les lecteurs
(humains ou LLM) puissent repérer les décalages d'un coup d'œil.

## Garanties d'aller-retour

Quand leanreview ouvre un fichier d'échange, les modifications ont lieu dans
la TUI et **chaque enregistrement de brouillon réécrit le fichier sur
place** (quitter enregistre aussi). Ce qui suit survit à un aller-retour sans
changement : les `id` des commentaires, les `author`, les horodatages `at`, les `replies`, le
`patch`, `title`, et `summary`. La TUI ne change que ce que l'humain a
changé : les corps, les états, et l'ensemble des commentaires
(ajoutés/supprimés).

La sortie est déterministe : indentation de deux espaces, ordre des clés
stable, une ligne de diff par élément du patch, saut de ligne final. Les
allers-retours successifs d'une revue inchangée sont identiques octet par
octet.

## Notes de conception

Le format a connu plusieurs itérations délibérées avant d'être figé :

1. **JSON plutôt qu'un format texte maison.** Un format « diff entrelacé
   avec blocs de commentaires » a été envisagé — il se lit agréablement et
   ancre les commentaires implicitement. Il a été écarté parce que les
   principaux générateurs et lecteurs sont des programmes (les LLM émettent
   du JSON valide bien plus fiablement qu'une syntaxe maison), l'humain édite
   dans la TUI plutôt que dans un éditeur de texte, et un analyseur
   personnalisé est une source infinie de bugs d'échappement.
2. **Patch en tableau de lignes, pas en chaîne.** La première ébauche
   intégrait le patch comme une seule chaîne JSON ; un diff plein de `\n`
   échappés sur une seule ligne est illisible, indiffable, et
   in-surlignable. La forme en tableau règle les trois problèmes sans coût
   d'analyse supplémentaire, avec la tolérance aux chaînes conservée pour les
   générateurs minimalistes.
3. **États plutôt que suppression.** Les commentaires rejetés restent dans le
   fichier parce que le rejet est précisément ce que l'autre partie de la
   conversation a le plus besoin de savoir.

## Support éditeur

Le répertoire
[`editors/`](https://github.com/perrito666/leanreview/tree/main/editors) du
dépôt fournit un support prêt à l'emploi :

- **Vim / Neovim** — un filetype (`lreview`) pour `*.review.json` qui
  superpose des couleurs de diff (lignes ajoutées/supprimées/hunk dans le
  tableau `patch`) à la coloration syntaxique JSON.
- **VS Code** — une extension minimale avec la même grammaire superposée,
  plus une association de schéma JSON pour la validation et l'autocomplétion.
- **IDE JetBrains** — un mappage de schéma JSON (validation + autocomplétion)
  via l'URL du schéma publié ; la grammaire TextMate de l'extension VS Code
  peut être importée pour les couleurs de diff.

Tous s'appuient sur la convention de nommage `*.review.json` — une raison de
plus de la suivre.
