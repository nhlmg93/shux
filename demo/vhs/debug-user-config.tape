Output demo/vhs/debug-user-config.gif

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
Type "./demo/vhs/debug-user-setup.sh"
Enter
Sleep 800ms
Show

# Attach with REAL ~/.config/shux (not demo config).
Type "./shux"
Enter
Wait+Screen@45s /[$#❯]/
Sleep 3s

Hide
Ctrl+B
Type "q"
Sleep 500ms
