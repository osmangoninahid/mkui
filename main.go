package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// version is overwritten at build time via -ldflags.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	list := flag.Bool("list", false, "print targets and exit (no TUI)")
	brandMode := flag.Bool("brand", false, "use brand accent colors instead of terminal-native")
	flag.Parse()

	if *showVersion {
		fmt.Println("mkui", version)
		return
	}

	dir, name, err := FindMakefile(".")
	if err != nil {
		fail(err)
	}

	targets, err := LoadTargets(dir, name)
	if err != nil {
		fail(err)
	}
	if len(targets) == 0 {
		fail(fmt.Errorf("no targets found in %s", name))
	}

	// Stay useful in pipelines and scripts: `mkui --list | fzf` should work,
	// and a TUI would be wrong if stdout is not a terminal anyway.
	if *list || !isTTY() {
		for _, t := range targets {
			if t.Doc != "" {
				fmt.Printf("%s\t%s\n", t.Name, t.Doc)
			} else {
				fmt.Println(t.Name)
			}
		}
		return
	}

	mdl := NewModel(dir, name, targets)
	if *brandMode {
		mdl.theme = NewBrandTheme()
	}
	p := tea.NewProgram(mdl, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fail(err)
	}
}

func isTTY() bool {
	st, err := os.Stdout.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mkui:", err)
	os.Exit(1)
}
