# Découvrir quoi revoir

`--list` recherche les pull/merge requests ouvertes et permet d'en choisir
une :

```bash
leanreview --list                        # your review queue (default engine + filter)
leanreview --list gh "author:alice"      # explicit engine and search filter
leanreview --list "repo:owner/name"      # filter only; engine from config
leanreview --list | cat                  # piped: prints a plain table instead
```

Dans un terminal, les résultats ouvrent un petit sélecteur — `Enter` revoit
immédiatement la requête sélectionnée, `q` ferme :

![Discovery picker](../screens/picker.svg)

## Filtres

Les filtres sont spécifiques à chaque moteur :

- **gh** — une requête de recherche GitHub (par défaut :
  `is:open review-requested:@me`). Les qualificateurs entre guillemets sont
  conservés tels quels, donc `--list gh 'label:"needs review"'` fonctionne
  comme écrit.
- **glab** — une chaîne de requête REST (par défaut :
  `state=opened&reviewer_username=@me`, avec `@me` résolu vers votre nom
  d'utilisateur).

## Filtres nommés

Les filtres peuvent être **nommés** dans la
[configuration](../reference/configuration.md) et sélectionnés avec
`engine:name` (ou `:name` pour conserver le moteur par défaut) ; des
arguments supplémentaires affinent le filtre nommé :

```bash
leanreview --list :bugs                  # named filter, default engine
leanreview --list gh:bugs                # named filter, explicit engine
leanreview --list :bugs "base:main"      # named filter + extra qualifiers
```

```json
{
  "list_engine": "gh",
  "list_filter": "is:open review-requested:@me",
  "list_filters": {
    "bugs": "is:open label:bug",
    "team": "is:open team-review-requested:mycorp/reviewers"
  }
}
```

`list_filter` reste le filtre de secours utilisé quand `--list` ne reçoit
aucun filtre du tout. Un premier argument ne compte comme sélecteur que
s'il commence par `:` ou si son préfixe nomme un moteur — les
qualificateurs bruts comme `author:x` fonctionnent toujours sans guillemets
en tant que simples filtres.
