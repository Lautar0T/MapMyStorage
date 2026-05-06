# Guia de uso: real-disk-map

`real-disk-map` escanea una ruta y muestra uso real de disco: el espacio fisico asignado localmente, no solo el tamano logico informado por el archivo.

## Instalacion

Descarga un binario desde GitHub Releases o instala con Go:

```sh
go install github.com/lautaro/real-disk-map/cmd/rdm-cli@latest
```

Tambien puedes compilar desde fuente:

```sh
make build
./bin/rdm-cli -version
```

## Uso

TUI interactiva:

```sh
rdm-cli
rdm-cli ~/Downloads
```

Salida JSON o CSV:

```sh
rdm-cli -json -root ~/Downloads > disk.json
rdm-cli -csv -root ~/Downloads > disk.csv
```

Resumen no interactivo:

```sh
rdm-cli -no-interactive -root ~/Downloads
```

Filtros:

```sh
rdm-cli -exclude ".git,node_modules,*.tmp" -hidden -max-depth 4
```

## Notas

- `physical_size` es el espacio realmente asignado en disco.
- `logical_size` es el tamano declarado por el archivo.
- La deteccion de placeholders cloud depende de metadatos disponibles en el sistema operativo y en el cliente de sincronizacion.
