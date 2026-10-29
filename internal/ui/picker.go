// Package ui provides flint's interactive terminal UI built on the Charm
// suite (Bubble Tea, Bubbles, Lip Gloss). It is used when flint is run
// interactively without a profile argument.
package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrAborted is returned when the user cancels the picker (Esc or Ctrl+C).
var ErrAborted = fmt.Errorf("selection aborted")

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FDFD96")). // flint-spark yellow
			Bold(true).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF8C42")). // ember orange
			Bold(true)

	itemStyle = lipgloss.NewStyle().PaddingLeft(2)
)

// profileItem adapts a profile name to the list.Item / list.DefaultItem API.
type profileItem string

func (p profileItem) Title() string       { return string(p) }
func (p profileItem) Description() string { return "" }
func (p profileItem) FilterValue() string { return string(p) }

// pickerModel is the Bubble Tea model wrapping the list component.
type pickerModel struct {
	list     list.Model
	choice   string
	aborted  bool
	quitting bool
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Leave room for the title and help. Keep it compact.
		h := msg.Height - 4
		if h > 14 {
			h = 14
		}
		if h < 3 {
			h = 3
		}
		m.list.SetSize(msg.Width, h)
		return m, nil

	case tea.KeyMsg:
		// Let the list handle keys while filtering (so "q"/esc edit the filter).
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.aborted = true
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if it, ok := m.list.SelectedItem().(profileItem); ok {
				m.choice = string(it)
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m pickerModel) View() string {
	if m.quitting {
		return ""
	}
	return "\n" + m.list.View()
}

// newPickerModel builds the picker's Bubble Tea model for the given profiles.
// Split out from PickProfile so it can be driven directly in tests.
func newPickerModel(profiles []string) pickerModel {
	items := make([]list.Item, len(profiles))
	for i, p := range profiles {
		items[i] = profileItem(p)
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = selectedStyle
	delegate.Styles.NormalTitle = itemStyle

	l := list.New(items, delegate, 0, 0)
	l.Title = "Select an AWS profile"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings() // we handle q/esc/ctrl+c ourselves

	return pickerModel{list: l}
}

// PickProfile shows an interactive, filterable picker for the given profiles
// and returns the chosen one. The UI is rendered to out (callers should pass
// stderr so stdout stays clean for `eval`). It returns ErrAborted if the user
// cancels.
func PickProfile(in io.Reader, out io.Writer, profiles []string) (string, error) {
	prog := tea.NewProgram(
		newPickerModel(profiles),
		tea.WithInput(in),
		tea.WithOutput(out),
	)
	final, err := prog.Run()
	if err != nil {
		return "", err
	}

	fm := final.(pickerModel)
	if fm.aborted || fm.choice == "" {
		return "", ErrAborted
	}
	return fm.choice, nil
}
