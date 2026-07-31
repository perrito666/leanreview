# Descubrir qué revisar

`--list` encuentra pull/merge requests abiertos y te deja elegir uno:

```bash
leanreview --list                        # your review queue (default engine + filter)
leanreview --list gh "author:alice"      # explicit engine and search filter
leanreview --list "repo:owner/name"      # filter only; engine from config
leanreview --list | cat                  # piped: prints a plain table instead
```

En una terminal los resultados abren un pequeño selector — `Enter` revisa
la solicitud seleccionada de inmediato, `q` lo cierra:

![Selector de descubrimiento](../screens/picker.svg)

## Filtros

Los filtros son específicos de cada motor:

- **gh** — una consulta de búsqueda de GitHub (por defecto:
  `is:open review-requested:@me`). Los calificadores entre comillas se
  mantienen intactos, así que `--list gh 'label:"needs review"'` funciona
  tal como está escrito.
- **glab** — una cadena de consulta REST (por defecto:
  `state=opened&reviewer_username=@me`, con `@me` resuelto a tu nombre de
  usuario).

## Filtros con nombre

Los filtros pueden **tener nombre** en la
[configuración](../reference/configuration.md) y seleccionarse con
`engine:name` (o `:name` para mantener el motor por defecto); los
argumentos adicionales refinan el filtro con nombre:

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

`list_filter` sigue siendo el valor de respaldo usado cuando `--list` no
recibe ningún filtro. Un primer argumento cuenta como selector solo cuando
empieza con `:` o su prefijo nombra un motor — los calificadores
directos como `author:x` siguen funcionando sin comillas como filtros
simples.
