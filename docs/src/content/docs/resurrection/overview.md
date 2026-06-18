---
title: Recovery model
description: How shux resurrection preserves layout and scrollback across detach and restart.
---

Resurrection is shux's durability layer. When enabled (`shux.opt.resurrection`, default `true`), the daemon records pane output and periodic layout checkpoints so you can come back closer to the state you left.

## What gets saved

| Artifact | Purpose |
| --- | --- |
| **Journals** | Append-only PTY output per pane (bounded by `journal_max_mb`) |
| **Manifest** | Window/pane layout snapshot and metadata |
| **State directory** | `shux.opt.state_dir` (defaults to XDG state path) |

Checkpoints run on meaningful events and before graceful restart.

## Typical workflow

1. Start shux and arrange several panes — documentation in `less`, a long-running process, an editor, a shell.
2. Detach with `ctrl+b d`. The daemon keeps running; journals continue recording.
3. Reattach later with `./shux`. Layout and scrollback replay from the manifest and journals.

This is the user story shux optimizes for: **detach without fear, reattach without starting from a blank terminal.**

## Graceful restart

`shux restart` checkpoints state, spawns a replacement daemon, and hands off the listen address. Clients are detached during restart; reattach to the new backend afterward.

Use restart when you need to reload configuration or upgrade the binary while preserving as much session state as practical.

## Limits

- Resurrection records **terminal output**, not arbitrary process memory. A restarted program inside a pane may need to be relaunched manually.
- Journal size is capped per pane; very chatty output can rotate older data.
- Config changes apply only after a new daemon starts.

## Disabling resurrection

```lua
shux.opt.resurrection = false
```

A fresh daemon with resurrection disabled clears prior resurrection state in `state_dir`.

## Recovery tiers

| Tier | What survives | Notes |
| --- | --- | --- |
| **L0** | Nothing | `resurrection = false` or no state directory |
| **L1** | Layout | Windows, splits, and pane geometry restore; shells respawn |
| **L2** | Layout + screen replay | L1 plus PTY journals replayed into fresh VTs (MVP default) |
| **L3** | Process reattach | Planned — reattach live shell processes after restart |

Journal files are stored at `{state_dir}/panes/win{N}_{paneID}.journal` where `N` is the window's ordinal in the session. Replay is deferred briefly after pane init so a respawned shell does not immediately overwrite restored output.
