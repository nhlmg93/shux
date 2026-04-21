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
make ci-test
```

Notes:

- `make test` runs the local unit/integration suite
- `make ci-test` runs the full Docker-backed suite used for CI, including integration, e2e, fuzz seed coverage, and stress tests
- `make test-native` is kept as a compatibility alias for `make test`
- CI uses both the local suite and the Docker-backed CI suite

## License

MIT
