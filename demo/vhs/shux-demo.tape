Output demo/vhs/shux-demo.gif

Require bash
Require node
Require less
Require nvim

Set Shell "bash"
Set FontSize 16
Set Width 1280
Set Height 720
Set Theme "Catppuccin Mocha"
Set Padding 32
Set Framerate 30
Set PlaybackSpeed 1.25
Set TypingSpeed 40ms
Set LoopOffset 82%

# Bootstrap isolated daemon off-screen (demo bind port + state dir).
Hide
Type "./demo/vhs/reset-demo.sh"
Enter
Sleep 500ms
Type "./demo/vhs/run-shux.sh"
Enter
Wait+Screen@30s /SYNC:OFF/
Show
Sleep 1.5s

# p-1: less README (stays running in the background).
Type "./demo/vhs/open-less.sh"
Enter
Sleep 1.5s

# p-2: long-running Node heartbeat.
Ctrl+B
Type "%"
Sleep 500ms
Type "./demo/vhs/run-node-heartbeat.sh"
Enter
Sleep 2s
Wait+Screen@10s /node \| heartbeat/

# p-3: Neovim draft (leave the editor open).
Ctrl+B
Type "o"
Sleep 300ms
Ctrl+B
Copy '"'
Paste
Sleep 500ms
Type "./demo/vhs/open-nvim-draft.sh"
Enter
Sleep 2s
Wait+Screen@15s /edited in neovim/

# p-4: real shell output from wc.
Ctrl+B
Type "o"
Sleep 300ms
Ctrl+B
Type "o"
Sleep 300ms
Ctrl+B
Copy '"'
Paste
Sleep 500ms
Type "./demo/vhs/run-wc-demo.sh"
Enter
Sleep 800ms
Wait+Screen@10s /38 README/
Sleep 2.5s

# Detach; daemon keeps journaling pane output.
Ctrl+B
Type "d"
Sleep 800ms

# Reattach — show all four panes restored.
Hide
Type "clear"
Enter
Type "./demo/vhs/run-shux.sh"
Enter
Wait+Screen@30s /edited in neovim/
Show
Sleep 3.5s

# Clean shutdown for the next render.
Hide
Ctrl+B
Type "q"
Sleep 500ms
