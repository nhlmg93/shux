Output demo/vhs/debug-user-fixed.gif

Require bash

Set Shell "bash"
Set FontSize 16
Set Width 1280
Set Height 720
Set Theme "Catppuccin Mocha"
Set Padding 32
Set Framerate 30
Set PlaybackSpeed 1.0
Set TypingSpeed 40ms

Hide
Type "./demo/vhs/debug-user-fix.sh"
Enter
Sleep 1s
Show

Type "./shux"
Enter
Wait+Screen@30s /[$#❯]/
Sleep 2s
Type "echo shux attach ok"
Enter
Sleep 1.5s

Hide
Ctrl+B
Type "q"
Sleep 500ms
