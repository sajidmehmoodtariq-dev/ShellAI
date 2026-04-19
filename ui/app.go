package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"shellai/internal/config"
	"shellai/internal/executor"
	"shellai/internal/feedback"
	"shellai/internal/llm"
	"shellai/internal/parser"
	"shellai/internal/safety"
	"shellai/internal/search"
)

type appState int

const (
	stateInput appState = iota
	stateSearching
	stateResult
	stateConfirm
	stateRunning
	stateExplain
)

type searchDoneMsg struct {
	intent  parser.ParsedIntent
	matches []search.ScoredMatch
	query   string
}

type appErrMsg struct{ err error }

type runStreamMsg struct {
	stderr bool
	line   string
}

type runDoneMsg struct {
	result executor.RunResult
	err    error
}

type explainTokenMsg struct{ token string }
type explainDoneMsg struct{ err error }

type model struct {
	state      appState
	returnTo   appState
	theme      Theme
	width      int
	height     int
	spinner    spinner.Model
	input      textinput.Model
	confirm    textinput.Model
	search     *search.Engine
	tmpl       *executor.TemplateEngine
	runner     *executor.Runner
	explainer  llm.Explainer
	markdown   *glamour.TermRenderer
	errText    string
	query      string
	intent     parser.ParsedIntent
	matches    []search.ScoredMatch
	selected   int
	build      executor.BuildResult
	finalCmd   string
	missing    []string
	manualVals map[string]string
	safety     safety.Assessment
	runOut     []string
	runErr     []string
	runDone    bool
	runStatus  string
	runCh      chan tea.Msg
	feedback   *feedback.Store
	explainRaw string
	explainMD  string
	explainEnd bool
	explainErr string
	explainCh  chan tea.Msg
}

var unresolvedPlaceholder = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

func NewModel() (model, error) {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	in := textinput.New()
	in.Placeholder = "Describe what you want to do, then press Enter"
	in.Focus()
	in.Prompt = "> "
	in.CharLimit = 512
	in.Width = 80

	cf := textinput.New()
	cf.Prompt = "> "
	cf.CharLimit = 256
	cf.Width = 60

	renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(90))

	userDB := config.UserCommandsPath()
	eng, err := search.NewEngineFromDatabases(config.CommandsPath(), userDB)
	if err != nil {
		return model{}, err
	}

	explainer, err := llm.NewExplainer(context.Background(), llm.AutoOptions{})
	if err != nil {
		return model{}, err
	}

	return model{
		state:      stateInput,
		theme:      NewTheme(),
		spinner:    sp,
		input:      in,
		confirm:    cf,
		search:     eng,
		tmpl:       executor.NewTemplateEngine(),
		runner:     executor.NewRunner(),
		feedback:   feedback.NewStore(),
		explainer:  explainer,
		markdown:   renderer,
		manualVals: map[string]string{},
	}, nil
}

func Run() error {
	m, err := NewModel()
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(40, msg.Width-10)
		m.confirm.Width = max(30, msg.Width-16)
		m.refreshRenderer()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.state == stateSearching {
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	}

	switch m.state {
	case stateInput:
		return m.updateInput(msg)
	case stateSearching:
		return m.updateSearching(msg)
	case stateResult:
		return m.updateResult(msg)
	case stateConfirm:
		return m.updateConfirm(msg)
	case stateRunning:
		return m.updateRunning(msg)
	case stateExplain:
		return m.updateExplain(msg)
	}

	return m, nil
}

func (m model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "enter" {
			query := strings.TrimSpace(m.input.Value())
			if query == "" {
				return m, nil
			}
			m.errText = ""
			m.query = query
			m.state = stateSearching
			return m, tea.Batch(m.spinner.Tick, searchCmd(m.search, query))
		}
	}

	return m, cmd
}

func (m model) updateSearching(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case searchDoneMsg:
		if err := m.feedback.RecordMatches(msg.matches); err != nil {
			m.errText = err.Error()
		}
		if len(msg.matches) == 0 {
			m.state = stateInput
			m.errText = "No matching command found. Try a clearer request."
			return m, nil
		}
		m.intent = msg.intent
		m.matches = msg.matches
		m.selected = 0
		m.manualVals = map[string]string{}
		m.rebuildSelection()
		m.state = stateResult
		return m, nil

	case appErrMsg:
		m.state = stateInput
		m.errText = msg.err.Error()
		return m, nil
	}

	return m, nil
}

