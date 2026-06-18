# shux

shux is an experimental terminal multiplexer with a tmux-like attach flow, Neovim-style Lua configuration, and a resurrection layer for durable session recovery.

## Documentation

**Canonical docs:** [https://nhlmg93.github.io/](https://nhlmg93.github.io/)

- [Getting started](https://nhlmg93.github.io/getting-started/install/)
- [Keybindings](https://nhlmg93.github.io/using/keybindings/)
- [Configuration](https://nhlmg93.github.io/configuration/overview/)
- [Resurrection](https://nhlmg93.github.io/resurrection/overview/)
- [CLI reference](https://nhlmg93.github.io/cli/commands/)

Build and preview docs locally:

```bash
make docs-dev    # http://localhost:4321/
```

Example config: [`runtime/example/`](runtime/example/)

## Quick start

```bash
make
./shux
```

Detach with `ctrl+b d`, quit with `ctrl+b q`. See the docs for full usage.

## Status

Early development — APIs and behavior may change. Report issues on [GitHub](https://github.com/nhlmg93/shux/issues).
