package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	stateWelcome appState = iota
	stateInput
	stateSearching
	stateResult
	stateConfirm
	stateRunning
	stateExplain
)

type searchProgressMsg struct {
	text string
	ch   <-chan tea.Msg
}

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

var welcomeExamples = []string{
	"copy files to usb",
	"find logs older than 7 days",
	"create a tar archive from project folder",
}

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
	runScroll  int
	feedback   *feedback.Store
	explainRaw string
	explainMD  string
	explainEnd bool
	explainErr string
	explainCh  chan tea.Msg

	searchStatus    string
	inputSignal     string
	welcomeIndex    int
	llmAvailable    bool
	commandCount    int
	supportsColor   bool
	supportsUnicode bool
}

var unresolvedPlaceholder = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

func NewModel() (model, error) {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot

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
	supportsColor := strings.TrimSpace(os.Getenv("NO_COLOR")) == ""
	supportsUnicode := strings.ToLower(strings.TrimSpace(os.Getenv("TERM"))) != "dumb"
	if !supportsUnicode {
		sp.Spinner = spinner.Line
	}

	userDB := config.UserCommandsPath()
	eng, err := search.NewEngineFromDatabases(config.CommandsPath(), userDB)
	if err != nil {
		return model{}, err
	}

	explainer, err := llm.NewExplainer(context.Background(), llm.AutoOptions{})
	if err != nil {
		explainer = &llm.FallbackExplainer{}
	}
	llmAvailable := explainer.ProviderName() != "fallback"

	startState := stateInput
	if shouldShowWelcome() {
		startState = stateWelcome
	}

	return model{
		state:           startState,
		theme:           NewTheme(),
		spinner:         sp,
		input:           in,
		confirm:         cf,
		search:          eng,
		tmpl:            executor.NewTemplateEngine(),
		runner:          executor.NewRunner(),
		feedback:        feedback.NewStore(),
		explainer:       explainer,
		markdown:        renderer,
		manualVals:      map[string]string{},
		searchStatus:    "Ready",
		inputSignal:     "neutral",
		llmAvailable:    llmAvailable,
		commandCount:    eng.EntryCount(),
		supportsColor:   supportsColor,
		supportsUnicode: supportsUnicode,
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
	if m.state == stateSearching {
		return tea.Batch(textinput.Blink, m.spinner.Tick)
	}
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
	case stateWelcome:
		return m.updateWelcome(msg)
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
	m.inputSignal = inferInputSignal(m.input.Value())

	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "enter" {
			query := strings.TrimSpace(m.input.Value())
			if query == "" {
				return m, nil
			}
			m.errText = ""
			m.query = query
			m.state = stateSearching
			m.searchStatus = "parsing intent"
			return m, tea.Batch(m.spinner.Tick, startSearchCmd(m.search, query))
		}
	}

	return m, cmd
}

func (m model) updateWelcome(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "up", "k":
		if m.welcomeIndex > 0 {
			m.welcomeIndex--
		}
	case "down", "j":
		if m.welcomeIndex < len(welcomeExamples)-1 {
			m.welcomeIndex++
		}
	case "enter":
		if len(welcomeExamples) > 0 {
			m.input.SetValue(welcomeExamples[m.welcomeIndex])
		}
		_ = markWelcomeSeen()
		m.state = stateInput
		m.input.Focus()
	case "esc":
		_ = markWelcomeSeen()
		m.state = stateInput
		m.input.Focus()
	}

	return m, nil
}

func (m model) updateSearching(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case searchProgressMsg:
		m.searchStatus = msg.text
		return m, waitForChan(msg.ch)

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
		m.searchStatus = "ready"
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
		m.runScroll = 0
		m.runStatus = "Running command..."
		m.state = stateRunning
		m.runCh = make(chan tea.Msg, 512)
		return m, tea.Batch(startRunCmd(m.runner, m.finalCmd, m.runCh), waitForChan(m.runCh))

	case "n", "esc":
		if m.safety.Level == safety.LevelDangerous {
			m.state = stateResult
		} else {
			m.state = stateInput
			m.input.SetValue(m.query)
			m.input.Focus()
		}
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
		total := len(m.runOut) + len(m.runErr)
		maxScroll := max(0, total-1)
		switch msg.String() {
		case "up", "k":
			if m.runScroll > 0 {
				m.runScroll--
			}
			return m, nil
		case "down", "j":
			if m.runScroll < maxScroll {
				m.runScroll++
			}
			return m, nil
		case "pgup":
			m.runScroll = max(0, m.runScroll-8)
			return m, nil
		case "pgdown":
			m.runScroll = min(maxScroll, m.runScroll+8)
			return m, nil
		}

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
	case stateWelcome:
		body = m.viewWelcome()
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
	renderedBody := m.theme.App.Render(body)
	status := m.renderStatusBar()
	if m.height <= 0 {
		return renderedBody + "\n" + status
	}
	bodyLines := strings.Count(renderedBody, "\n") + 1
	fill := m.height - bodyLines - 1
	if fill < 0 {
		fill = 0
	}
	return renderedBody + strings.Repeat("\n", fill) + "\n" + status
}

