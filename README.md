# shux

> you shouldn't have.

`shux` is an experimental terminal multiplexer built around full terminal emulation, a single-process design, and simple disk-backed session restore.

A larger docs site can come later. For now, this README just covers the basics.

## Status

Pre-release and still changing quickly.
Expect rough edges, missing features, and config/runtime details that may evolve.

## Highlights

- Full terminal emulation via Ghostty
- True color, mouse, Unicode, and scrollback support
- Single-process design with no daemon or client/server split
- Disk-backed detach and restore
- Lua config with lightweight plugin hooks

## Build

Requirements:

- Go 1.26+
- Zig 0.15.2
- `pkg-config`
- a C toolchain

Build `shux`:

```bash
make
```

That produces `./shux` in the repo root.

## Run

```bash
./shux
```

## Contributing

Contributions are welcome, but the project is still pre-release and moving quickly.
Small, focused changes are easiest to review.

Useful commands:

```bash
make
make test
make test-native
make bench-persistence
```

Notes:

- `make test` is the full Docker-backed path
- `make test-native` runs on the host and expects tools like `nano`, `vim`, and `less`
- `make bench-persistence` runs snapshot and attach/detach benchmarks for the disk-backed session path
- CI uses a cached native workflow for normal pushes and pull requests, plus a separate Docker parity workflow
- fuzz targets exist for pure helpers like snapshot decoding, stat parsing, and row rendering

## License

MIT
