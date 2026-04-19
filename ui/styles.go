package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	App           lipgloss.Style
	Title         lipgloss.Style
	Subtitle      lipgloss.Style
	Prompt        lipgloss.Style
	Hint          lipgloss.Style
	MainResult    lipgloss.Style
	Alternative   lipgloss.Style
	Command       lipgloss.Style
	Output        lipgloss.Style
	SafeBox       lipgloss.Style
	WarningBox    lipgloss.Style
	DangerBox     lipgloss.Style
	DangerTitle   lipgloss.Style
	SectionTitle  lipgloss.Style
	Spinner       lipgloss.Style
	Error         lipgloss.Style
	Dim           lipgloss.Style
	InputLabel    lipgloss.Style
	ConfirmPrompt lipgloss.Style
}

func NewTheme() Theme {
	accent := lipgloss.Color("#3A86FF")
	ok := lipgloss.Color("#2A9D8F")
	warn := lipgloss.Color("#F4A261")
	danger := lipgloss.Color("#E63946")
	muted := lipgloss.Color("#7A7F87")

	return Theme{
		App:      lipgloss.NewStyle().Padding(1, 2),
		Title:    lipgloss.NewStyle().Bold(true).Foreground(accent),
		Subtitle: lipgloss.NewStyle().Foreground(muted),
		Prompt:   lipgloss.NewStyle().Foreground(accent).Bold(true),
		Hint:     lipgloss.NewStyle().Foreground(muted),

		MainResult: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1),

		Alternative: lipgloss.NewStyle().
			Foreground(muted).
			Padding(0, 1),

		Command: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EDEDED")).
			Background(lipgloss.Color("#20242B")).
			Padding(0, 1),

		Output: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D7D7D7")).
			Background(lipgloss.Color("#111317")).
			Padding(0, 1),

		SafeBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ok).
			Foreground(ok).
			Padding(0, 1),

		WarningBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(warn).
			Foreground(warn).
			Padding(0, 1),

		DangerBox: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(danger).
			Foreground(lipgloss.Color("#FFE9EC")).
			Background(lipgloss.Color("#3B0E13")).
			Padding(0, 1),

		DangerTitle:   lipgloss.NewStyle().Bold(true).Foreground(danger),
		SectionTitle:  lipgloss.NewStyle().Bold(true).Foreground(accent),
		Spinner:       lipgloss.NewStyle().Foreground(accent),
		Error:         lipgloss.NewStyle().Foreground(danger).Bold(true),
		Dim:           lipgloss.NewStyle().Foreground(muted),
		InputLabel:    lipgloss.NewStyle().Foreground(accent).Bold(true),
		ConfirmPrompt: lipgloss.NewStyle().Foreground(warn).Bold(true),
	}
}
