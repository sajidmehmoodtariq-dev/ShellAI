package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	App             lipgloss.Style
	Title           lipgloss.Style
	Subtitle        lipgloss.Style
	Prompt          lipgloss.Style
	Hint            lipgloss.Style
	WelcomeItem     lipgloss.Style
	MainResult      lipgloss.Style
	Alternative     lipgloss.Style
	Command         lipgloss.Style
	OutputLine      lipgloss.Style
	ErrorLine       lipgloss.Style
	OutputPane      lipgloss.Style
	SafeBox         lipgloss.Style
	WarningBox      lipgloss.Style
	DangerBox       lipgloss.Style
	DangerTitle     lipgloss.Style
	SectionTitle    lipgloss.Style
	Spinner         lipgloss.Style
	Error           lipgloss.Style
	Dim             lipgloss.Style
	InputLabel      lipgloss.Style
	ConfirmPrompt   lipgloss.Style
	InputNeutral    lipgloss.Style
	InputRecognized lipgloss.Style
	InputConfident  lipgloss.Style
	InputUnsure     lipgloss.Style
	ConfidenceHigh  lipgloss.Style
	ConfidenceMid   lipgloss.Style
	ConfidenceLow   lipgloss.Style
	ExplainPane     lipgloss.Style
	ExplainLabel    lipgloss.Style
	StatusBar       lipgloss.Style
	StatusMode      lipgloss.Style
	StatusOK        lipgloss.Style
	StatusMuted     lipgloss.Style
}

func NewTheme() Theme {
	accent := lipgloss.Color("#3A86FF")
	ok := lipgloss.Color("#2A9D8F")
	warn := lipgloss.Color("#F4A261")
	danger := lipgloss.Color("#E63946")
	muted := lipgloss.Color("#7A7F87")

	return Theme{
		App:         lipgloss.NewStyle().Padding(1, 2),
		Title:       lipgloss.NewStyle().Bold(true).Foreground(accent),
		Subtitle:    lipgloss.NewStyle().Foreground(muted),
		Prompt:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		Hint:        lipgloss.NewStyle().Foreground(muted),
		WelcomeItem: lipgloss.NewStyle().Foreground(lipgloss.Color("#DDE3EA")),

		MainResult: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1),

		Alternative: lipgloss.NewStyle().
			Foreground(muted).
			Italic(true).
			Padding(0, 1),

		Command: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EDEDED")).
			Background(lipgloss.Color("#20242B")).
			Padding(0, 1),

		OutputLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D7D7D7")),

		ErrorLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB3BA")),

		OutputPane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4B5563")).
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
		InputNeutral: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4B5563")).
			Padding(0, 1),
		InputRecognized: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1),
		InputConfident: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ok).
			Padding(0, 1),
		InputUnsure: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(warn).
			Padding(0, 1),
		ConfidenceHigh: lipgloss.NewStyle().Foreground(ok),
		ConfidenceMid:  lipgloss.NewStyle().Foreground(warn),
		ConfidenceLow:  lipgloss.NewStyle().Foreground(danger),
		ExplainPane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1),
		ExplainLabel: lipgloss.NewStyle().Foreground(accent).Bold(true),
		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#1A1F26")).
			Foreground(lipgloss.Color("#C9D1D9")).
			Padding(0, 1),
		StatusMode:  lipgloss.NewStyle().Foreground(accent).Bold(true),
		StatusOK:    lipgloss.NewStyle().Foreground(ok),
		StatusMuted: lipgloss.NewStyle().Foreground(muted),
	}
}
