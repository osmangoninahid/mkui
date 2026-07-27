package main

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme centralises every styling decision so portability is one file, not
// scattered literals.
//
// The rule behind these choices: colours 0-15 are *slots* that the user's
// terminal theme fills in. Ask for colour 5 and a Solarized user gets
// Solarized magenta, a Gruvbox user gets Gruvbox magenta. Ask for 212 or
// #ff6ac1 and everyone gets the same fixed pink no matter what their theme
// says, which is how a TUI ends up unreadable on somebody's light background.
//
// Sticking to 0-15 also means we never need to know whether the background is
// light or dark, so we never issue an OSC 11 query. That query is a round trip
// to the terminal that some multiplexers swallow and that races with Bubble
// Tea's own input reader. Not asking the question is more robust than asking
// it well.
type Theme struct {
	Title    lipgloss.Style
	Path     lipgloss.Style
	Selected lipgloss.Style
	Name     lipgloss.Style
	Dim      lipgloss.Style
	Err      lipgloss.Style
	OK       lipgloss.Style

	// Glyphs are theme data too: a Linux console or C-locale SSH session
	// renders non-ASCII as garbage, so every one of these needs a fallback.
	Cursor   string // marker for the selected row
	NoCursor string // same display width as Cursor
	Ellipsis string
	Sep      string // separator in the help line
	Caret    string // filter input caret
	OKMark   string
	ErrMark  string
}

// NewTheme builds the default, terminal-native theme.
func NewTheme() Theme { return newTheme(false) }

// NewBrandTheme layers a brand accent over the terminal-native base.
//
// Only the title and the selection bar take brand color. Body text, the doc
// column and the help line stay on ANSI 0-15, because those are the parts a
// user reads continuously and they must keep working against a background we
// do not control. An accent is a garnish; a whole hardcoded palette is a
// readability bug waiting for someone with a light theme.
func NewBrandTheme() Theme {
	t := newTheme(true)
	t.Title = lipgloss.NewStyle().Bold(true).Foreground(brand.Accent)
	t.Selected = lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("0")).Background(brand.Accent)
	t.Err = lipgloss.NewStyle().Foreground(brand.Err)
	t.OK = lipgloss.NewStyle().Foreground(brand.Ok)
	return t
}

func newTheme(brandMode bool) Theme {
	_ = brandMode
	const (
		red     = "1"
		green   = "2"
		cyan    = "6"
		bright0 = "8" // "bright black": every theme's designated dim grey
	)

	t := Theme{
		Title: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cyan)),
		Path:  lipgloss.NewStyle().Foreground(lipgloss.Color(bright0)),

		// Reverse swaps the user's own foreground and background, so the
		// selected row is guaranteed to contrast no matter the theme. A
		// hardcoded highlight colour can collide with the background; this
		// cannot.
		Selected: lipgloss.NewStyle().Reverse(true).Bold(true),

		// Unstyled: most targets are ordinary, and colouring everything means
		// colouring nothing.
		Name: lipgloss.NewStyle(),
		Dim:  lipgloss.NewStyle().Foreground(lipgloss.Color(bright0)),

		Err: lipgloss.NewStyle().Foreground(lipgloss.Color(red)),
		OK:  lipgloss.NewStyle().Foreground(lipgloss.Color(green)),
	}

	// Under NO_COLOR every attribute is stripped, reverse video included, so
	// the cursor glyph becomes the only thing marking the selection. It has to
	// be text, not styling.
	if unicodeOK() {
		t.Cursor, t.NoCursor, t.Ellipsis = "\u25b8 ", "  ", "\u2026"
		t.Sep, t.Caret = " \u00b7 ", "\u2596"
		t.OKMark, t.ErrMark = "\u2713", "\u2717"
	} else {
		t.Cursor, t.NoCursor, t.Ellipsis = "> ", "  ", "..."
		t.Sep, t.Caret = " | ", "_"
		t.OKMark, t.ErrMark = "OK", "FAIL"
	}
	return t
}

// unicodeOK reports whether it is safe to emit non-ASCII glyphs. A Linux
// virtual console or a C-locale SSH session will render them as garbage.
func unicodeOK() bool {
	if os.Getenv("MKUI_ASCII") != "" {
		return false
	}
	if os.Getenv("TERM") == "linux" {
		return false
	}
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return strings.Contains(strings.ToUpper(v), "UTF-8") ||
				strings.Contains(strings.ToUpper(v), "UTF8")
		}
	}
	return false
}
