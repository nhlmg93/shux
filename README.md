# shux

shux is an experimental terminal multiplexer with a tmux-like attach flow, Neovim-style Lua configuration, and a resurrection layer for durable session recovery.

![Four-pane layout, detach, and reattach with scrollback restored](demo/shux-demo.gif)

## Documentation

**Canonical docs:** [https://nhlmg93.github.io/shux/](https://nhlmg93.github.io/shux/)

- [Getting started](https://nhlmg93.github.io/shux/getting-started/install/)
- [Keybindings](https://nhlmg93.github.io/shux/using/keybindings/)
- [Configuration](https://nhlmg93.github.io/shux/configuration/overview/)
- [Resurrection](https://nhlmg93.github.io/shux/resurrection/overview/)
- [CLI reference](https://nhlmg93.github.io/shux/cli/commands/)

Build and preview docs locally:

```bash
make docs-dev    # http://localhost:4321/shux/
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
