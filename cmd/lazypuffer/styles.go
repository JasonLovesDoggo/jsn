package main

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent  = lipgloss.Color("212")
	colorMuted   = lipgloss.Color("245")
	colorBorder  = lipgloss.Color("237")
	colorWarning = lipgloss.Color("203")

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	subtleStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("57"))

	focusedBorder = lipgloss.NewStyle().
			Border(lipgloss.Border{
			Top:          "-",
			Bottom:       "-",
			Left:         "|",
			Right:        "|",
			TopLeft:      "+",
			TopRight:     "+",
			BottomLeft:   "+",
			BottomRight:  "+",
			MiddleLeft:   "|",
			MiddleRight:  "|",
			MiddleTop:    "-",
			MiddleBottom: "-",
		}).
		BorderForeground(colorBorder).
		AlignVertical(lipgloss.Top).
		Padding(0, 1)

	blurredBorder = lipgloss.NewStyle().
			Border(lipgloss.Border{
			Top:          "-",
			Bottom:       "-",
			Left:         "|",
			Right:        "|",
			TopLeft:      "+",
			TopRight:     "+",
			BottomLeft:   "+",
			BottomRight:  "+",
			MiddleLeft:   "|",
			MiddleRight:  "|",
			MiddleTop:    "-",
			MiddleBottom: "-",
		}).
		BorderForeground(colorBorder).
		AlignVertical(lipgloss.Top).
		Padding(0, 1)
)
