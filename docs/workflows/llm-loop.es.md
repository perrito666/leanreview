# Revisar con un LLM

leanreview puede ser la mitad humana de una **conversación de revisión
offline** con un LLM: el modelo revisa un diff y escribe sus comentarios en
un único archivo autocontenido, tú los evalúas en la TUI, y el modelo lee
tu veredicto de vuelta y actúa en consecuencia. Una configuración típica:
antes de enviar un PR, un LLM realiza una primera pasada de revisión; un
humano descarta o refina esos comentarios; el LLM usa el resultado para
mejorar el cambio.

El medio es el [formato de intercambio de revisión (review exchange)
](../reference/exchange-format.md) — un documento JSON (`*.review.json`)
que lleva el diff y sus comentarios.

## El bucle

**1. El LLM escribe la revisión.** Calcula el diff del cambio, escribe
`topic.review.json` con sus comentarios anclados a líneas del diff, y
valida sus propios anclajes de forma no interactiva:

```bash
leanreview topic.review.json --export /tmp/check.md   # orphaned = bad anchor
```

**2. Tú evalúas en la TUI.**

```bash
leanreview topic.review.json
```

Los comentarios del modelo aparecen enmarcados en línea bajo las líneas del
diff que comentan, atribuidos (`@assistant`). Luego:

- `x` — **descarta** un comentario con el que no estés de acuerdo.
  Permanece en el archivo con `state: "dismissed"` para que el modelo sepa
  que no debe volver a plantearlo; `x` de nuevo lo restaura.
- `r` — **responde** a un comentario: contesta una pregunta, explica un
  descarte o da indicaciones sin reescribir las palabras del modelo. Las
  respuestas se atribuyen a ti (`author` en la configuración,
  `LEANREVIEW_AUTHOR` o `$USER`) y se muestran dentro de la caja del
  comentario.
- `e` — edita el cuerpo de un comentario para refinarlo o corregirlo.
- `c` — añade tus propios comentarios, igual que en cualquier revisión.
- `dd` — elimina definitivamente (no deja rastro en la conversación).
- `Enter` — abre el **lector de conversación** del comentario bajo el
  cursor: selecciona el comentario o cualquier respuesta individual
  (`j`/`k`) para editarla (`e`) o borrarla (`d`) — toda la conversación es
  editable, no solo tu lado.

Cada cambio **reescribe el archivo en su lugar** — sal cuando termines; no
hay paso de exportación.

**3. El LLM actúa según tu veredicto.** Los comentarios descartados se
eliminan (y nunca se vuelven a plantear), los cuerpos editados son la nueva
instrucción, y tus propios comentarios añadidos tienen el mayor peso. Para
otra ronda, el modelo regenera el diff y un nuevo archivo de intercambio,
arrastrando solo lo que sigue en discusión.

## Habilidad de agente lista para usar

El repositorio incluye una habilidad para el lado del LLM en
[`skills/leanreview-loop/`](https://github.com/perrito666/leanreview/tree/main/skills/leanreview-loop)
— colócala en el directorio de habilidades de tu agente (para Claude Code:
`.claude/skills/leanreview-loop/`) y pedir "una revisión antes de abrir el
PR" ejecuta este bucle de principio a fin.

## Editar el archivo directamente

Normalmente nunca abres el JSON a mano — la TUI es el editor. Pero el
archivo está diseñado para ser legible: el patch se guarda línea por línea,
así que produce diffs limpios en git y el repositorio incluye [soporte de
editor](https://github.com/perrito666/leanreview/tree/main/editors)
(Vim/Neovim, VS Code, JetBrains) con colores de diff y validación de
esquema para cuando sí mires dentro.

## Exportar una conversación desde cualquier revisión

Cualquier sesión de leanreview puede iniciar una conversación:
`:export topic.review.json` (o `--export topic.review.json`) escribe el
diff actual y tus comentarios en borrador como un archivo de intercambio —
entrégaselo a un LLM como feedback de revisión estructurado en lugar de la
exportación plana a Markdown.
