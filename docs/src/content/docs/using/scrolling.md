---
title: Scrolling
description: Scroll through pane scrollback in shux.
---

Scroll the active pane through scrollback with:

- `Page Up` / `Page Down`
- Mouse wheel over a pane
- `ctrl+b [` to enter copy mode, then arrow keys or `Page Up` / `Page Down`

Copy-mode search highlights can be styled via `shux.opt.ui`; see [UI chrome](/configuration/options/#ui-chrome/).

Scrollback is rendered by libghostty. The default capacity is 10,000 lines (`shux.opt.scrollback`).

When you detach and reattach, resurrection can replay journaled output so scrollback is closer to what you left behind. See [Resurrection](/resurrection/overview/).
