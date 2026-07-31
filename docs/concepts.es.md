# Conceptos

## Anclaje semántico

La regla de diseño central: **las filas de pantalla renderizadas nunca son
ubicaciones canónicas de comentarios**. Un comentario se ancla a una
ubicación semántica — ruta, lado (antiguo/nuevo), rango de líneas, y las
líneas de contexto circundantes — no a una coordenada de terminal. Ese
anclaje sobrevive al redimensionado, al plegado de hunks, al ajuste de
línea, y a los cambios entre unificado ↔ dividido, porque esos solo cambian
la *proyección* del diff, no el diff en sí.

## Borradores

Todo comentario empieza como un **borrador**, guardado localmente y nunca
enviado a ningún lado hasta que lo envías explícitamente. Los borradores
viven en `$XDG_STATE_HOME/leanreview/drafts/` (`~/.local/state/…`), un
archivo JSON por fuente, escrito de forma atómica.

Cada fuente de revisión tiene una **clave de identidad estable** — un hash
del contenido del patch para archivos/stdin, la especificación del diff
para comparaciones de git, host/repo/número para PRs y MRs — de modo que
reabrir la misma fuente recarga sus borradores, y dos fuentes distintas
nunca comparten estado. `--discard` elimina el borrador guardado de una
fuente.

## Reubicación y huérfanos

En modo pull-request el diff puede cambiar debajo de tus borradores
guardados: alguien sube nuevos commits entre tus sesiones. Al cargar,
leanreview re-ancla cada borrador comparando su contexto circundante
capturado contra el nuevo diff (siguiendo renombrados de archivos), y
reubica un comentario **solo cuando hay una coincidencia única** — un
contexto que coincide dos veces es ambiguo, y adivinar pondría tu nota en
la línea equivocada.

Los comentarios que no se pueden ubicar quedan marcados como **huérfanos**:
permanecen en tu lista de borradores, se muestran con una etiqueta
`[orphaned]`, quedan excluidos del envío (la pantalla de confirmación
advierte sobre ellos), y esperan a que los reubiques o los elimines. Nada
se descarta silenciosamente.

## Envío

El envío es explícito y atómico donde el host lo permite:

- **GitHub** — todos los comentarios de línea se suben como una sola
  revisión con el evento elegido (comentario / aprobar / solicitar
  cambios); las respuestas de hilo en borrador se publican después.
- **GitLab** — no tiene un endpoint de revisión atómica, así que los
  comentarios se convierten en discusiones de diff posicionadas y
  publicadas en orden, y el evento se corresponde con una aprobación o una
  nota.

En cualquier caso, todo lo que el host haya aceptado se elimina de
inmediato de tus borradores — un reintento tras un fallo parcial envía solo
lo que aún está pendiente, nunca un duplicado.

## El editor

`c`, `e`, y `r` abren tu editor, resuelto en este orden: `editor` en el
archivo de configuración / `LEANREVIEW_EDITOR` → `GIT_EDITOR` → `VISUAL` →
`EDITOR` → `git var GIT_EDITOR` → `vi`. Los valores se interpretan como
líneas de comando (`code --wait`, `nvim -f`). El buffer es un archivo
Markdown con un encabezado en comentario HTML que lleva el contexto; el
encabezado se elimina automáticamente, y guardar un cuerpo vacío descarta
el comentario.
