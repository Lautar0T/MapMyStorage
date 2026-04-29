# real-disk-map

`real-disk-map` mapea un directorio raíz (por defecto tu home) y muestra **uso real en disco** (`allocated size`, `size on disk`), no tamaño lógico.

Incluye:
- CLI interactivo cross-platform (macOS + Windows)
- extensión Raycast para macOS reutilizando el mismo core

## Objetivo clave

La métrica primaria es siempre el tamaño físico asignado localmente:
- placeholders online-only (Dropbox/OneDrive/iCloud, etc.) aparecen con `0 B` o mínimo físico real
- el tamaño lógico se muestra como columna secundaria comparativa

## Arquitectura

```text
real-disk-map/
├── core/
│   ├── disk/      # cálculo de tamaño real + escáner
│   ├── export/    # JSON/CSV
│   ├── models/    # tipos compartidos
│   └── cache/     # infraestructura de cache
├── cmd/rdm-cli/   # CLI + TUI (Bubble Tea)
└── raycast-extension/
    └── src/       # frontend Raycast (TypeScript)
```

El frontend de Raycast **no duplica lógica de escaneo**: invoca `rdm-cli -json` y consume la salida del core.

## Cómo se calcula “real disk usage”

### macOS

- `Lstat` para no seguir symlinks por defecto
- tamaño físico desde `st_blocks * 512`
- soporte para sparse files y archivos comprimidos
- detección cloud vía xattrs (`com.apple.icloud.*`, `com.dropbox.*`, etc.)

### Windows

- `GetCompressedFileSizeW` para tamaño asignado real
- ajuste por cluster con `GetDiskFreeSpaceW`
- detección placeholder/local vía atributos (`OFFLINE`, `RECALL_ON_*`, `PINNED`, etc.)

## Comportamiento del escáner

- carpetas: suma recursiva de tamaño físico real de hijos
- symlinks: no se siguen por defecto (`-follow-symlinks` opcional)
- errores de permisos: se contabilizan y el escaneo continúa
- exclusiones: por patrón (`-exclude "*.log,node_modules"`)
- profundidad máxima opcional (`-max-depth`)
- salida reutilizable JSON/CSV

## CLI (rdm-cli)

### Instalación rápida

```bash
make build
```

binario en `bin/rdm-cli`.

Para instalar en macOS/Linux en PATH:

```bash
make install
```

### Uso

```bash
# TUI interactivo
rdm-cli
rdm-cli /ruta/al/root

# JSON / CSV
rdm-cli -json -root ~ > disk.json
rdm-cli -csv -root ~ > disk.csv

# filtros
rdm-cli -exclude ".git,node_modules,*.tmp" -hidden -max-depth 4
```

### Atajos TUI

- `↑/↓` o `k/j`: mover cursor
- `Enter`: entrar carpeta / reveal archivo
- `Backspace` o `←`: subir nivel
- `r`: refrescar
- `s`: cambiar criterio de orden
- `/`: buscar
- `h`: ocultar/mostrar hidden
- `l`: alternar foco real/lógico
- `v`: panel visual por niveles (resumen tipo treemap ASCII)
- `o`: abrir/revelar en Finder/Explorer
- `t`: abrir en terminal
- `y`: copiar path
- `e`: exportar JSON/CSV
- `q`: salir

## Raycast Extension (macOS)

### Setup

```bash
cd raycast-extension
npm install
npm run build
```

Luego importar carpeta `raycast-extension` en Raycast.

### Features

- listado de archivos/carpetas más pesados por tamaño real
- drill-down folder-by-folder
- búsqueda rápida
- cache en memoria por path + refresh manual
- acciones:
  - Open in Finder
  - Open in Terminal
  - Reveal File in Finder
  - Copy Path
  - Refresh
  - Change Root

### Preferencias

- root path por defecto
- mostrar ocultos
- excluir patrones
- follow symlinks (default `false`)
- max depth opcional

## Desarrollo

Prerequisitos:
- Go 1.22+
- Node.js 18+
- Make

Comandos útiles:

```bash
make build
make build-all
make test
```

Build cross-platform (incluye Windows):

```bash
make windows
make darwin
```

## Tests incluidos

- unit tests de escáner y exclusiones
- test de symlink (no follow por defecto)
- test de sparse file (verifica `physical < logical`)
- test de inferencia de estado online-only por ratio físico/lógico

## Notas y límites

- La detección cloud depende de atributos/xattrs disponibles por proveedor/cliente.
- En volúmenes o políticas corporativas específicas, ciertos metadatos cloud pueden variar.
- En árboles extremadamente grandes, usar `-exclude` y/o `-max-depth` mejora latencia inicial.
