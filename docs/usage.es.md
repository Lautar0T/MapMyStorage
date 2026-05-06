# Guia de uso: MapMyStorage

`MapMyStorage` escanea una ruta y muestra uso real de disco: el espacio fisico asignado localmente, no solo el tamano logico informado por el archivo.

## Instalacion

Descarga un binario desde GitHub Releases o instala con Go:

```sh
go install github.com/lautar0t/MapMyStorage/cmd/mapmystorage@latest
```

Tambien puedes compilar desde fuente:

```sh
make build
./bin/mapmystorage -version
```

## Uso

TUI interactiva:

```sh
mapmystorage
mapmystorage ~/Downloads
```

Salida JSON o CSV:

```sh
mapmystorage -json -root ~/Downloads > disk.json
mapmystorage -csv -root ~/Downloads > disk.csv
```

Resumen no interactivo:

```sh
mapmystorage -no-interactive -root ~/Downloads
```

Filtros:

```sh
mapmystorage -exclude ".git,node_modules,*.tmp" -hidden -max-depth 4
```

## Notas

- `physical_size` es el espacio realmente asignado en disco.
- `logical_size` es el tamano declarado por el archivo.
- La deteccion de placeholders cloud depende de metadatos disponibles en el sistema operativo y en el cliente de sincronizacion.
