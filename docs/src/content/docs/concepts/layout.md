---
title: Sessions, windows, and panes
description: The layout hierarchy inside a shux daemon.
---

shux organizes terminals in a familiar tmux-like hierarchy:

```text
Session
 └── Window 1
 │    ├── Pane A
 │    └── Pane B
 └── Window 2
      └── Pane C
```

## Session

A session is the top-level container. Today shux bootstraps a default session when the daemon starts. Session state includes which windows exist and their layout.

## Window

A window is a tab-like container with its own pane tree. Switch windows with `ctrl+b n` / `ctrl+b p` or `ctrl+b 1`–`9` and `ctrl+b 0` for window 10.

Create a window with `ctrl+b c`.

## Pane

A pane is a single terminal surface backed by a PTY running your shell or program. Split panes with:

| Key | Split |
| --- | --- |
| `ctrl+b %` | Left / right |
| `ctrl+b "` | Top / bottom |

Focus the next pane with `ctrl+b o`. Close the active pane with `ctrl+b x`.

If you close the last pane in a window, that window closes. If you close the last window, shux quits.

## Focus and input

Only one pane is active at a time. Keys (except the shux prefix) go to the active pane. The prefix key (`ctrl+b` by default) is intercepted by shux for multiplexing commands.
