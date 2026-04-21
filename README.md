# shux

> you shouldn't have.

`shux` is a terminal multiplexer for Linux built around three ideas:

- **real terminal emulation** via Ghostty
- **simple actor-style concurrency** for sessions, windows, and panes
- **disk-backed recovery** so sessions can be detached, reattached, and restored

It targets the same daily workflow space as tmux, but with a smaller, more explicit architecture.

## Status

**Pre-release.**

shux is already useful for real workflows, but it is still moving quickly. Core session/window/pane behavior, splits, navigation, snapshots, detach/reattach, and restore are in place. Some tmux-like actions and UX paths are still incomplete.

## Documentation

The docs site lives in [`docs/`](./docs).

Start it locally with:

```bash
cd docs
npm install
npm run dev
```

Useful entry points:

- [`docs/src/content/docs/introduction.md`](./docs/src/content/docs/introduction.md)
- [`docs/src/content/docs/installation.md`](./docs/src/content/docs/installation.md)
- [`docs/src/content/docs/quick-start.md`](./docs/src/content/docs/quick-start.md)
- [`ROADMAP.md`](./ROADMAP.md)

## Highlights

- Full terminal emulation via Ghostty
- True color, mouse, Unicode, and scrollback support
- Named sessions with disk-backed snapshots
- Live reattach when the owner is still running
- Snapshot restore when the owner is gone
- Lua config with lightweight plugin hooks

## Build

Requirements:

- Go 1.26+
- Zig 0.15.2
- `pkg-config`
- a C/C++ toolchain

Build `shux`:

```bash
make
```

That produces `./shux` in the repo root.

## Run

```bash
./shux
```

## Development

Useful commands:

```bash
make
make test
make ci-test
```

Notes:

- `make test` runs the local Go test suite
- `make ci-test` runs the Docker-backed CI suite
- `make test-native` is kept as a compatibility alias for `make test`

## License

MIT
