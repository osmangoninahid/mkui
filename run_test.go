package main

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestArgsPromptInput drives the arg prompt through Update: `a` opens it,
// typing accumulates, backspace deletes a whole rune (not a byte, which would
// corrupt a multibyte value — invariant 4), and esc closes it while keeping the
// typed line so re-opening shows it again.
func TestArgsPromptInput(t *testing.T) {
	send := func(m model, k tea.KeyMsg) model {
		next, _ := m.Update(k)
		return next.(model)
	}

	m := NewModel("/dir", "Makefile", []Target{{Name: "deploy"}}, []string{"ENV"})

	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if !m.argsOn {
		t.Fatal("`a` should open the arg prompt")
	}

	for _, r := range "ENV=café" {
		m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.args != "ENV=café" {
		t.Fatalf("typed args = %q, want %q", m.args, "ENV=café")
	}

	m = send(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.args != "ENV=caf" {
		t.Fatalf("after one backspace args = %q, want %q (the é must go whole)", m.args, "ENV=caf")
	}

	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.argsOn {
		t.Fatal("esc should close the prompt")
	}
	if m.args != "ENV=caf" {
		t.Fatalf("esc should keep the typed buffer, got %q", m.args)
	}
}

func TestSplitArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"pairs split on whitespace", "ENV=staging TAG=v2", []string{"ENV=staging", "TAG=v2"}},
		{"stray and repeated spaces drop empties", "  ENV=staging   TAG=v2 ", []string{"ENV=staging", "TAG=v2"}},
		{"blank yields nothing", "   ", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := splitArgs(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitArgs(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMakeCommand(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target string
		args   string
		want   string
	}{
		{"no args", "build", "", `make 'build'`},
		{"var pairs are separate quoted tokens", "deploy", "ENV=staging TAG=v2", `make 'deploy' 'ENV=staging' 'TAG=v2'`},
		// A single quote in a value must be escaped into the quoting, not left
		// to break out of it — otherwise the value could inject shell syntax
		// into the `sh -c` wrapper.
		{"single quote in value is escaped, not injected", "deploy", "MSG=it's", `make 'deploy' 'MSG=it'\''s'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := makeCommand(tc.target, tc.args); got != tc.want {
				t.Errorf("makeCommand(%q, %q) = %s, want %s", tc.target, tc.args, got, tc.want)
			}
		})
	}
}