func (m model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "1", "2", "3":
		idx := int(key.String()[0] - '1')
		if idx >= 0 && idx < len(m.matches) {
			m.selected = idx
			m.manualVals = map[string]string{}
			m.rebuildSelection()
		}
		return m, nil

	case "enter":
		m.state = stateConfirm
		m.prepareConfirmInput()
		return m, nil

	case "e":
		m.returnTo = stateResult
		m.state = stateExplain
		m.explainRaw = ""
		m.explainMD = ""
		m.explainErr = ""
		m.explainEnd = false
		m.explainCh = make(chan tea.Msg, 256)
		return m, tea.Batch(startExplainCmd(m.explainer, m.finalCmd, m.explainCh), waitForChan(m.explainCh))

	case "esc":
		m.state = stateInput
		m.input.SetValue("")
		return m, nil
	}

	return m, nil
}

func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.missing) > 0 {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "enter":
				placeholder := m.missing[0]
				val := strings.TrimSpace(m.confirm.Value())
				if val == "" {
					m.errText = "Value cannot be empty for {" + placeholder + "}."
					return m, nil
				}
				m.manualVals[placeholder] = val
				m.confirm.SetValue("")
				m.errText = ""
				m.rebuildSelection()
				m.prepareConfirmInput()
				return m, nil
			case "esc":
				m.state = stateResult
				return m, nil
			}
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, cmd
	}

	switch key.String() {
	case "y", "enter":
		if m.safety.Level == safety.LevelDangerous {
			if strings.TrimSpace(strings.ToLower(m.confirm.Value())) != "yes" {
				m.errText = "Dangerous command requires typing yes exactly."
				return m, nil
			}
		}
		m.errText = ""
		m.runOut = nil
		m.runErr = nil
		m.runDone = false
		m.runStatus = "Running command..."
		m.state = stateRunning
		m.runCh = make(chan tea.Msg, 512)
		return m, tea.Batch(startRunCmd(m.runner, m.finalCmd, m.runCh), waitForChan(m.runCh))

	case "n", "esc":
		m.state = stateResult
		m.confirm.SetValue("")
		return m, nil

	case "e":
		m.returnTo = stateConfirm
		m.state = stateExplain
		m.explainRaw = ""
		m.explainMD = ""
		m.explainErr = ""
		m.explainEnd = false
		m.explainCh = make(chan tea.Msg, 256)
		return m, tea.Batch(startExplainCmd(m.explainer, m.finalCmd, m.explainCh), waitForChan(m.explainCh))
	}

	return m, cmd
}

func (m model) updateRunning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runStreamMsg:
		if msg.stderr {
			m.runErr = append(m.runErr, msg.line)
		} else {
			m.runOut = append(m.runOut, msg.line)
		}
		return m, waitForChan(m.runCh)

	case runDoneMsg:
		m.runDone = true
		if msg.err != nil {
			m.runStatus = fmt.Sprintf("Command finished with exit code %d", msg.result.ExitCode)
			m.errText = msg.err.Error()
		} else {
			m.runStatus = "Command finished successfully"
		}
		return m, nil

	case tea.KeyMsg:
		if !m.runDone {
			return m, nil
		}

		switch msg.String() {
		case "y":
			m.state = stateInput
			m.input.SetValue("")
			m.confirm.SetValue("")
			return m, nil
		case "n":
			if err := m.feedback.RecordMiss(m.query, m.finalCmd); err != nil {
				m.errText = err.Error()
			}
			m.state = stateInput
			m.input.SetValue("")
			m.confirm.SetValue("")
			return m, nil
		case "enter", "esc":
			m.errText = "Rate result first: y = correct, n = incorrect."
			return m, nil
		}
	}

	return m, nil
}

func (m model) updateExplain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case explainTokenMsg:
		m.explainRaw += msg.token
		m.renderExplainMarkdown()
		return m, waitForChan(m.explainCh)

	case explainDoneMsg:
		m.explainEnd = true
		if msg.err != nil {
			m.explainErr = msg.err.Error()
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "esc" || (m.explainEnd && msg.String() == "enter") {
			m.state = m.returnTo
			return m, nil
		}
	}

	return m, nil
}

func (m *model) rebuildSelection() {
	if len(m.matches) == 0 || m.selected < 0 || m.selected >= len(m.matches) {
		return
	}
	entry := m.matches[m.selected].Entry
	m.build = m.tmpl.Build(entry.CommandTemplate, m.intent)
	m.finalCmd, m.missing = applyManualValues(m.build.Command, m.build.MissingPlaceholders, m.manualVals)
	if len(m.missing) == 0 {
		m.safety = safety.Analyze(m.finalCmd)
	} else {
		m.safety = safety.Assessment{
			Level:        safety.LevelWarning,
			Confirmation: safety.ConfirmExplicit,
			Reasons:      []string{"Some required command values are still missing."},
		}
	}
}

