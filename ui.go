package main

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

type runFinishedMsg struct {
	target string
	err    error
}

type model struct {
	dir      string
	makefile string

	all      []Target
	filtered []Target

	cursor   int
	offset   int
	filter   string
	filterOn bool

	width, height int
	status        string
	theme         Theme
}

func NewModel(dir, makefile string, targets []Target) model {
	return model{
		dir:      dir,
		makefile: makefile,
		all:      targets,
		filtered: targets,
		width:    80,
		height:   24,
		theme:    NewTheme(),
	}
}

func (m model) Init() tea.Cmd { return nil }

// visibleRows is the list height, leaving room for header, footer and status.
func (m model) visibleRows() int {
	n := m.height - 5
	if n < 1 {
		return 1
	}
	return n
}

func (m *model) applyFilter() {
	if m.filter == "" {
		m.filtered = m.all
	} else {
		q := strings.ToLower(m.filter)
		out := make([]Target, 0, len(m.all))
		for _, t := range m.all {
			if strings.Contains(strings.ToLower(t.Name), q) ||
				strings.Contains(strings.ToLower(t.Doc), q) {
				out = append(out, t)
			}
		}
		m.filtered = out
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

func (m *model) clampOffset() {
	rows := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// run hands the terminal over to make so colours, progress bars and any
// interactive prompt behave exactly as they would if you had typed the
// command. The trailing `read` keeps the output on screen until you are done
// reading it, instead of snapping straight back into the alt screen.
func (m model) run(t Target) tea.Cmd {
	script := fmt.Sprintf(
		`make %s; code=$?; `+
			`printf '\n\033[2m── exit %%s ── press enter to return ──\033[0m' "$code"; `+
			`read -r _; exit $code`,
		shellQuote(t.Name),
	)
	c := exec.Command("sh", "-c", script)
	c.Dir = m.dir
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return runFinishedMsg{target: t.Name, err: err}
	})
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampOffset()
		return m, nil

	case runFinishedMsg:
		if msg.err != nil {
			m.status = m.theme.Err.Render(fmt.Sprintf("%s %s failed: %v", m.theme.ErrMark, msg.target, msg.err))
		} else {
			m.status = m.theme.OK.Render(fmt.Sprintf("%s %s", m.theme.OKMark, msg.target))
		}
		return m, nil

	case tea.KeyMsg:
		if m.filterOn {
			switch msg.String() {
			case "esc", "ctrl+c":
				m.filterOn, m.filter = false, ""
				m.applyFilter()
			case "enter":
				m.filterOn = false
			case "backspace":
				if m.filter != "" {
					m.filter = m.filter[:len(m.filter)-1]
					m.applyFilter()
				}
			default:
				if len(msg.Runes) > 0 {
					m.filter += string(msg.Runes)
					m.applyFilter()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "/":
			m.filterOn = true
			m.status = ""
		case "j", "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.clampOffset()
			}
		case "k", "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.clampOffset()
			}
		case "g", "home":
			m.cursor, m.offset = 0, 0
		case "G", "end":
			m.cursor = len(m.filtered) - 1
			m.clampOffset()
		case "enter":
			if len(m.filtered) > 0 {
				m.status = ""
				return m, m.run(m.filtered[m.cursor])
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	th := m.theme

	b.WriteString(th.Title.Render("mkui"))
	b.WriteString("  ")
	b.WriteString(th.Path.Render(fmt.Sprintf("%s/%s", m.dir, m.makefile)))
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(th.Dim.Render("  no matching targets") + "\n")
	}

	rows := m.visibleRows()
	end := m.offset + rows
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	// Column width must be measured in cells, not bytes. A CJK character is
	// one rune but occupies two columns, and a multi-byte rune is several
	// bytes but one column; len() is wrong in both directions.
	nameW := 0
	for _, t := range m.filtered[m.offset:end] {
		if w := runewidth.StringWidth(t.Name); w > nameW {
			nameW = w
		}
	}

	cursorW := runewidth.StringWidth(th.Cursor)

	for i := m.offset; i < end; i++ {
		t := m.filtered[i]
		selected := i == m.cursor

		doc := ""
		if t.Doc != "" {
			// Truncate on cell boundaries so a rune is never cut in half.
			if avail := m.width - nameW - cursorW - 2; avail > 3 {
				doc = runewidth.Truncate(t.Doc, avail, th.Ellipsis)
			}
		}

		// Only pad the name column when something follows it, otherwise every
		// undocumented row carries invisible trailing whitespace.
		pad := ""
		if doc != "" {
			pad = strings.Repeat(" ", nameW-runewidth.StringWidth(t.Name)) + "  "
		}

		if selected {
			// Style the row as one unit: reverse video applied per-span would
			// leave unhighlighted gaps between the columns. Pad to the full
			// width so the selection reads as a bar rather than a ragged blob.
			row := th.Cursor + t.Name + pad + doc
			if gap := m.width - runewidth.StringWidth(row); gap > 0 {
				row += strings.Repeat(" ", gap)
			}
			b.WriteString(th.Selected.Render(row) + "\n")
			continue
		}

		row := th.NoCursor + th.Name.Render(t.Name) + pad
		if doc != "" {
			row += th.Dim.Render(doc)
		}
		b.WriteString(row + "\n")
	}

	b.WriteString("\n")
	switch {
	case m.filterOn:
		b.WriteString(fmt.Sprintf("/%s", m.filter) + th.Dim.Render(th.Caret))
	case m.status != "":
		b.WriteString(m.status)
	default:
		keys := strings.Join([]string{
			"up/down move", "enter run", "/ filter", "q quit",
		}, th.Sep)
		b.WriteString(th.Dim.Render(fmt.Sprintf("%s   %d/%d",
			keys, len(m.filtered), len(m.all))))
	}
	return b.String()
}
