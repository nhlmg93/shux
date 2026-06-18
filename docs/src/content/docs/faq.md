---
title: FAQ
description: Frequently asked questions about shux.
---

## How is shux different from tmux?

shux aims for a smaller, modern architecture with built-in resurrection (journaling + layout restore) and Neovim-style Lua configuration. Keybindings are tmux-like by default, but the internals and config model differ.

## Why doesn't my config change after I edit `init.lua`?

Configuration loads once at daemon start. Attach does not reload Lua. Use `shux restart` or quit and start a new daemon.

## Does `--bash` affect an already-running daemon?

No. `--bash` only applies when spawning a new daemon.

## Where is resurrection data stored?

In `shux.opt.state_dir`, which defaults to your XDG state directory. Journals and the layout manifest live there.

## Can I run multiple daemons?

Each daemon binds to `shux.opt.bind` (default `127.0.0.1:23234`). Use a different bind address in config for multiple instances.

## Is shux production-ready?

No. shux is experimental. Expect breaking changes while the recovery model and plugin API mature.

## Where is the full documentation?

This site is canonical. The [GitHub wiki](https://github.com/nhlmg93/shux/wiki) links here for discoverability and community notes.
