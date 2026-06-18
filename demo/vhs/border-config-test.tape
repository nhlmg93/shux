Output demo/vhs/border-config-test.gif

Require bash

Set Shell "bash"
Set FontSize 16
Set Width 960
Set Height 540
Set Theme "Catppuccin Mocha"
Set Padding 24
Set Framerate 24
Set PlaybackSpeed 1.0
Set TypingSpeed 35ms

# Isolated daemon with demo/vhs/config (pane_border_lines=single, pane_outer_border=false).
Hide
Type "./demo/vhs/reset-demo.sh"
Enter
Sleep 600ms
Type "./demo/vhs/run-shux.sh"
Enter
Sleep 5s
Show
Sleep 1s

# Top pane marker.
Type "echo TOP-PANE"
Enter
Wait+Screen@15s /TOP-PANE/
Sleep 800ms

# Horizontal split.
Ctrl+B
Copy '"'
Paste
Sleep 500ms
Type "echo BOTTOM-LEFT"
Enter
Sleep 1s

# Vertical split in bottom row.
Ctrl+B
Type "%"
Sleep 500ms
Type "echo BOTTOM-RIGHT"
Enter
Sleep 2s

# Hold frame for border inspection (no outer box, single split lines).
Sleep 3s

Hide
Ctrl+B
Type "q"
Sleep 500ms
