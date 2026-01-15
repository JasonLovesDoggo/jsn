package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type setupModel struct {
	path   string
	err    error
	width  int
	height int
}

func newSetupModel(path string, err error) setupModel {
	return setupModel{path: path, err: err}
}

func (m setupModel) Init() tea.Cmd {
	return nil
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m setupModel) View() string {
	title := headerStyle.Render("lazypuffer setup")
	errLine := errorStyle.Render(fmt.Sprintf("config error: %v", m.err))
	hint := subtleStyle.Render("Create a config file, then rerun lazypuffer.")
	path := subtleStyle.Render(fmt.Sprintf("path: %s", m.path))
	template := ConfigTemplate(m.path, Profile{})

	body := strings.Join([]string{
		title,
		"",
		errLine,
		hint,
		path,
		"",
		template,
		"",
		subtleStyle.Render("Press q to quit"),
	}, "\n")

	if m.width == 0 || m.height == 0 {
		return body
	}
	style := lipgloss.NewStyle().Width(m.width).Height(m.height)
	return style.Render(body)
}
