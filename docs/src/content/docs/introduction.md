---
title: Introduction
description: What is shux and why it exists.
---

## What is shux?

`shux` is a terminal multiplexer for Linux built around three core ideas:

1. **Real terminal emulation** via Ghostty
2. **Simple actor-style concurrency** for sessions, windows, and panes
3. **Disk-backed recovery** so sessions can be detached, reattached, and restored

## Why shux?

shux aims at the same daily workflow space as tmux, but with a smaller, more explicit architecture. It is designed to be:

- **Minimal** — bounded, predictable state management
- **Reliable** — disk-backed persistence means your sessions survive crashes
- **Modern** — built on a full terminal emulator (Ghostty) for accurate terminal behavior

## Status

**Pre-release.**

shux is already useful for real workflows, but it is still moving quickly. The core session/window/pane model, splitting, navigation, disk snapshots, and live reattach paths are implemented. Some user-facing tmux-like actions already exist in the action/keymap layer but are still stubs or incomplete in the UI.

See the [ROADMAP](https://github.com/nhelmig/shux/blob/main/ROADMAP.md) for the current target shape.
