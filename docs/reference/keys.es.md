# Teclas

Presiona `?` dentro de la TUI para ver esta referencia en cualquier
momento.

![Superposición de ayuda](../screens/help.svg)

## Navegación

| Tecla | Acción |
| --- | --- |
| `j` / `k`, `↓` / `↑` | abajo / arriba una línea (los conteos funcionan: `3j`) |
| `J` / `K` | cambio siguiente / anterior |
| `]c` / `[c` | hunk siguiente / anterior |
| `Tab` / `Shift-Tab`, `]f` / `[f` | archivo siguiente / anterior |
| `gg` / `G` | primera / última línea |
| `Ctrl-d` / `Ctrl-u` | media página |
| `PgDn` / `PgUp` | página completa |
| `h` / `l`, `←` / `→` | desplazar líneas largas (unificado) / lado objetivo (dividido) |
| `0` / `$` | desplazar al inicio / fin de línea |

## Vista

| Tecla | Acción |
| --- | --- |
| `t` | alternar unificado / dividido |
| `T` | alternar contexto de archivo completo alrededor del diff (carga perezosa, con caché; `]c`/`[c` siguen saltando entre hunks) |
| `S` | ciclar el coloreado de sintaxis: cambios en rojo/verde · sintaxis en todo (con tinte) · apagado |
| `w` | alternar ajuste de línea de líneas largas y previsualizaciones de comentarios |
| `i` | alternar previsualizaciones de comentarios en línea |
| `\` | alternar barra lateral de archivos modificados |
| `za` / `zR` / `zM` | plegar hunk actual / expandir todo / colapsar todo |
| `/`, `n`, `N` | buscar texto en el diff, coincidencia siguiente / anterior |
| `f` | selector de archivos |
| `C` | lista de comentarios |
| `Enter` | abrir conversación (línea `●`: editar/borrar respuestas) / hilo (línea `◆`), si no, lista de comentarios |

## Revisión

| Tecla | Acción |
| --- | --- |
| `v` / `V` | seleccionar líneas / bloque modificado |
| `c` | comentar en línea o selección (abre `$EDITOR`) |
| `R` | sugerir un cambio — el editor se abre con el código seleccionado dentro de un bloque ```` ```suggestion ````; los forges lo muestran aplicable |
| `e` | editar comentario en borrador bajo el cursor |
| `x` | descartar / restaurar comentario bajo el cursor (se conserva, nunca se envía) |
| `r` | responder al comentario bajo el cursor ([conversaciones de intercambio](exchange-format.md)) o a su hilo (modo PR) |
| `I` | adjuntar una imagen al borrador bajo el cursor: un prompt de ruta que valida el archivo y permite corregir en caso de error; la imagen se renderiza localmente al momento y se sube al enviar (GitLab — la API de GitHub no tiene endpoint de subida) |
| `dd` | eliminar comentario bajo el cursor |

## Modo pull-request

| Tecla | Acción |
| --- | --- |
| `p` (o `:pr`) | detalles del PR: título, descripción, enlace |
| `P` | conversación general: navegar, responder, añadir (los borradores se publican al enviar) |
| `s` | enviar revisión (pantalla de confirmación) |

## Comandos

| Comando | Acción |
| --- | --- |
| `:w` | guardar borradores |
| `:export FILE` | exportar comentarios (`.json`: [intercambio de revisión](exchange-format.md), si no, Markdown) |
| `:comment` / `:approve` / `:request` | abrir envío con ese evento |
| `:q` / `q` | salir |

## Reasignación

Los enlaces de una sola tecla en modo normal pueden reasignarse mediante
el mapa `keys` en la [configuración](configuration.md). Una acción vacía
desvincula una tecla; las teclas individuales pueden vincularse a acciones
de dos teclas (`next-hunk`, `delete-comment`, …).

Las secuencias de dos teclas (`gg`, `]c`, …) se reasignan mediante la lista
`sequences` — objetos en lugar de nombres concatenados, para que las
secuencias sigan siendo expresables en cualquier distribución de teclado:

```json
"sequences": [
  { "keys": [",", "c"], "action": "next-hunk" },
  { "keys": ["d", "d"], "action": "" }
]
```

Una acción vacía elimina una secuencia. `leanreview --check-config` informa
de solapamientos: un prefijo de secuencia eclipsa un enlace de tecla única
(el prefijo gana), y los dígitos no pueden iniciar un enlace — los prefijos
numéricos de conteo siguen siendo fijos.
