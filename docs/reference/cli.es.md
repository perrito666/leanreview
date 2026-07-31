# Línea de comandos

```text
leanreview [flags] [target]        (the "review" verb is also accepted)
```

## Objetivos

| Objetivo | Significado |
| --- | --- |
| `<file.diff>` / `-` | un archivo de patch / stdin |
| `.` (o nada) | árbol de trabajo vs HEAD |
| `<revA> <revB>` | rango de revisiones explícito de git |
| `418`, `#418`, `!42` | número de PR/MR (owner/repo se infiere del origin) |
| `owner/repo#418`, `group/repo!42`, URL completa | referencia explícita de PR/MR |

## Flags

| Flag | Significado |
| --- | --- |
| `--base <ref>` | comparar `<ref>...HEAD` (merge-base) en lugar del árbol de trabajo |
| `--staged` | comparar el índice contra HEAD |
| `-U, --context N` | líneas de contexto unificado (3 por defecto, configurable) |
| `--export FILE` | escribir comentarios en borrador como Markdown y salir |
| `--discard` | eliminar el borrador guardado de esta fuente y salir |
| `--list [engine] [filter]` | descubrir PRs/MRs abiertos y elegir uno para revisar (tabla plana cuando se canaliza) |
| `-h, --help` | mostrar ayuda |
| `-v, --version` | imprimir la versión y salir |

Ver [Descubrir qué revisar](../workflows/discovery.md) para la sintaxis
completa del selector de `--list` (motores, filtros con nombre,
refinamientos).
