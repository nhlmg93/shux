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

## Can I hide the status bar or pane borders?

Yes. Set fields on `shux.opt.ui` in your Lua config—for example `shux.opt.ui = { statusline = false }` or `shux.opt.ui = { pane_borders = false }`. See [UI chrome](/configuration/options/#ui-chrome/) for defaults and examples. UI options follow the same daemon-start reload rules as the rest of your config (see above).

## How do I kill a session?

Use the CLI:

```bash
shux kill-session -t work
```

That closes every window in the session, removes it from the daemon, and checkpoints the store. Closing the last pane in the last window of a session ends the session the same way (tmux-like). If you kill the **last** remaining session, the daemon stops and on-disk resurrection state is cleared.

Create additional named sessions with `shux new-session -s NAME`. List them with `shux list-sessions` or `shux ps --sessions`.

## How do I clear persisted resurrection data?

| Goal | Command |
| --- | --- |
| Inspect on-disk store | `shux ls` |
| Remove orphan journals | `shux prune` |
| Wipe manifest + all journals | `shux rm` |
| Kill last session (daemon stops, store cleared) | `shux kill-session -t main` |

`shux rm` works offline. With a running daemon, use `shux rm --force` or stop the daemon first. Journals for closed panes are deleted automatically; each checkpoint also prunes orphans.

## Can I run multiple daemons?

Each daemon binds to `shux.opt.bind` (default `127.0.0.1:23234`). Use a different bind address in config for multiple instances.

## Does process state survive `shux restart`?

Yes, on a graceful restart path shux uses L3 handoff so pane PTYs and long-running shell processes stay alive. If L3 is unavailable (for example a cold daemon start after crash/reboot), shux falls back to L2 layout + journal replay.

## Is shux production-ready?

shux is an early MVP suitable for daily-driver experimentation on a single machine. Resurrection now supports L3 for graceful restart and L2 fallback for cold starts; expect occasional breaking changes.

## Where is the full documentation?

This site is canonical. The [GitHub wiki](https://github.com/nhlmg93/shux/wiki) links here for discoverability and community notes.
