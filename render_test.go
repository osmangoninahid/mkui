package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var update = flag.Bool("update", false, "update .golden files under testdata/")

// fixtureTargets deliberately mixes ASCII, a Latin-1 accented name and a
// double-width CJK name so a cell-width regression (invariant 4) or a lost
// glyph shows up as a golden diff rather than passing silently.
func fixtureTargets() []Target {
	return []Target{
		{Name: "build", Doc: "Compile the binary", Phony: true},
		{Name: "déployer", Doc: "Déployer en production — avec vérification", Phony: true},
		{Name: "测试", Doc: "Run the test suite", Phony: true},
		{Name: "main.o"},
	}
}

// renderCase renders the fixture UI at a given color profile and width. The
// color profile is global lipgloss state, so cases run sequentially, not in
// parallel.
func renderCase(profile termenv.Profile, width, height int) string {
	lipgloss.SetColorProfile(profile)
	m := NewModel("/home/you/project", "Makefile", fixtureTargets())
	m.width, m.height = width, height
	m.cursor = 1
	return m.View()
}

// escapeANSI makes the golden files reviewable: an ESC byte becomes a literal
// `\e`, so a color regression is a legible diff instead of terminal-controlling
// noise a reviewer cannot see.
func escapeANSI(s string) string {
	return strings.ReplaceAll(s, "\x1b", `\e`)
}

var renderCases = []struct {
	name    string
	profile termenv.Profile
	width   int
}{
	{"truecolor", termenv.TrueColor, 62},
	{"ansi256", termenv.ANSI256, 62},
	{"ansi16", termenv.ANSI, 62},
	// NO_COLOR is the Ascii profile: reverse video is stripped, so the ▸ glyph
	// is the only remaining marker of the selected row (invariant 3). The glyph
	// stays Unicode here because the locale is UTF-8 — NO_COLOR is about color,
	// not glyphs.
	{"nocolor", termenv.Ascii, 62},
	// A width that forces the doc column to truncate on a cell boundary.
	{"narrow", termenv.ANSI256, 34},
}

func TestRenderGolden(t *testing.T) {
	// Pin everything unicodeOK() and the renderer read, so the goldens do not
	// depend on the developer's shell. UTF-8 locale + no MKUI_ASCII + non-linux
	// TERM means glyphs are Unicode across every case.
	t.Setenv("MKUI_ASCII", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("LANG", "en_US.UTF-8")

	for _, tc := range renderCases {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeANSI(renderCase(tc.profile, tc.width, 12))
			golden := filepath.Join("testdata", tc.name+".golden")

			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (regenerate with: go test -run TestRenderGolden -update): %v", err)
			}
			if got != string(want) {
				t.Errorf("render mismatch for %q\n--- got ---\n%s\n--- want ---\n%s", tc.name, got, string(want))
			}
		})
	}
}

func TestUnicodeOK(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"utf-8 via LANG", map[string]string{"LANG": "en_US.UTF-8"}, true},
		{"C locale is ascii", map[string]string{"LANG": "C"}, false},
		// LC_ALL wins over LANG even when LANG says UTF-8.
		{"LC_ALL precedence", map[string]string{"LC_ALL": "C", "LANG": "en_US.UTF-8"}, false},
		{"TERM=linux forces ascii", map[string]string{"LANG": "en_US.UTF-8", "TERM": "linux"}, false},
		{"MKUI_ASCII forces ascii", map[string]string{"LANG": "en_US.UTF-8", "MKUI_ASCII": "1"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Start from a known-empty slate so a stray var in the dev's shell
			// cannot flip the result.
			for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG", "TERM", "MKUI_ASCII"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if got := unicodeOK(); got != tc.want {
				t.Errorf("unicodeOK() = %v, want %v", got, tc.want)
			}
		})
	}
}