func (m model) viewInput() string {
	promptBox := m.renderPromptBox()
	parts := []string{
		m.theme.Title.Render("ShellAI"),
		m.theme.Subtitle.Render("Intent-first shell assistant"),
		"",
		m.theme.InputLabel.Render("Input"),
		promptBox,
		m.theme.Hint.Render("Enter: search command  |  Ctrl+C: quit"),
	}
	if m.errText != "" {
		parts = append(parts, "", m.theme.Error.Render(m.errText))
	}
	return strings.Join(parts, "\n")
}

func (m model) viewSearching() string {
	text := m.searchStatus
	if text == "" {
		text = "parsing intent"
	}
	return strings.Join([]string{
		m.theme.Title.Render("Searching"),
		m.theme.Spinner.Render(m.spinner.View() + " " + text),
		m.theme.Dim.Render("Query: " + m.query),
	}, "\n")
}

func (m model) viewResult() string {
	if len(m.matches) == 0 {
		return m.theme.Error.Render("No results")
	}
	main := m.matches[m.selected]
	mainCmd := m.finalCmd
	if m.safety.Level == safety.LevelDangerous {
		mainCmd = highlightDangerous(mainCmd)
	}
	confidence := m.renderConfidence(main.Score)
	flags := renderFlags(main.Entry.Flags)
	mainBlock := strings.Join([]string{
		m.theme.SectionTitle.Render("Selected Command  " + confidence),
		m.theme.Command.Render(mainCmd),
		m.theme.Subtitle.Render(main.Entry.Explanation),
		flags,
	}, "\n")

	alt := make([]string, 0, 2)
	for i, item := range m.matches {
		if i == m.selected {
			continue
		}
		alt = append(alt, m.theme.Alternative.Render(fmt.Sprintf("%d) %s  %s", i+1, item.Entry.CommandTemplate, m.renderConfidence(item.Score))))
		if len(alt) == 2 {
			break
		}
	}

	card := m.theme.MainResult
	if m.safety.Level == safety.LevelDangerous {
		card = m.theme.DangerBox
	}

	parts := []string{
		m.theme.Title.Render("Result"),
		card.Render(mainBlock),
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
			m.theme.DangerBox.Render(m.confirm.View()),
			m.theme.Hint.Render("Enter: submit  |  n/Esc: cancel  |  e: explain"),
		)
	} else {
		parts = append(parts,
			m.theme.Hint.Render("Run now?  [y] yes  [n] no  [e] explain"),
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
	lines := make([]string, 0, len(m.runOut)+len(m.runErr))
	for _, line := range m.runOut {
		lines = append(lines, m.theme.OutputLine.Render(line))
	}
	for _, line := range m.runErr {
		lines = append(lines, m.theme.ErrorLine.Render(line))
	}
	visible := m.windowLines(lines, max(6, m.height/3), m.runScroll)
	scrollHint := ""
	if len(lines) > len(visible) {
		scrollHint = m.theme.Dim.Render(fmt.Sprintf("Scroll: %d/%d (j/k, PgUp/PgDn)", m.runScroll+1, len(lines)))
	}

	parts := []string{
		m.theme.Title.Render("Running"),
		m.theme.Dim.Render(m.runStatus),
		m.theme.Command.Render(m.finalCmd),
		"",
		m.theme.SectionTitle.Render("Output"),
		m.theme.OutputPane.Render(strings.Join(visible, "\n")),
	}
	if scrollHint != "" {
		parts = append(parts, scrollHint)
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
		m.theme.ExplainLabel.Render("LLM: " + m.explainer.ProviderName()),
	}
	if strings.TrimSpace(m.explainMD) == "" {
		parts = append(parts, m.theme.Spinner.Render(m.spinner.View()+" Generating explanation..."))
	} else {
		parts = append(parts, m.theme.ExplainPane.Render(m.explainMD))
	}
	if m.explainErr != "" {
		parts = append(parts, m.theme.Error.Render(m.explainErr))
	}
	if m.explainEnd {
		parts = append(parts, m.theme.Hint.Render("Enter/Esc: back"))
	}
	return strings.Join(parts, "\n")
}

func startSearchCmd(engine *search.Engine, query string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan tea.Msg, 4)
		go func() {
			ch <- searchProgressMsg{text: "parsing intent", ch: ch}
			intent := parser.ParseIntent(query)
			ch <- searchProgressMsg{text: fmt.Sprintf("searching %d commands", engine.EntryCount()), ch: ch}
			matches := engine.Search(intent, 3)
			ch <- searchDoneMsg{intent: intent, matches: matches, query: query}
			close(ch)
		}()
		return waitForChan(ch)()
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

func (m model) renderPromptBox() string {
	style := m.theme.InputNeutral
	label := "● idle"
	switch m.inputSignal {
	case "recognized":
		style = m.theme.InputRecognized
		label = "◉ intent"
	case "confident":
		style = m.theme.InputConfident
		label = "● confident"
	case "unsure":
		style = m.theme.InputUnsure
		label = "◌ unsure"
	}
	if !m.supportsUnicode {
		label = strings.ReplaceAll(strings.ReplaceAll(label, "●", "*"), "◉", "*")
		label = strings.ReplaceAll(label, "◌", "-")
	}
	modePrefix := m.currentModePrefix()
	return style.Render(modePrefix + " " + m.input.View() + "\n" + m.theme.Dim.Render(label))
}

func inferInputSignal(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return "neutral"
	}
	intent := parser.ParseIntent(q)
	hasAction := strings.TrimSpace(intent.Action) != ""
	hasTarget := strings.TrimSpace(intent.Target) != ""
	if hasAction && hasTarget {
		return "confident"
	}
	if hasAction {
		return "recognized"
	}
	if len(strings.Fields(q)) >= 3 {
		return "unsure"
	}
	return "neutral"
}

