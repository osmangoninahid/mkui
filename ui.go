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

	// vars are the makefile-defined variables offered as `VAR=value` override
	// candidates; args is the line being edited in the arg prompt before a run.
	vars   []string
	args   string
	argsOn bool

	width, height int
	status        string
	theme         Theme
}

func NewModel(dir, makefile string, targets []Target, vars []string) model {
	return model{
		dir:      dir,
		makefile: makefile,
		all:      targets,
		filtered: targets,
		vars:     vars,
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
func (m model) run(t Target, args string) tea.Cmd {
	script := fmt.Sprintf(
		`%s; code=$?; `+
			`printf '\n\033[2m── exit %%s ── press enter to return ──\033[0m' "$code"; `+
			`read -r _; exit $code`,
		makeCommand(t.Name, args),
	)
	c := exec.Command("sh", "-c", script)
	c.Dir = m.dir
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return runFinishedMsg{target: t.Name, err: err}
	})
}

// makeCommand builds the `make ...` invocation. The target and every VAR=value
// token are shell-quoted before they reach `sh -c`, so a value cannot inject
// shell syntax (`$(...)`, `;`, `&&`). splitArgs tokenises on whitespace, which
// is why a value containing spaces is out of scope by design.
func makeCommand(target, args string) string {
	cmd := "make " + shellQuote(target)
	for _, tok := range splitArgs(args) {
		cmd += " " + shellQuote(tok)
	}
	return cmd
}

// splitArgs turns the raw arg line into individual make arguments on
// whitespace, dropping empty fields from stray or repeated spaces.
func splitArgs(s string) []string {
	return strings.Fields(s)
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

		if m.argsOn {
			switch msg.String() {
			case "esc", "ctrl+c":
				// Leave the prompt but keep the typed line: re-opening with `a`
				// shows it again, so tweaking one value and re-running is cheap.
				m.argsOn = false
			case "enter":
				m.argsOn = false
				if len(m.filtered) > 0 {
					m.status = ""
					return m, m.run(m.filtered[m.cursor], m.args)
				}
			case "backspace":
				if m.args != "" {
					// Trim by rune, not byte: a value may hold multibyte text and
					// slicing a byte would leave a half-rune (invariant 4).
					r := []rune(m.args)
					m.args = string(r[:len(r)-1])
				}
			default:
				if len(msg.Runes) > 0 {
					m.args += string(msg.Runes)
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
		case "a":
			// Open the arg prompt for the selected target. Plain `enter` still
			// runs with no args, so this stays out of the common path.
			if len(m.filtered) > 0 {
				m.argsOn = true
				m.status = ""
			}
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
				return m, m.run(m.filtered[m.cursor], "")
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
	case m.argsOn:
		target := ""
		if len(m.filtered) > 0 {
			target = m.filtered[m.cursor].Name
		}
		// The command you would have typed: `make <target> <VAR=value...>`,
		// with the same caret as the filter prompt. Plain text, so it reads
		// correctly under NO_COLOR where styling is stripped (invariant 3).
		b.WriteString("make " + target + " " + m.args + th.Dim.Render(th.Caret))
		if len(m.vars) > 0 {
			// A discovery hint: the variables this makefile actually defines.
			// Truncate on cell boundaries so a wide rune is never sliced in two.
			hint := "vars: " + strings.Join(m.vars, " ")
			b.WriteString("\n" + th.Dim.Render(runewidth.Truncate(hint, m.width, th.Ellipsis)))
		}
	case m.filterOn:
		b.WriteString(fmt.Sprintf("/%s", m.filter) + th.Dim.Render(th.Caret))
	case m.status != "":
		b.WriteString(m.status)
	default:
		keys := strings.Join([]string{
			"up/down move", "enter run", "a args", "/ filter", "q quit",
		}, th.Sep)
		b.WriteString(th.Dim.Render(fmt.Sprintf("%s   %d/%d",
			keys, len(m.filtered), len(m.all))))
	}
	return b.String()
}
