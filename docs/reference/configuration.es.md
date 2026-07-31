# Configuración

Los ajustes se resuelven en orden creciente de precedencia: valores por
defecto integrados → archivo de configuración → entorno → flags de línea
de comandos. El archivo de configuración vive en
`$XDG_CONFIG_HOME/leanreview/config.json`
(`~/.config/leanreview/config.json`); un archivo mal formado se ignora con
una advertencia impresa al iniciar.

```json
{
  "editor": "nvim -f",
  "syntax": true,
  "syntax_style": "github",
  "theme": "default",
  "tab_width": 4,
  "context": 3,
  "keys": { " ": "down", "m": "next-hunk" },
  "list_engine": "gh",
  "list_filter": "is:open review-requested:@me",
  "list_filters": { "bugs": "is:open label:bug" },
  "wrap": true,
  "wrap_width": 120
}
```

## Generar y validar

Empieza desde una base completa en lugar de un archivo en blanco:

```bash
leanreview --init-config     # escribe el archivo de arriba con todos los
                             # valores por defecto, el mapa de teclas
                             # completo y una referencia $schema
```

El mapa `keys` generado lista **todas** las asignaciones por defecto, así
que reasignar es editar, no adivinar. El generador se niega a sobrescribir
un archivo existente.

Valida en cualquier momento:

```bash
leanreview --check-config    # informa errores de tipeo, acciones
                             # desconocidas y valores inválidos; salida
                             # distinta de cero si hay problemas
```

Los editores también validan: la referencia `$schema` apunta al
[esquema de configuración](../schema/leanreview-config.schema.json)
publicado, dando validación y autocompletado (incluidos los nombres de
acción para `keys`) en VS Code y JetBrains sin configuración — consulta
[`editors/`](https://github.com/perrito666/leanreview/tree/main/editors)
para Vim/Neovim.

## Ajustes

- `editor` — comando del editor (interpretado como línea de comando, p.
  ej. `code --wait`).
- `syntax` / `syntax_style` — habilita el resaltado y elige un estilo de
  Chroma. El valor por defecto, `auto`, coincide con el fondo de tu
  terminal (`monokai` en oscuro, `github` en claro); se puede establecer
  explícitamente cualquier
  [nombre de estilo de Chroma](https://xyproto.github.io/splash/docs/).
- `theme` — paleta de la TUI: `default` o `mono`.
- `tab_width` — columnas a las que se expande un tab.
- `context` — líneas de contexto unificado por defecto cuando no se pasa
  `-U`.
- `keys` — reasigna los enlaces de modo normal, `{ "<key>": "<action>" }`.
  Ver [Teclas](keys.md#remapping).
- `list_engine` / `list_filter` / `list_filters` — valores por defecto
  para `--list`: el motor de descubrimiento (`gh` o `glab`), el filtro de
  búsqueda de respaldo, y un mapa de filtros con nombre seleccionables
  como `--list :name` / `--list engine:name`.
- `images` — renderizado de imágenes en comentarios: `auto` (protocolo
  kitty en kitty/ghostty, `chafa` si está instalado, desactivado en otro
  caso), `kitty`, `chafa` u `off`. Las URLs remotas nunca se descargan; se
  muestran como etiquetas.
- `author` — el nombre con el que se atribuyen tus respuestas en las
  [conversaciones de intercambio de revisión](exchange-format.md)
  (por defecto: `$USER`).
- `wrap` / `wrap_width` — ajuste de línea de líneas largas del diff y
  previsualizaciones de comentarios (activado por defecto, `w` alterna).
  El código hace ajuste de línea duro en el borde de la columna, los
  comentarios en límites de palabra; en diseño unificado el punto de
  ajuste es `min(wrap_width, view)` (120 por defecto), en diseño dividido
  es el ancho del panel lateral. Con el ajuste de línea desactivado, las
  líneas largas se recortan y `h`/`l` las desplazan.

## Entorno

| Variable | Efecto |
| --- | --- |
| `LEANREVIEW_EDITOR` | comando del editor (anula el archivo de configuración) |
| `LEANREVIEW_SYNTAX=0` | deshabilita el resaltado de sintaxis |
| `NO_COLOR` | deshabilita todo el color (tema mono) |
| `LEANREVIEW_AUTHOR` | nombre de atribución de respuestas (anula `author`) |
| `LEANREVIEW_IMAGES` | backend de renderizado de imágenes (anula `images`) |
| `LEANREVIEW_LOG` | ruta del archivo de log |

## Estado en disco

- Borradores: `$XDG_STATE_HOME/leanreview/drafts/` (`~/.local/state/…`),
  un archivo JSON por fuente, escrituras atómicas.
- Logs: `$XDG_STATE_HOME/leanreview/leanreview.log` — nunca stdout, que
  pertenece a la TUI.
