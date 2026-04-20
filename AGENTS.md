# AGENTS.md

## Project Description

Building a terminal multiplexer (gomux) to replace tmux with a simpler, modern architecture. Focus on:
- Single-process design (no client/server split)
- Pure Go with minimal dependencies
- Full terminal emulation via Ghostty
- Clean actor-based concurrency
- GPU-accelerated rendering when hosted in modern terminals

## Expertise

I am a tmux expert. All decisions are rooted in deep understanding of terminal multiplexing patterns and trade-offs.

## Development Guidelines

### Code Changes
- **Small incremental changes only** - Maximum 10-15 lines of code per edit
- User will test between changes
- Only commit when explicitly requested by user

### Communication
- Keep responses minimal (e.g., just "ok") after the user has seen the pattern
- No need to count lines or explicitly ask to test once the workflow is established

### Git Workflow
- Only commit when user explicitly says so
- Write thoughtful but brief commit messages describing what the commit introduces

## Architecture Principles

1. **Single Process** - No daemon/client split. Process dies = session ends. Trade-off for simplicity.
2. **Actor Model** - goroutines + channels for clean concurrency
3. **Full Terminal Emulation** - Ghostty provides complete VT support
4. **Modern Stack** - Go + Bubble Tea, no legacy dependencies
