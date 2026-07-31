# leanreview

Un cliente de revisión de código para terminal. Revisa un **archivo de
patch/diff**, una **comparación local de git**, un **pull request de
GitHub** o un **merge request de GitLab** en la misma TUI: navega el diff,
adjunta comentarios en borrador anclados a ubicaciones semánticas del diff,
y luego **expórtalos como Markdown** (ideal para retroalimentar a una IA
como feedback de revisión) o **envíalos como una revisión real**.

Es un cliente de revisión, no un cliente de git: el `git`, `gh` y `glab`
instalados se encargan de la semántica del repositorio y del forge;
leanreview se encarga de la navegación, el renderizado, el estado de
revisión y los comentarios.

![La vista principal](screens/main.svg)

*La vista principal: barra lateral de archivos modificados, diff unificado,
un comentario en borrador en su caja en línea (`●`), y el marcador de un
hilo de revisión existente (`◆`).*

## Instalación

```bash
go install github.com/perrito666/leanreview/cmd/leanreview@latest
```

O desde un checkout:

```bash
make            # builds ./leanreview
make install    # installs into GOBIN
```

Dependencias opcionales por modo: [`gh`](https://cli.github.com/) (tras
`gh auth login`) para GitHub, [`glab`](https://gitlab.com/gitlab-org/cli)
(tras `glab auth login`) para GitLab. La revisión de archivos de patch y de
git local no necesita nada más que `git` — y un simple archivo de patch no
necesita ni siquiera eso.

## Inicio rápido

```bash
leanreview change.diff        # review a patch file
leanreview --base main        # review this branch against main
leanreview 418                # review PR/MR 418 of this repo's origin
leanreview --list             # pick something from your review queue
```

## A dónde ir después

- **[Revisar sin un forge](workflows/local.md)** — archivos de patch, stdin,
  árbol de trabajo, cambios en staging, rangos de revisiones, y exportación
  a Markdown.
- **[Revisar un pull request de GitHub](workflows/github.md)** — hilos,
  respuestas, y envío atómico de revisiones a través de `gh`.
- **[Revisar un merge request de GitLab](workflows/gitlab.md)** — el mismo
  flujo a través de `glab`, y cómo el envío se corresponde con el modelo de
  GitLab.
- **[Descubrir qué revisar](workflows/discovery.md)** — `--list`, filtros
  de motor, y filtros con nombre.
- **[Conceptos](concepts.md)** — cómo funcionan los borradores, el anclaje,
  y la reubicación.
- **[Teclas](reference/keys.md)**, **[Línea de comandos](reference/cli.md)**,
  y referencia de **[Configuración](reference/configuration.md)**.
