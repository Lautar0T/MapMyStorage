# Instructivo de uso: real-disk-map

Este documento explica:
- qué hace la herramienta,
- cómo usarla en macOS y Windows,
- cómo usar la extensión de Raycast,
- y qué hace cada archivo/carpeta principal del proyecto.

## 1) Qué hace la herramienta

`real-disk-map` escanea un directorio raíz (por defecto, tu home) y calcula:
- `physical_size`: tamaño **real** asignado en disco (`allocated size` / `size on disk`),
- `logical_size`: tamaño lógico del archivo.

El ranking y orden por defecto usan `physical_size`.
Esto permite ver correctamente placeholders cloud (online-only), sparse files y archivos comprimidos.

## 2) Uso rápido del CLI

### Comando principal

```bash
rdm-cli [opciones] [ruta]
```

Si no pasás ruta, usa tu home.

### Modos

1. TUI interactiva (por defecto)
```bash
rdm-cli
rdm-cli /ruta/a/escanear
```

2. JSON (para integrar con otras herramientas)
```bash
rdm-cli -json -root ~ > salida.json
```

3. CSV
```bash
rdm-cli -csv -root ~ > salida.csv
```

4. Texto no interactivo
```bash
rdm-cli -no-interactive -root ~
```

### Filtros útiles

```bash
rdm-cli -exclude ".git,node_modules,*.tmp" -hidden -max-depth 4
```

- `-exclude`: patrones separados por coma
- `-hidden`: incluye ocultos
- `-max-depth`: profundidad máxima (`0` = ilimitada)
- `-follow-symlinks`: opcional; por defecto **false**

### Atajos en TUI

- `↑/↓` o `k/j`: mover selección
- `Enter`: entrar carpeta / revelar archivo
- `Backspace` o `←`: subir
- `r`: refrescar escaneo
- `s`: cambiar criterio de orden
- `/`: buscar por nombre/path
- `h`: ocultar/mostrar hidden
- `l`: alternar vista real/lógica
- `v`: resumen visual (barras ASCII)
- `o`: abrir/revelar en Finder/Explorer
- `t`: abrir en terminal
- `y`: copiar ruta al portapapeles
- `e`: exportar JSON/CSV
- `q`: salir

## 3) Uso por plataforma

## macOS

### Requisitos
- Go (según `go.mod`),
- Node.js (solo para Raycast),
- Make.

### Build e instalación del CLI

```bash
cd ollama_version
make build
# bin/rdm-cli

# opcional: instalar en PATH
make install
```

### Ejemplos

```bash
rdm-cli ~/Library
rdm-cli -json -root ~/Dropbox > dropbox-real.json
rdm-cli -csv -root ~/Downloads > downloads-real.csv
```

### Raycast (macOS)

```bash
cd ollama_version/raycast-extension
npm install
npm run build
```

Después importás la carpeta `raycast-extension` en Raycast.

Preferencias configurables:
- root default,
- ocultos,
- exclusiones,
- symlinks,
- max depth.

Acciones en la lista:
- Enter Directory,
- Open in Finder,
- Reveal File in Finder,
- Open in Terminal,
- Copy Path,
- Refresh,
- Change Root.

## Windows

### Opción A: usar binario release
- Descargá `rdm-cli-windows-amd64.exe` y agregalo al PATH.

### Opción B: compilar con Go
En PowerShell:

```powershell
cd ollama_version
$env:GOOS="windows"
$env:GOARCH="amd64"
go build -o .\bin\rdm-cli.exe .\cmd\rdm-cli
```

### Uso

```powershell
.\bin\rdm-cli.exe
.\bin\rdm-cli.exe -json -root C:\Users\TuUsuario > salida.json
.\bin\rdm-cli.exe -csv -root C:\ > salida.csv
```

En Windows, el cálculo real usa APIs Win32 (`GetCompressedFileSizeW` + cluster size), y además detecta señales de cloud placeholders por atributos de archivo.

## 4) Flujo recomendado

1. Empezá con TUI en tu home.
2. Ordená por real (default) y abrí carpetas grandes.
3. Usá `l` para comparar real vs lógico.
4. Si necesitás automatizar/reportar, exportá JSON o CSV.
5. Para acceso diario en macOS, usá el comando Raycast.

