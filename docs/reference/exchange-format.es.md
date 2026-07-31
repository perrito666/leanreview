# Formato de intercambio de revisión

El intercambio de revisión (review exchange) es el formato de archivo de
leanreview para **conversaciones de revisión offline**: un único documento
JSON que lleva un diff unificado y los comentarios sobre él, pasado de un
lado a otro entre herramientas. El bucle canónico es un LLM que escribe
una revisión, un humano que la edita en la TUI de leanreview, y el LLM que
lee el resultado de vuelta — pero cualquier par de partes que hablen el
formato pueden mantener la conversación.

- **Tipo de medio**: JSON (UTF-8). Nombre de archivo recomendado:
  `*.review.json`.
- **Versión**: `1` (esta página). Se publica un
  [JSON Schema](../schema/leanreview-review.schema.json) legible por
  máquina para validación y herramientas de editor.
- **Detección**: por contenido, no por nombre de archivo — la presencia de
  la clave `"leanreview_review"` cerca del inicio de un objeto JSON.
  `leanreview <file>` abre cualquier archivo que se detecte como un
  intercambio en modo conversación.

## Ejemplo

```json
{
  "leanreview_review": 1,
  "title": "auth refactor",
  "summary": "Two issues, one blocking.",
  "patch": [
    "diff --git a/internal/api/handler.go b/internal/api/handler.go",
    "--- a/internal/api/handler.go",
    "+++ b/internal/api/handler.go",
    "@@ -70,5 +70,6 @@ func Handle(input Input) (Output, error) {",
    "     ctx := context.Background()",
    "     defer cancel(ctx)",
    "-    result := calculate(input)",
    "+    result, err := calculate(input)",
    "+    _ = err",
    "     return result, nil"
  ],
  "comments": [
    {
      "id": "c1",
      "author": "assistant",
      "path": "internal/api/handler.go",
      "side": "RIGHT",
      "start_line": 72,
      "body": "`err` is assigned and discarded — return it instead.",
      "state": "active",
      "snippet": "result, err := calculate(input)",
      "replies": [
        { "author": "hduran", "body": "Agreed, but only log it: callers can't handle it." }
      ]
    }
  ]
}
```

## Campos del documento

| Campo | Requerido | Significado |
| --- | --- | --- |
| `leanreview_review` | sí | Versión del formato, entero. Quienes lean **deben** rechazar versiones desconocidas. Debe mantenerse como la primera clave: también sirve como marcador de detección. |
| `title` | no | Título de la revisión legible por humanos (se muestra en la barra de título de la TUI). |
| `summary` | no | Resumen/veredicto a nivel de revisión; sobrevive la ida y vuelta hasta el resumen de envío. |
| `patch` | sí | El diff unificado al que se anclan los comentarios (ver más abajo). |
| `comments` | sí | La conversación; puede estar vacía. |

Los campos desconocidos en cualquier parte del documento se **toleran**:
quienes lean los ignoran, así que la evolución aditiva ocurre sin necesidad
de subir de versión — pero los intermediarios (incluida la reescritura de
leanreview) no conservan campos que no conocen, así que quien escribe no
debe depender de que campos adicionales sobrevivan una ida y vuelta. Los
cambios incompatibles suben `leanreview_review`.

## El patch

El patch es el diff unificado completo (estilo git) sobre el que trata la
conversación. Incluirlo directamente — en lugar de referenciar un commit —
es lo que hace que el archivo sea autocontenido y que la conversación sea
posible offline.

Quienes escriben **deben** emitirlo como un **array JSON de líneas**, una
línea del diff por elemento, sin un elemento vacío final (el salto de
línea final es implícito). Quienes leen **deben** también aceptar una
única cadena (con `\n` incrustados) por tolerancia hacia escritores hechos
a mano.

