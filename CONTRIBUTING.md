# Contributing

Thanks for helping improve `real-disk-map`.

## Requirements

- Go 1.25 or newer
- Make

## Development

```sh
make test
make vet
make build
```

Run the CLI locally:

```sh
./bin/rdm-cli -no-interactive -root . -max-depth 1
```

Before opening a pull request, run:

```sh
go fmt ./...
go test ./...
go vet ./...
```

Keep changes focused and include tests for scanner, export, or CLI behavior when changing those areas.
