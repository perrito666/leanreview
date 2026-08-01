# Revisar un pull request de GitHub

Requiere [`gh`](https://cli.github.com/) autenticado con `gh auth login`.
Los hosts Enterprise funcionan a través de la propia autenticación de `gh`.

## Abrir un pull request

```bash
leanreview 418                                   # infers owner/repo from origin
leanreview owner/repo#418
leanreview https://github.com/owner/repo/pull/418
```

leanreview obtiene el **diff canónico** del PR a través de `gh` — la
representación exacta a la que GitHub ancla los comentarios de revisión,
de modo que las posiciones siempre coinciden con lo que muestra GitHub —
más los metadatos del PR y sus hilos de revisión existentes.

La primera línea de la barra de título muestra una insignia `gh`, la
referencia del PR, y su título.

## La superposición del pull request

Presiona `p` (o `:pr`) para ver los detalles del pull request: título,
autor, ramas, URL, y la descripción renderizada como Markdown con estilo.
`j`/`k` desplazan, `esc`/`p` cierra.

![Superposición de detalles del PR](../screens/pr-overlay.svg)

Debajo de la descripción, la superposición lista la **conversación** del
PR — los comentarios generales anclados a la solicitud y no a una línea —
del más antiguo al más reciente, con autores y fechas. Las imágenes
adjuntas a la descripción o a los comentarios de conversación se
renderizan directamente en la superposición.

![La conversación en la superposición, con su imagen renderizada](../screens/pr-overlay-kitty.png)

*Un comentario general de conversación con una foto, dibujada en la
superposición en ghostty mediante el protocolo gráfico de kitty.*

## La conversación general

`P` abre la conversación como pantalla propia: `j`/`k` seleccionan un
comentario, `r`/`Enter` responde, `a` añade un comentario general nuevo.
Las conversaciones de ambos forges son planas, así que una respuesta es un
comentario nuevo pre-rellenado con una cita Markdown de lo que responde.
Todo lo que escribas queda como borrador — marcado *(draft — posts on
submit)*, editable con `e` y eliminable con `d` — y se publica al enviar
la revisión.

En la pantalla de envío, `g` escribe el **resumen de la revisión** — el
comentario general adjunto a la revisión misma (el cuerpo de la review en
GitHub, la nota inicial en GitLab).

!!! note "Imágenes adjuntas"
    `I` adjunta una imagen local a un borrador, y se renderiza en la TUI de
    inmediato — pero la API REST de GitHub no tiene endpoint de subida de
    adjuntos, así que el envío en GitHub rechaza borradores con imágenes
    locales: elimínalas o referencia una URL ya alojada. En GitLab se suben
    automáticamente ([ver el flujo de GitLab](gitlab.md#adjuntar-imagenes)).

## Hilos y respuestas

Los hilos de revisión existentes aparecen como marcadores `◆` en la
columna, con el comentario raíz previsualizado en línea. En una línea
marcada:

- `Enter` abre el hilo completo (raíz, respuestas, indicadores de
  resuelto/desactualizado).
- `r` redacta una respuesta en tu editor. Las respuestas quedan en
  borrador y se publican cuando envías.

`C` lista tus borradores y, debajo de ellos, todos los hilos existentes.

## Enviar

`s` (o `:comment` / `:approve` / `:request`) abre la pantalla de envío:

![Confirmación de envío](../screens/submit.svg)

Elige el evento con `c`/`a`/`R`, luego `y` envía. Todos los comentarios de
línea en borrador se suben como **una revisión atómica**; las respuestas en
borrador se publican en sus hilos después. Nada se envía nunca antes de
esta confirmación.

Si una respuesta falla después de que se creó la revisión, todo lo que
GitHub ya haya aceptado se elimina de tus borradores — reintentar solo
envía lo que aún está pendiente, nunca una revisión duplicada.

## Cuando el HEAD del PR se mueve

Si aterrizan nuevos commits entre tus sesiones, los borradores guardados se
re-anclan al nuevo diff comparando el contexto circundante capturado de
cada comentario (siguiendo renombrados), reubicando solo cuando hay una
coincidencia única. Los comentarios que no se pueden ubicar quedan
marcados como **huérfanos**, excluidos del envío, y se conservan para que
los reubiques — ver [Conceptos](../concepts.md).
