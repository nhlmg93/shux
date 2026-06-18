---
title: Scrolling
description: Scroll through pane scrollback in shux.
---

Scroll the active pane through scrollback with:

- `Page Up` / `Page Down`
- Mouse wheel over a pane

Scrollback is rendered by libghostty. The default capacity is 10,000 lines (`shux.opt.scrollback`).

When you detach and reattach, resurrection can replay journaled output so scrollback is closer to what you left behind. See [Resurrection](/resurrection/overview/).
