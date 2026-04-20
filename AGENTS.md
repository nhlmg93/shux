# AGENTS.md

## Project Description

Building shux to replace tmux with a simpler, modern architecture. Focus on:
- Single-process design (no client/server split)
- Pure Go with minimal dependencies
- Full terminal emulation via Ghostty
- Explicit loop-based concurrency

## Expertise

you are a tmux expert. All decisions are rooted in deep understanding of terminal multiplexing patterns and trade-offs.

## Development Guidelines

### Git Workflow
- Only commit when user explicitly says so
- Write thoughtful but brief commit messages describing what the commit introduces

## External Reference Checkouts

When you need upstream implementation/reference material, these repositories should be available under `/tmp`:
- `/tmp/tmux` -> `https://github.com/tmux/tmux.git`
- `/tmp/libghostty` -> `https://github.com/mitchellh/go-libghostty.git` (Go libghostty bindings)
- `/tmp/bubbletea` -> `https://github.com/charmbracelet/bubbletea.git`
- `/tmp/lipgloss` -> `https://github.com/charmbracelet/lipgloss.git`
- `/tmp/pty` -> `https://github.com/creack/pty.git`

If any of these are missing, clone them into `/tmp` before relying on them for reference.
