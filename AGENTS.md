# AGENTS.md

## Project Description

**shux** - A terminal multiplexer. You shouldn't have to think about it.

Building shux to replace tmux with a simpler, modern architecture. Focus on:
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

## Reference Materials (Do Not Auto-Read)

**ARCHITECTURE.md** and **ROADMAP.md** contain long-term planning and design documentation.

**Do not reference, quote, or use these files unless explicitly directed by the user.**

They exist for human reference and long-term planning. During active development, only use the context provided directly in conversation.

If user wants to discuss architecture or roadmap, they will say so explicitly (e.g., "look at ARCHITECTURE.md" or "what's in the roadmap?").