func (m model) renderConfidence(score float64) string {
	if !m.supportsColor {
		if score >= 0.75 {
			return "[high]"
		}
		if score >= 0.45 {
			return "[mid]"
		}
		return "[low]"
	}
	dot := "●"
	if !m.supportsUnicode {
		dot = "*"
	}
	if score >= 0.75 {
		return m.theme.ConfidenceHigh.Render(dot)
	}
	if score >= 0.45 {
		return m.theme.ConfidenceMid.Render(dot)
	}
	return m.theme.ConfidenceLow.Render(dot)
}

func renderFlags(flags []search.CommandFlag) string {
	if len(flags) == 0 {
		return ""
	}
	maxTags := min(3, len(flags))
	tags := make([]string, 0, maxTags)
	for i := 0; i < maxTags; i++ {
		tags = append(tags, "["+flags[i].Flag+"]")
	}
	return strings.Join(tags, " ")
}

func highlightDangerous(cmd string) string {
	replacer := strings.NewReplacer(
		"rm -rf", "[rm -rf]",
		"del /f", "[del /f]",
		"mkfs", "[mkfs]",
	)
	return replacer.Replace(cmd)
}

func (m model) windowLines(lines []string, height, scroll int) []string {
	if len(lines) == 0 {
		return []string{m.theme.Dim.Render("(no output yet)")}
	}
	if height <= 0 {
		height = 8
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll >= len(lines) {
		scroll = len(lines) - 1
	}
	start := scroll
	end := min(len(lines), start+height)
	return lines[start:end]
}

func (m model) renderStatusBar() string {
	mode := m.currentModeLabel()
	llmIcon := "●"
	if !m.supportsUnicode {
		llmIcon = "*"
	}
	llmText := "LLM off"
	llmStyle := m.theme.StatusMuted
	if m.llmAvailable {
		llmText = "LLM ready"
		llmStyle = m.theme.StatusOK
	}
	parts := []string{
		m.theme.StatusMode.Render("Mode: " + mode),
		m.theme.StatusMuted.Render(fmt.Sprintf("Commands: %d", m.commandCount)),
		llmStyle.Render(llmIcon + " " + llmText),
	}
	return m.theme.StatusBar.Render(strings.Join(parts, "  |  "))
}

func (m model) currentModeLabel() string {
	switch m.state {
	case stateExplain:
		return "explain"
	case stateConfirm:
		if len(m.missing) > 0 {
			return "add"
		}
		return "command"
	default:
		return "command"
	}
}

func (m model) currentModePrefix() string {
	switch m.currentModeLabel() {
	case "explain":
		if m.supportsUnicode {
			return "[?]"
		}
		return "[E]"
	case "add":
		return "[+]"
	default:
		return "[>]"
	}
}

func (m model) viewWelcome() string {
	logo := []string{
		"  ____  _          _ _    _    ___ ",
		" / ___|| |__   ___| | |  / \\  |_ _|",
		" \\___ \\| '_ \\ / _ \\ | | / _ \\  | | ",
		"  ___) | | | |  __/ | |/ ___ \\ | | ",
		" |____/|_| |_|\\___|_|_/_/   \\_\\___|",
	}
	if !m.supportsUnicode {
		logo = []string{"ShellAI"}
	}
	items := make([]string, 0, len(welcomeExamples))
	for i, ex := range welcomeExamples {
		prefix := "  "
		if i == m.welcomeIndex {
			prefix = "> "
		}
		items = append(items, m.theme.WelcomeItem.Render(prefix+ex))
	}
	return strings.Join([]string{
		m.theme.Title.Render(strings.Join(logo, "\n")),
		m.theme.Subtitle.Render("Intent-first shell assistant with safe execution and explain mode."),
		"",
		m.theme.SectionTitle.Render("Try an example"),
		strings.Join(items, "\n"),
		"",
		m.theme.Hint.Render("Up/Down: choose  |  Enter: start  |  Esc: skip"),
	}, "\n")
}

func shouldShowWelcome() bool {
	marker := filepath.Join(config.ConfigDir(), ".welcome_seen")
	_, err := os.Stat(marker)
	return os.IsNotExist(err)
}

func markWelcomeSeen() error {
	marker := filepath.Join(config.ConfigDir(), ".welcome_seen")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte("seen\n"), 0o644)
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
