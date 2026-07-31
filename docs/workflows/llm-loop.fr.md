# Revue avec un LLM

leanreview peut être la moitié humaine d'une **conversation de revue hors
ligne** avec un LLM : le modèle relit un diff et écrit ses commentaires dans
un fichier autonome unique, vous les triez dans la TUI, et le modèle relit
votre verdict et agit en conséquence. Une configuration typique : avant
qu'une PR ne soit soumise, un LLM effectue une première passe de revue ; un
humain rejette ou affine ces commentaires ; le LLM utilise le résultat pour
améliorer la modification.

Le support est le [format d'échange de revue](../reference/exchange-format.md)
(« review exchange » en anglais) — un document JSON (`*.review.json`) portant
le diff et ses commentaires.

## La boucle

**1. Le LLM écrit la revue.** Il diffe la modification, écrit
`topic.review.json` avec ses commentaires ancrés aux lignes du diff, et
valide ses propres ancrages de façon non interactive :

```bash
leanreview topic.review.json --export /tmp/check.md   # orphaned = bad anchor
```

**2. Vous triez dans la TUI.**

```bash
leanreview topic.review.json
```

Les commentaires du modèle apparaissent encadrés en ligne sous les lignes du
diff qu'ils commentent, attribués (`@assistant`). Ensuite :

- `x` — **rejeter** un commentaire avec lequel vous êtes en désaccord. Il
  reste dans le fichier avec `state: "dismissed"` pour que le modèle sache
  ne pas le soulever à nouveau ; `x` à nouveau le restaure.
- `e` — modifier le corps d'un commentaire pour l'affiner ou le corriger.
- `c` — ajouter vos propres commentaires, exactement comme dans n'importe
  quelle revue.
- `dd` — supprimer purement et simplement (ne laisse aucune trace dans la
  conversation).

Chaque modification **réécrit le fichier sur place** — quittez dès que vous
avez terminé ; il n'y a pas d'étape d'export.

**3. Le LLM agit selon votre verdict.** Les commentaires rejetés sont
abandonnés (et jamais soulevés à nouveau), les corps modifiés deviennent la
nouvelle instruction, et vos propres commentaires ajoutés ont le poids le
plus élevé. Pour un nouveau tour, le modèle régénère le diff et un nouveau
fichier d'échange, en ne conservant que ce qui est encore en discussion.

## Skill d'agent prête à l'emploi

Le dépôt fournit une skill pour le côté LLM dans
[`skills/leanreview-loop/`](https://github.com/perrito666/leanreview/tree/main/skills/leanreview-loop)
— déposez-la dans le répertoire de skills de votre agent (pour Claude Code :
`.claude/skills/leanreview-loop/`) et demander « une revue avant d'ouvrir la
PR » exécute cette boucle de bout en bout.

## Éditer le fichier directement

Vous n'ouvrez normalement jamais le JSON à la main — la TUI est l'éditeur.
Mais le fichier est conçu pour être lisible : le patch est stocké ligne par
ligne, ce qui le rend clairement diffable dans git, et le dépôt fournit un
[support éditeur](https://github.com/perrito666/leanreview/tree/main/editors)
(Vim/Neovim, VS Code, JetBrains) avec des couleurs de diff et une validation
de schéma pour les fois où vous regardez à l'intérieur.

## Exporter une conversation depuis n'importe quelle revue

N'importe quelle session leanreview peut démarrer une conversation :
`:export topic.review.json` (ou `--export topic.review.json`) écrit le diff
actuel et vos commentaires en brouillon sous forme de fichier d'échange —
remettez-le à un LLM comme retour de revue structuré plutôt que l'export
Markdown plat.
