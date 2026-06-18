Output demo/vhs/shux-demo.gif

Require sh

Set Shell "bash"
Set FontSize 16
Set Width 1280
Set Height 720
Set Theme "Catppuccin Mocha"
Set Padding 32
Set Framerate 30
Set PlaybackSpeed 1.25
Set TypingSpeed 40ms
Set LoopOffset 70%

# Bootstrap isolated daemon off-screen (demo bind port + state dir).
Hide
Type "./demo/vhs/reset-demo.sh"
Enter
Sleep 500ms
Type "./demo/vhs/run-shux.sh"
Enter
Wait+Screen@30s /sh-5\.3\$ /
Show
Sleep 1s

# Top-left: docs pane.
Type "echo '  docs | less README'"
Enter
Sleep 500ms

# Split right: long-running process pane.
Ctrl+B
Type "%"
Sleep 400ms
Type "echo '  node | long-running process'"
Enter
Sleep 500ms

# Split bottom-left: editor pane.
Ctrl+B
Type "o"
Sleep 200ms
Ctrl+B
Copy '"'
Paste
Sleep 400ms
Type "echo '  editor | nano draft'"
Enter
Sleep 500ms

# Split bottom-right: shell pane.
Ctrl+B
Type "o"
Sleep 200ms
Ctrl+B
Type "o"
Sleep 200ms
Ctrl+B
Copy '"'
Paste
Sleep 400ms
Type "echo '  shell | run commands here'"
Enter
Sleep 1.5s

# Detach; daemon keeps journaling pane output.
Ctrl+B
Type "d"
Sleep 800ms

# Reattach without flashing the bare shell or command line.
Hide
Type "clear"
Enter
Type "./demo/vhs/run-shux.sh"
Enter
Wait+Screen@30s /shell \| run commands here/
Show
Sleep 3s

# Clean shutdown for the next render.
Hide
Ctrl+B
Type "q"
Sleep 500ms
