# Revue d'une pull request GitHub

Nécessite [`gh`](https://cli.github.com/) authentifié avec `gh auth login`.
Les hôtes Enterprise fonctionnent via l'authentification propre à `gh`.

## Ouvrir une pull request

```bash
leanreview 418                                   # infers owner/repo from origin
leanreview owner/repo#418
leanreview https://github.com/owner/repo/pull/418
```

leanreview récupère le **diff canonique** de la PR via `gh` — la
représentation exacte à laquelle GitHub ancre les commentaires de revue,
donc les positions correspondent toujours à ce que GitHub affiche — ainsi
que les métadonnées de la PR et ses fils de discussion existants.

La première ligne de la barre de titre affiche un badge `gh`, la référence
de la PR et son titre.

## Le panneau superposé de la pull request

Appuyez sur `p` (ou `:pr`) pour voir les détails de la pull request : titre,
auteur, branches, URL, et la description rendue en Markdown stylisé.
`j`/`k` font défiler, `esc`/`p` ferme.

![PR details overlay](../screens/pr-overlay.svg)

Sous la description, le panneau liste la **conversation** de la PR — les
commentaires généraux rattachés à la demande plutôt qu'à une ligne — du
plus ancien au plus récent, avec auteurs et horodatages. Les images
jointes à la description ou aux commentaires de conversation sont rendues
directement dans le panneau.

![La conversation dans le panneau, avec son image rendue](../screens/pr-overlay-kitty.png)

*Un commentaire général de conversation portant une photo, dessinée dans
le panneau sur ghostty via le protocole graphique kitty.*

## La conversation générale

`P` ouvre la conversation dans son propre écran : `j`/`k` sélectionnent un
commentaire, `r`/`Enter` répond, `a` ajoute un commentaire général. Les
conversations des deux forges sont plates : une réponse est donc un
nouveau commentaire pré-rempli d'une citation Markdown de ce à quoi il
répond. Tout ce que vous écrivez est mis en brouillon — marqué *(draft —
posts on submit)*, modifiable avec `e` et supprimable avec `d` — et part
à l'envoi de la revue.

Sur l'écran d'envoi, `g` rédige le **résumé de la revue** — le commentaire
général attaché à la revue elle-même (le corps de la review sur GitHub, la
note d'ouverture sur GitLab).

## Fils de discussion et réponses

Les fils de discussion de revue existants apparaissent comme des marqueurs
`◆` dans la gouttière, avec le commentaire racine prévisualisé en ligne. Sur
une ligne marquée :

- `Enter` ouvre le fil complet (racine, réponses, indicateurs
  résolu/obsolète).
- `r` rédige une réponse dans votre éditeur. Les réponses sont mises en
  attente comme brouillons et publiées lors de la soumission.

`C` liste vos brouillons et, en dessous, chaque fil de discussion existant.

## Soumettre

`s` (ou `:comment` / `:approve` / `:request`) ouvre l'écran de soumission :

![Submission confirmation](../screens/submit.svg)

Choisissez l'événement avec `c`/`a`/`R`, puis `y` soumet. Tous les
commentaires de ligne en brouillon partent en **une seule revue atomique** ;
les réponses en attente sont publiées sur leurs fils de discussion ensuite.
Rien n'est jamais envoyé avant cette confirmation.

Si une réponse échoue après la création de la revue, tout ce que GitHub a
déjà accepté est retiré de vos brouillons — une nouvelle tentative ne
soumet que ce qui est encore en attente, jamais une revue en double.

## Quand la tête de la PR bouge

Si de nouveaux commits arrivent entre vos sessions, les brouillons
enregistrés sont réancrés au nouveau diff en faisant correspondre le
contexte environnant capturé de chaque commentaire (en suivant les
renommages), et ne sont relocalisés qu'en cas de correspondance unique. Les
commentaires qui ne peuvent pas être placés sont marqués comme
**orphelins**, exclus de la soumission, et conservés pour que vous les
repositionniez — voir [Concepts](../concepts.md).