La forma de array es deliberada: mantiene el archivo legible para humanos,
hace que las idas y vueltas produzcan diffs de texto significativos a
nivel de línea (el propio archivo de conversación suele guardarse en
git), y permite que los editores resalten el diff incrustado — ver
[soporte de editor](#editor-support).

## Comentarios

| Campo | Requerido | Significado |
| --- | --- | --- |
| `id` | recomendado | Identidad estable a través de las idas y vueltas. leanreview conserva los ids y asigna unos aleatorios cuando faltan. |
| `author` | no | Atribución de forma libre (`"assistant"`, un nombre de usuario). Se muestra en la TUI. |
| `path` | sí | Ruta del archivo tal como aparece en el patch. |
| `side` | sí | `LEFT` (imagen antigua: líneas eliminadas) o `RIGHT` (imagen nueva: líneas añadidas/de contexto). No distingue mayúsculas/minúsculas. |
| `start_line` | sí | Línea basada en 1 en esa imagen del patch. |
| `end_line` | no | Fin del rango multilínea inclusive; por defecto es `start_line`. |
| `body` | sí | Texto del comentario en Markdown. |
| `state` | no | `active` (por defecto), `dismissed`, `orphaned`, o `stale`. |
| `snippet` | no | La(s) línea(s) del diff ancladas, informativo. Se recalcula cuando está vacío. |
| `at` | no | Marca de tiempo de creación en RFC 3339. |
| `replies` | no | Respuestas de seguimiento, de más antigua a más reciente: `{ "author", "body", "at" }` (`at` opcional). |

Las marcas de tiempo son opcionales pero forman parte de la versión 1 a
propósito: los intermediarios no conservan campos desconocidos, así que todo
lo que un viaje de ida y vuelta deba transportar tiene que existir desde el
principio. Las respuestas se acumulan entre rondas, y su cronología es parte
de la conversación.

### Estados — el protocolo de la conversación

`state` es cómo el veredicto del humano viaja de vuelta hacia quien
escribió la revisión:

- **`active`** — el comentario se mantiene en pie. El siguiente actor
  debería atenderlo.
- **`dismissed`** — un humano lo rechazó (descartado). Se conserva en el
  archivo (el veredicto en sí es información) pero **no** debe ser
  atendido ni reenviado. En la TUI, `x` alterna el descarte.
- **`orphaned`** — el anclaje ya no se resuelve contra el patch (o el
  siguiente diff). leanreview establece esto al importar cuando
  `(path, side, start_line)` no existe en el patch incrustado; los
  comentarios huérfanos nunca se envían.
- **`stale`** — el anclaje puede haberse movido; una pasada de reanclaje
  está pendiente.

Un comentario `dismissed` conserva su estado incluso cuando ya no ancla:
la decisión humana prevalece sobre el fallo de anclaje.

### Reglas de anclaje

Los números de línea se interpretan contra el **patch incrustado**, con la
misma semántica que los comentarios de revisión de un forge: `side` elige
la imagen previa/posterior, y `start_line` es el número de línea basado en
1 en esa imagen. Al importar, leanreview resuelve cada comentario contra
el patch analizado:

- resoluble → el comentario obtiene un anclaje de contexto (líneas
  circundantes), de modo que puede sobrevivir a revisiones posteriores del
  diff mediante coincidencia de contenido;
- no resoluble → el comentario se importa como `orphaned` en lugar de
  descartarse o adivinarse: un desfase de un generador es una condición
  visible y corregible, nunca una pérdida silenciosa de datos.

Quienes escriben deberían verificar los números de línea contra el patch
que incrustan; el campo `snippet` existe para que quienes lean (humano o
LLM) puedan detectar discrepancias de un vistazo.

## Garantías de ida y vuelta

Cuando leanreview abre un archivo de intercambio, las ediciones ocurren en
la TUI y **cada guardado de borrador reescribe el archivo en su lugar**
(salir también guarda). Lo siguiente sobrevive intacto a una ida y vuelta:
los `id` de comentario, los `author`, las marcas `at`, las `replies`, el `patch`, el
`title`, y el `summary`. La TUI cambia solo lo que el humano cambió:
cuerpos, estados, y el conjunto de comentarios (los añadidos/eliminados).

La salida es determinista: indentación de dos espacios, orden estable de
claves, una línea de diff por elemento del patch, salto de línea final.
Idas y vueltas sucesivas de una revisión sin cambios son idénticas byte a
byte.

## Notas de diseño

El formato pasó por una iteración deliberada antes de fijarse:

1. **JSON en lugar de un formato de texto a medida.** Se consideró un
   formato entrelazado de "diff con bloques de comentarios" — se lee bien
   y ancla los comentarios implícitamente. Perdió porque quienes
   principalmente escriben y leen son programas (los LLM emiten JSON
   válido de forma mucho más fiable que una sintaxis a medida), el humano
   edita en la TUI en lugar de un editor de texto, y un parser a medida es
   una fuente interminable de errores de escapado.
2. **Patch como array de líneas, no como cadena.** El primer borrador
   incrustaba el patch como una única cadena JSON; un diff lleno de
   escapes `\n` en una sola línea es ilegible, no diffable, y no
   resaltable. La forma de array resuelve los tres problemas sin coste de
   análisis adicional, manteniendo la tolerancia de cadena para escritores
   mínimos.
3. **Estados en lugar de eliminación.** Los comentarios descartados
   permanecen en el archivo porque el rechazo es justo lo que el otro lado
   de la conversación más necesita saber.

## Soporte de editor

El directorio [`editors/`](https://github.com/perrito666/leanreview/tree/main/editors)
del repositorio incluye soporte listo para usar:

- **Vim / Neovim** — un filetype (`lreview`) para `*.review.json` que
  superpone colores de diff (líneas añadidas/eliminadas/de hunk dentro del
  array `patch`) sobre el resaltado de JSON.
- **VS Code** — una extensión mínima con la misma gramática en capas, más
  una asociación de JSON Schema para validación y autocompletado.
- **IDEs de JetBrains** — un mapeo de JSON Schema (validación +
  autocompletado) a través de la URL del schema publicado; la gramática
  TextMate de la extensión de VS Code puede importarse para obtener los
  colores de diff.

Todos ellos dependen de la convención de nombres `*.review.json` — otra
razón para seguirla.