func (m *model) prepareConfirmInput() {
	m.confirm.SetValue("")
	m.confirm.Focus()
	if len(m.missing) > 0 {
		m.confirm.Placeholder = "Enter value for {" + m.missing[0] + "}"
		return
	}
	if m.safety.Level == safety.LevelDangerous {
		m.confirm.Placeholder = "Type yes to continue"
	} else {
		m.confirm.Placeholder = "Press enter to continue or n to cancel"
	}
}

func (m *model) renderExplainMarkdown() {
	if m.markdown == nil {
		m.explainMD = m.explainRaw
		return
	}
	rendered, err := m.markdown.Render(m.explainRaw)
	if err != nil {
		m.explainMD = m.explainRaw
		return
	}
	m.explainMD = rendered
}

func (m *model) refreshRenderer() {
	if m.width <= 0 {
		return
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(max(60, m.width-8)),
	)
	if err == nil {
		m.markdown = r
		m.renderExplainMarkdown()
	}
}

func (m model) View() string {
	var body string
	switch m.state {
	case stateInput:
		body = m.viewInput()
	case stateSearching:
		body = m.viewSearching()
	case stateResult:
		body = m.viewResult()
	case stateConfirm:
		body = m.viewConfirm()
	case stateRunning:
		body = m.viewRunning()
	case stateExplain:
		body = m.viewExplain()
	}
	return m.theme.App.Render(body)
}

func (m model) viewInput() string {
	parts := []string{
		m.theme.Title.Render("ShellAI"),
		m.theme.Subtitle.Render("Intent-first shell assistant"),
		"",
		m.theme.InputLabel.Render("Input"),
		m.input.View(),
		m.theme.Hint.Render("Enter: search command  |  Ctrl+C: quit"),
	}
	if m.errText != "" {
		parts = append(parts, "", m.theme.Error.Render(m.errText))
	}
	return strings.Join(parts, "\n")
}

func (m model) viewSearching() string {
	return strings.Join([]string{
		m.theme.Title.Render("Searching"),
		m.theme.Spinner.Render(m.spinner.View() + " Matching command templates..."),
		m.theme.Dim.Render("Query: " + m.query),
	}, "\n")
}

func (m model) viewResult() string {
	if len(m.matches) == 0 {
		return m.theme.Error.Render("No results")
	}
	main := m.matches[m.selected]
	mainBlock := strings.Join([]string{
		m.theme.SectionTitle.Render("Selected Command"),
		m.theme.Command.Render(m.finalCmd),
		m.theme.Subtitle.Render(main.Entry.Explanation),
	}, "\n")

	alt := make([]string, 0, 2)
	for i, item := range m.matches {
		if i == m.selected {
			continue
		}
		alt = append(alt, m.theme.Alternative.Render(fmt.Sprintf("%d) %s  (%0.3f)", i+1, item.Entry.CommandTemplate, item.Score)))
		if len(alt) == 2 {
			break
		}
	}

	parts := []string{
		m.theme.Title.Render("Result"),
		m.theme.MainResult.Render(mainBlock),
		m.theme.SectionTitle.Render("Alternatives"),
	}
	if len(alt) == 0 {
		parts = append(parts, m.theme.Dim.Render("No alternatives available."))
	} else {
		parts = append(parts, strings.Join(alt, "\n"))
	}
	parts = append(parts,
		"",
		m.theme.Hint.Render("1/2/3: choose  |  Enter: continue  |  e: explain  |  Esc: back"),
	)
	if m.errText != "" {
		parts = append(parts, m.theme.Error.Render(m.errText))
	}

	return strings.Join(parts, "\n")
}

func (m model) viewConfirm() string {
	levelBox := m.renderSafetyBox()
	parts := []string{
		m.theme.Title.Render("Confirm"),
		m.theme.Command.Render(m.finalCmd),
		levelBox,
	}

	if len(m.missing) > 0 {
		parts = append(parts,
			m.theme.ConfirmPrompt.Render("Missing value for {"+m.missing[0]+"}."),
			m.confirm.View(),
			m.theme.Hint.Render("Enter value  |  Esc: back"),
		)
	} else if m.safety.Level == safety.LevelDangerous {
		parts = append(parts,
			m.theme.DangerTitle.Render("Type yes explicitly to run this command."),
			m.confirm.View(),
			m.theme.Hint.Render("Enter: submit  |  n/Esc: cancel  |  e: explain"),
		)
	} else {
		parts = append(parts,
			m.theme.Hint.Render("Enter/y: run  |  n/Esc: cancel  |  e: explain"),
		)
	}

	if m.errText != "" {
		parts = append(parts, m.theme.Error.Render(m.errText))
	}

	return strings.Join(parts, "\n")
}

