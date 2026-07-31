# leanreview

Un client de revue de code en terminal. Il permet de revoir un **fichier
patch/diff**, une **comparaison git locale**, une **pull request GitHub** ou
une **merge request GitLab** dans la même TUI : naviguer dans le diff,
attacher des commentaires en brouillon ancrés à des emplacements sémantiques
du diff, puis **les exporter en Markdown** (idéal pour renvoyer des notes de
revue à une IA en guise de retour dans un prompt) ou **les soumettre comme
une véritable revue**.

C'est un client de revue, pas un client git : les binaires `git`, `gh` et
`glab` installés gèrent la sémantique du dépôt et de la forge ; leanreview se
charge de la navigation, du rendu, de l'état de la revue et des commentaires.

![The main view](screens/main.svg)

*La vue principale : barre latérale des fichiers modifiés, diff unifié, un
commentaire en brouillon dans son encadré en ligne (`●`), et un marqueur de
fil de discussion existant (`◆`).*

## Installation

```bash
go install github.com/perrito666/leanreview/cmd/leanreview@latest
```

Ou depuis un clone :

```bash
make            # builds ./leanreview
make install    # installs into GOBIN
```

Dépendances optionnelles par mode : [`gh`](https://cli.github.com/) (après
`gh auth login`) pour GitHub, [`glab`](https://gitlab.com/gitlab-org/cli)
(après `glab auth login`) pour GitLab. La revue de fichier patch et de git
local n'a besoin que de `git` — et un simple fichier patch n'a même pas
besoin de cela.

## Démarrage rapide

```bash
leanreview change.diff        # review a patch file
leanreview --base main        # review this branch against main
leanreview 418                # review PR/MR 418 of this repo's origin
leanreview --list             # pick something from your review queue
```

## Pour aller plus loin

- **[Revue sans forge](workflows/local.md)** — fichiers patch, entrée
  standard, arbre de travail, modifications indexées, plages de révisions et
  export Markdown.
- **[Revue d'une pull request GitHub](workflows/github.md)** — fils de
  discussion, réponses et soumission atomique de la revue via `gh`.
- **[Revue d'une merge request GitLab](workflows/gitlab.md)** — le même
  déroulement via `glab`, et comment la soumission se transpose sur le
  modèle de GitLab.
- **[Découvrir quoi revoir](workflows/discovery.md)** — `--list`, filtres par
  moteur et filtres nommés.
- **[Concepts](concepts.md)** — comment fonctionnent les brouillons,
  l'ancrage et la relocalisation.
- Référence **[Touches](reference/keys.md)**, **[Ligne de commande](reference/cli.md)**
  et **[Configuration](reference/configuration.md)**.
