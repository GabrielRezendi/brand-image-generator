package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorBrand  = lipgloss.Color("#E8D5A3")
	colorAccent = lipgloss.Color("#2DD4BF")
	colorMuted  = lipgloss.Color("#8B9A9E")
	colorError  = lipgloss.Color("#F07178")
	colorOK     = lipgloss.Color("#7FD99A")
	colorBorder = lipgloss.Color("#3A4A4E")

	brandStyle = lipgloss.NewStyle().
			Foreground(colorBrand).
			Bold(true)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError)

	okStyle = lipgloss.NewStyle().
			Foreground(colorOK)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)
