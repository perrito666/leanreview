# Revisar un merge request de GitLab

Requiere [`glab`](https://gitlab.com/gitlab-org/cli) autenticado con
`glab auth login`. El adaptador se elige según el host, así que la TUI es
idéntica al flujo de GitHub — la insignia de la barra de título dice
`glab`.

## Abrir un merge request

```bash
leanreview 'https://gitlab.com/group/repo/-/merge_requests/42'
leanreview 'group/repo!42'                # nested subgroups work too
leanreview 42                             # inside a checkout with a GitLab origin
```

El endpoint de cambios de GitLab devuelve hunks por archivo; leanreview
reconstruye el patch unificado canónico a partir de ellos, de modo que las
posiciones de los comentarios coinciden con lo que muestra GitLab.

Todo lo del [flujo de GitHub](github.md) aplica: la superposición de
detalles con `p`, los marcadores de hilo `◆`, `Enter` para leer un hilo,
`r` para dejar una respuesta en borrador, `C` para la lista de comentarios.

## Cómo se corresponde el envío con GitLab

GitLab no tiene un endpoint de revisión atómica con comentarios, así que
`s` corresponde el envío con el modelo de GitLab:

- cada comentario de línea en borrador se convierte en una **discusión de
  diff posicionada**;
- el resumen de la revisión se convierte en una **nota** del merge
  request;
- **Aprobar** aprueba el MR;
- **Solicitar cambios** publica una nota de "Changes requested".

Los comentarios se publican en orden, y un fallo a mitad de camino informa
cuántos ya se publicaron — esos se eliminan de tus borradores para que un
reintento no pueda volver a publicarlos.
