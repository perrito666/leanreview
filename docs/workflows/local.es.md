# Revisar sin un forge

Este es el flujo de trabajo fundamental de leanreview: sin host, sin
cuenta, sin red — solo un diff y tus notas. Todo lo que los modos de forge
añaden (hilos, envío) se construye sobre esto, así que todo lo que aparece
aquí también aplica a esos modos.

## Abrir un diff

Cualquiera de estos te deja en la misma TUI de revisión:

```bash
leanreview change.diff          # a patch file
git diff | leanreview -         # stdin
leanreview .                    # working tree vs HEAD (also: no argument)
leanreview --staged             # index vs HEAD
leanreview --base main          # main...HEAD (merge-base comparison)
leanreview HEAD~3 HEAD          # explicit revision range
```

`-U`/`--context` controla el número de líneas de contexto unificado
(3 por defecto, configurable).

## Navegar

`j`/`k` mueven línea a línea, `J`/`K` saltan entre cambios, `]c`/`[c` entre
hunks, y `Tab`/`Shift-Tab` (o `]f`/`[f`) entre archivos. `t` alterna entre
diseño unificado ↔ dividido, `\` alterna la barra lateral de archivos
modificados, `za` pliega un hunk, `/` busca texto en el diff.

La barra de título siempre muestra el título de la revisión en su primera
línea y el archivo actual, la posición, y el diseño en la segunda.

![Diseño dividido](../screens/split.svg)

*Diseño dividido: eliminaciones estilizadas en el panel izquierdo,
adiciones en el derecho, y un comentario en borrador enmarcado sobre el
panel derecho.*

## Comentar

Presiona `c` en una línea — o `v` para seleccionar un rango (`V` toma todo
el bloque modificado), luego `c`. Tu `$EDITOR` se abre con una plantilla
Markdown; escribe la nota, guarda, sal. El comentario se convierte en un
**borrador**: la línea recibe un `●` en la columna izquierda y la nota se
previsualiza en línea en una caja con borde justo debajo (`i` oculta/muestra
las previsualizaciones). `e` lo edita, `dd` lo elimina.

En diseño dividido, `h`/`l` eligen sobre qué lado de un cambio emparejado
estás comentando — la barra de estado muestra `[LEFT]`/`[RIGHT]`.

Una selección debe corresponder a un rango continuo en un solo lado; las
selecciones que cruzan lados se rechazan antes de que se abra el editor.

## Revisar tus notas

`C` lista todos los borradores; `Enter` salta a uno, `e` edita, `d` elimina.

![Lista de comentarios](../screens/comments.svg)

## Exportar

`:export notes.md` — o de forma no interactiva `leanreview --export
notes.md change.diff` — escribe Markdown agrupado por archivo, con el
fragmento comentado citado encima de cada nota:

````markdown
## internal/api/handler.go

### L72 (RIGHT)
```go
result, err := calculate(input)
```
> This still ignores the error from calculate().
````

Esta salida está diseñada para pegarse directamente de vuelta en un prompt
de IA como feedback de revisión, o en un correo de revisión de texto plano.

## Persistencia de borradores

Los borradores se guardan automáticamente (`:w` fuerza un guardado; salir
también guarda) y se recargan la próxima vez que abras la **misma fuente**
— cada fuente tiene una clave de identidad estable (ver
[Conceptos](../concepts.md)). `leanreview --discard <target>` elimina el
borrador guardado de una fuente.
