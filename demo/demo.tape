# Render with:  vhs demo/demo.tape
# Requires vhs, ttyd and ffmpeg on PATH:  brew install vhs ttyd ffmpeg

Output demo/mkui.gif

Require mkui

Set Shell "bash"
Set FontSize 18
Set FontFamily "JetBrains Mono"
Set Width 900
Set Height 480
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Hide
Type "cd $(mktemp -d) && curl -sL https://example.com/sample/Makefile -o Makefile 2>/dev/null || true"
Enter
Type "clear"
Enter
Show

Type "mkui"
Sleep 400ms
Enter
Sleep 1.5s

# Browse the list
Down@300ms 3
Sleep 800ms

# Filter down to a target
Type "/"
Sleep 300ms
Type@120ms "test"
Sleep 1.2s
Enter
Sleep 600ms

# Run it, watch real make output, return
Enter
Sleep 2.5s
Enter
Sleep 1.5s

Type "q"
Sleep 800ms
