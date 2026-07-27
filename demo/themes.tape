# Renders the SAME screen under four terminal themes, as real PNGs.
#
# This is the portability check that a design mockup cannot give you: the
# output is produced by the actual binary writing actual escape sequences into
# a real terminal emulator. If a color is unreadable on a light theme, it will
# be unreadable here too.
#
#   vhs demo/themes.tape

Output demo/frames/

Require mkui

Set Shell "bash"
Set FontSize 16
Set FontFamily "JetBrains Mono"
Set Width 820
Set Height 380
Set Padding 20
Set TypingSpeed 1ms

# --- dark, truecolor -------------------------------------------------------
Set Theme "Solarized Dark"
Hide
Type "clear" Enter
Show
Type "mkui" Enter
Sleep 2s
Type "q" Enter

# --- light background: the case hardcoded palettes usually break -----------
Set Theme "Solarized Light"
Hide
Type "clear" Enter
Show
Type "mkui" Enter
Sleep 2s
Type "q" Enter

# --- a very different dark palette -----------------------------------------
Set Theme "Gruvbox Dark"
Hide
Type "clear" Enter
Show
Type "mkui" Enter
Sleep 2s
Type "q" Enter

# --- no color at all: the ASCII + cursor-glyph path ------------------------
Env NO_COLOR "1"
Env MKUI_ASCII "1"
Hide
Type "clear" Enter
Show
Type "mkui" Enter
Sleep 2s
Type "q" Enter