func (m model) renderSafetyBox() string {
	reasonText := strings.Join(m.safety.Reasons, "\n")
	impactText := strings.Join(m.safety.WhatCouldGoWrong, "\n")

	content := strings.TrimSpace(strings.Join([]string{
		"Level: " + string(m.safety.Level),
		reasonText,
		impactText,
	}, "\n"))

	switch m.safety.Level {
	case safety.LevelDangerous:
		return m.theme.DangerBox.Render(content)
	case safety.LevelWarning:
		return m.theme.WarningBox.Render(content)
	default:
		return m.theme.SafeBox.Render(content)
	}
}

func (m model) viewRunning() string {
	parts := []string{
		m.theme.Title.Render("Running"),
		m.theme.Dim.Render(m.runStatus),
		m.theme.Command.Render(m.finalCmd),
		"",
		m.theme.SectionTitle.Render("Output"),
		m.theme.Output.Render(strings.Join(m.runOut, "\n")),
	}
	if len(m.runErr) > 0 {
		parts = append(parts, m.theme.SectionTitle.Render("Errors"), m.theme.Output.Render(strings.Join(m.runErr, "\n")))
	}
	if m.runDone {
		parts = append(parts, m.theme.Hint.Render("Was this correct? y: yes  |  n: no (logs miss)"))
	}
	if m.errText != "" {
		parts = append(parts, m.theme.Error.Render(m.errText))
	}
	return strings.Join(parts, "\n")
}

func (m model) viewExplain() string {
	parts := []string{
		m.theme.Title.Render("Explain"),
		m.theme.Dim.Render("Provider: " + m.explainer.ProviderName()),
	}
	if strings.TrimSpace(m.explainMD) == "" {
		parts = append(parts, m.theme.Spinner.Render(m.spinner.View()+" Generating explanation..."))
	} else {
		parts = append(parts, m.explainMD)
	}
	if m.explainErr != "" {
		parts = append(parts, m.theme.Error.Render(m.explainErr))
	}
	if m.explainEnd {
		parts = append(parts, m.theme.Hint.Render("Enter/Esc: back"))
	}
	return strings.Join(parts, "\n")
}

func searchCmd(engine *search.Engine, query string) tea.Cmd {
	return func() tea.Msg {
		intent := parser.ParseIntent(query)
		matches := engine.Search(intent, 3)
		return searchDoneMsg{intent: intent, matches: matches, query: query}
	}
}

func startRunCmd(runner *executor.Runner, command string, ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			res, err := runner.Run(
				context.Background(),
				command,
				func(ev executor.StreamEvent) { ch <- runStreamMsg{stderr: false, line: ev.Data} },
				func(ev executor.StreamEvent) { ch <- runStreamMsg{stderr: true, line: ev.Data} },
			)
			ch <- runDoneMsg{result: res, err: err}
			close(ch)
		}()
		return nil
	}
}

func startExplainCmd(explainer llm.Explainer, command string, ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			err := explainer.Explain(
				context.Background(),
				llm.ExplainRequest{Command: command},
				func(token string) error {
					ch <- explainTokenMsg{token: token}
					return nil
				},
			)
			ch <- explainDoneMsg{err: err}
			close(ch)
		}()
		return nil
	}
}

func waitForChan(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func applyManualValues(command string, missing []string, values map[string]string) (string, []string) {
	resolved := command
	for key, val := range values {
		resolved = strings.ReplaceAll(resolved, "{"+key+"}", val)
	}

	stillMissing := make([]string, 0, len(missing))
	for _, name := range missing {
		if _, ok := values[name]; !ok {
			stillMissing = append(stillMissing, name)
		}
	}

	for _, match := range unresolvedPlaceholder.FindAllStringSubmatch(resolved, -1) {
		if len(match) == 2 {
			name := match[1]
			if !contains(stillMissing, name) {
				stillMissing = append(stillMissing, name)
			}
		}
	}

	return resolved, stillMissing
}

func contains(items []string, target string) bool {
	for _, it := range items {
		if it == target {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
