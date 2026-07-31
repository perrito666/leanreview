# Concepts

## Ancrage sémantique

La règle de conception centrale : **les lignes affichées à l'écran ne sont
jamais des emplacements canoniques pour les commentaires**. Un commentaire
s'ancre à un emplacement sémantique — chemin, côté (ancien/nouveau), plage de
lignes et lignes de contexte environnantes — et non à une coordonnée de
terminal. Cet ancrage survit au redimensionnement, au repliement des hunks,
au retour à la ligne et aux bascules unifié ↔ scindé, car ceux-ci ne changent
que la *projection* du diff, pas le diff lui-même.

## Brouillons

Chaque commentaire commence comme un **brouillon**, enregistré localement et
jamais envoyé nulle part avant soumission explicite. Les brouillons résident
dans `$XDG_STATE_HOME/leanreview/drafts/` (`~/.local/state/…`), un fichier
JSON par source, écrit de manière atomique.

Chaque source de revue possède une **clé d'identité stable** — un hachage du
contenu du patch pour les fichiers/l'entrée standard, la spécification du
diff pour les comparaisons git, l'hôte/dépôt/numéro pour les PR et MR — de
sorte que rouvrir la même source recharge ses brouillons, et que deux
sources différentes ne partagent jamais leur état. `--discard` supprime le
brouillon enregistré pour une source.

## Relocalisation et orphelins

En mode pull request, le diff peut changer sous vos brouillons enregistrés :
quelqu'un pousse de nouveaux commits entre vos sessions. Au chargement,
leanreview réancre chaque brouillon en faisant correspondre son contexte
environnant capturé avec le nouveau diff (en suivant les renommages de
fichiers), et ne relocalise un commentaire **qu'en cas de correspondance
unique** — un contexte qui correspond deux fois est ambigu, et deviner
placerait votre note sur la mauvaise ligne.

Les commentaires qui ne peuvent pas être placés sont marqués comme
**orphelins** : ils restent dans votre liste de brouillons, s'affichent avec
une étiquette `[orphaned]`, sont exclus de la soumission (l'écran de
confirmation avertit à leur sujet), et attendent que vous les repositionniez
ou les supprimiez. Rien n'est jamais abandonné silencieusement.

## Soumission

La soumission est explicite et atomique lorsque l'hôte le permet :

- **GitHub** — tous les commentaires de ligne partent en une seule revue
  avec l'événement choisi (comment / approve / request changes) ; les
  réponses de fil de discussion en attente sont publiées ensuite.
- **GitLab** — n'a pas de point d'accès de revue atomique, donc les
  commentaires deviennent des discussions positionnées sur le diff, publiées
  dans l'ordre, et l'événement se transpose sur une approbation ou une note.

Dans les deux cas, tout ce que l'hôte a accepté est immédiatement retiré de
vos brouillons — une nouvelle tentative après un échec partiel n'envoie que
ce qui est encore en attente, jamais de doublon.

## L'éditeur

`c`, `e` et `r` ouvrent votre éditeur, résolu dans cet ordre : `editor` dans
le fichier de configuration / `LEANREVIEW_EDITOR` → `GIT_EDITOR` → `VISUAL`
→ `EDITOR` → `git var GIT_EDITOR` → `vi`. Les valeurs sont analysées comme
des lignes de commande (`code --wait`, `nvim -f`). Le tampon est un fichier
Markdown avec un en-tête en commentaire HTML portant le contexte ; l'en-tête
est retiré automatiquement, et enregistrer un corps vide abandonne le
commentaire.
