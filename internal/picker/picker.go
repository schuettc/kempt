// Package picker provides an interactive terminal UI for choosing an install
// profile and then refining the set of packages to apply.
package picker

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Profile is a named bundle of packages the user can pick as a starting point.
type Profile struct {
	Name, Description string
	Packages          []string
}

// Item is a single selectable package in the checklist.
type Item struct {
	Name, Description string
	Selected          bool
}

// Result is the outcome of a picker session.
type Result struct {
	Profile   string
	Packages  []string // selected item names, sorted
	Confirmed bool     // false if the user quit/aborted
}

type mode int

const (
	modeProfile mode = iota
	modePackages
)

type model struct {
	mode     mode
	cursor   int
	profiles []Profile
	items    []Item
	selected map[string]bool
	profile  string
	done     bool // true once the user confirmed in modePackages
}

func newModel(profiles []Profile, items []Item) model {
	selected := make(map[string]bool, len(items))
	for _, it := range items {
		if it.Selected {
			selected[it.Name] = true
		}
	}
	m := model{
		mode:     modeProfile,
		profiles: profiles,
		items:    items,
		selected: selected,
	}
	// With no profiles to choose from, skip straight to the package checklist.
	if len(profiles) == 0 {
		m.mode = modePackages
	}
	return m
}

func (m model) Init() tea.Cmd { return nil }

// rowCount returns the number of navigable rows in the current mode.
func (m model) rowCount() int {
	if m.mode == modeProfile {
		return len(m.profiles)
	}
	return len(m.items)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch km.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyUp:
		return m.moveCursor(-1), nil
	case tea.KeyDown:
		return m.moveCursor(1), nil
	case tea.KeySpace:
		if m.mode == modePackages && m.cursor < len(m.items) {
			name := m.items[m.cursor].Name
			m.selected[name] = !m.selected[name]
		}
		return m, nil
	case tea.KeyEnter:
		return m.enter()
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "q":
			return m, tea.Quit
		case "j":
			return m.moveCursor(1), nil
		case "k":
			return m.moveCursor(-1), nil
		}
	}
	return m, nil
}

func (m model) moveCursor(delta int) model {
	c := m.cursor + delta
	if c < 0 {
		c = 0
	}
	if n := m.rowCount(); n > 0 && c > n-1 {
		c = n - 1
	}
	m.cursor = c
	return m
}

func (m model) enter() (tea.Model, tea.Cmd) {
	if m.mode == modeProfile {
		if m.cursor < len(m.profiles) {
			p := m.profiles[m.cursor]
			m.profile = p.Name
			for _, name := range p.Packages {
				m.selected[name] = true
			}
		}
		m.mode = modePackages
		m.cursor = 0
		return m, nil
	}
	// modePackages: confirm and quit.
	m.done = true
	return m, tea.Quit
}

// isChecked reports whether the named item is currently selected.
func (m model) isChecked(name string) bool { return m.selected[name] }

// result computes the final Result, with Packages sorted.
func (m model) result() Result {
	var pkgs []string
	for _, it := range m.items {
		if m.selected[it.Name] {
			pkgs = append(pkgs, it.Name)
		}
	}
	sort.Strings(pkgs)
	return Result{
		Profile:   m.profile,
		Packages:  pkgs,
		Confirmed: m.done,
	}
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).MarginBottom(1)
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	checkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	descStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	selRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).MarginTop(1)
)

func (m model) View() string {
	var b []string

	if m.mode == modeProfile {
		b = append(b, titleStyle.Render("Choose a profile"))
		for i, p := range m.profiles {
			b = append(b, m.renderRow(i, "", p.Name, p.Description, false))
		}
		b = append(b, helpStyle.Render("↑/↓ move · enter choose · q quit"))
	} else {
		b = append(b, titleStyle.Render("Select packages"))
		for i, it := range m.items {
			b = append(b, m.renderRow(i, "", it.Name, it.Description, m.selected[it.Name]))
		}
		b = append(b, helpStyle.Render("space toggle · enter confirm · q quit"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, b...) + "\n"
}

func (m model) renderRow(i int, _ string, name, desc string, checked bool) string {
	cursor := "  "
	if i == m.cursor {
		cursor = cursorStyle.Render("❯ ")
	}

	var box string
	if m.mode == modePackages {
		if checked {
			box = checkStyle.Render("[x] ")
		} else {
			box = "[ ] "
		}
	}

	label := name
	if i == m.cursor {
		label = selRowStyle.Render(name)
	}

	row := cursor + box + label
	if desc != "" {
		row += "  " + descStyle.Render("— "+desc)
	}
	return row
}

// Run drives the interactive picker (profile choice → package checklist) on the
// terminal and blocks until the user confirms or quits. TTY-only; not unit-tested.
func Run(profiles []Profile, items []Item) (Result, error) {
	p := tea.NewProgram(newModel(profiles, items))
	final, err := p.Run()
	if err != nil {
		return Result{}, err
	}
	return final.(model).result(), nil
}