## 5) Mapa de archivos y para qué sirve cada uno

## Core (compartido)

- `core/models/types.go`
  - Tipos compartidos (`Entry`, `ScanConfig`, eventos de progreso, sort, walking del árbol).

- `core/disk/scanner.go`
  - Escaneo recursivo (concurrencia, exclusiones, hidden, symlinks, progreso async).
  - Calcula totales de carpetas sumando hijos por `physical_size`.

- `core/disk/size_darwin.go`
  - Implementación macOS del tamaño real (`st_blocks * 512`).

- `core/disk/size_windows.go`
  - Implementación Windows con Win32 (`GetCompressedFileSizeW`, `GetDiskFreeSpaceW`).

- `core/disk/size_unix.go`
  - Implementación Unix genérica (no-darwin, no-windows).

- `core/disk/cloud_detector.go`
  - Detección de proveedor/estado cloud por xattrs o heurística.

- `core/disk/cloud_windows_attrs_windows.go`
  - Detección cloud online-only/local mediante atributos Windows.

- `core/disk/cloud_windows_attrs_stub.go`
  - Stub no-Windows para compilar cross-platform.

- `core/disk/xattr_darwin.go`
  - Helpers xattr en macOS.

- `core/disk/xattr_stub.go`
  - Stub no-darwin para compilar cross-platform.

- `core/disk/metadata_unix.go`
  - Ownership UID/GID para Unix.

- `core/disk/metadata_windows.go`
  - Stub ownership en Windows.

- `core/export/json.go`
  - Export de resultados a JSON/CSV + summary.

- `core/cache/cache.go`
  - Infraestructura de cache persistente (opcional/no crítica en el flujo actual).

- `core/disk/scanner_test.go`
  - Tests de escáner, exclusiones, symlink, sparse file e inferencia online-only.

## CLI

- `cmd/rdm-cli/main.go`
  - Entrada principal, flags, modo TUI/batch, invocación del scanner.

- `cmd/rdm-cli/tui/model.go`
  - Estado de UI, keymap, acciones OS (open/reveal/terminal/copy/export).

- `cmd/rdm-cli/tui/update.go`
  - Manejo de input y eventos (incluye progreso de escaneo).

- `cmd/rdm-cli/tui/view.go`
  - Render de tabla, columnas, colores por tamaño y panel visual.

- `cmd/rdm-cli/commands/root.go`
  - Código legacy, actualmente marcado con build tag `ignore`.

## Raycast

- `raycast-extension/src/index.tsx`
  - UI principal Raycast, navegación por carpetas, acciones y refresh.

- `raycast-extension/src/utils/exec.ts`
  - Puente con `rdm-cli -json` (reutiliza el core).

- `raycast-extension/src/utils/format.ts`
  - Helpers de formato (bytes, ratio, fechas, colores, status).

- `raycast-extension/src/scan.tsx`
  - Entry point para comando `scan`.

- `raycast-extension/src/quick-scan.tsx`
  - Entry point para comando `quick-scan`.

- `raycast-extension/package.json`
  - Metadata, comandos y preferencias de la extensión.

## Build / docs

- `README.md`
  - Guía principal del proyecto.

- `Makefile`
  - Build/test/package para distintas plataformas.

- `scripts/build.sh`
  - Script de build manual.

- `scripts/install.sh`
  - Instalación rápida en macOS/Linux.

- `go.mod` / `go.sum`
  - Dependencias del módulo Go.

## 6) Troubleshooting rápido

1. "No veo el tamaño real correcto"
- Confirmá que estás mirando `physical_size` (no `logical_size`).
- En TUI, usá `l` para comparar columnas.

2. "Permisos denegados"
- El escáner sigue sin romper el proceso; esas rutas se omiten y cuentan como error.

3. "Raycast no encuentra rdm-cli"
- Instalá/compilá el binario y asegurá PATH o ubicación en rutas detectadas por `exec.ts`.

4. "Placeholder cloud aparece con tamaño"
- Puede mostrar tamaño físico mínimo real local (metadata/cluster), no el tamaño lógico total remoto.
