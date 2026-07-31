# Revue d'une merge request GitLab

Nécessite [`glab`](https://gitlab.com/gitlab-org/cli) authentifié avec
`glab auth login`. L'adaptateur est choisi selon l'hôte, donc la TUI est
identique au flux GitHub — le badge de la barre de titre affiche `glab`.

## Ouvrir une merge request

```bash
leanreview 'https://gitlab.com/group/repo/-/merge_requests/42'
leanreview 'group/repo!42'                # nested subgroups work too
leanreview 42                             # inside a checkout with a GitLab origin
```

Le point d'accès des modifications de GitLab renvoie des hunks par fichier ;
leanreview reconstruit le patch unifié canonique à partir de ceux-ci, donc
les positions des commentaires s'alignent avec ce que GitLab affiche.

Tout ce qui vient du [flux GitHub](github.md) s'applique : le panneau
superposé de détails `p`, les marqueurs de fil `◆`, `Enter` pour lire un
fil, `r` pour mettre en attente une réponse, `C` pour la liste des
commentaires.

## Comment la soumission se transpose sur GitLab

GitLab n'a pas de point d'accès de revue atomique avec commentaires, donc
`s` transpose la soumission sur le modèle de GitLab :

- chaque commentaire de ligne en brouillon devient une **discussion
  positionnée sur le diff** ;
- le résumé de la revue devient une **note** sur la merge request ;
- **Approve** approuve la MR ;
- **Request changes** publie une note « Changes requested ».

Les commentaires sont publiés dans l'ordre, et un échec en cours de route
rapporte combien ont déjà été publiés — ceux-ci sont retirés de vos
brouillons afin qu'une nouvelle tentative ne puisse pas les republier.
