package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestRenderProfiles(t *testing.T) {
	os.Setenv("LANG", "en_US.UTF-8")
	os.Unsetenv("TERM")
	targets := []Target{
		{Name: "build", Doc: "Compile the binary", Phony: true},
		{Name: "déployer", Doc: "Déployer en production — avec vérification", Phony: true},
		{Name: "测试", Doc: "Run the test suite", Phony: true},
		{Name: "main.o"},
	}
	m := NewModel("/home/you/project", "Makefile", targets)
	m.width, m.height = 62, 12
	m.cursor = 1

	for _, p := range []struct {
		n string
		p termenv.Profile
	}{{"TrueColor / ANSI256", termenv.ANSI256}, {"ANSI16", termenv.ANSI}, {"NO_COLOR", termenv.Ascii}} {
		lipgloss.SetColorProfile(p.p)
		fmt.Printf("\n========== %s ==========\n%s\n", p.n, m.View())
	}
}
