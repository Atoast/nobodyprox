package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nobodyprox/nobodyprox/pkg/config"
	"github.com/nobodyprox/nobodyprox/pkg/event"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

type viewMode int

const (
	modeDashboard viewMode = iota
	modeCombined
)

var (
	// Base Colors
	purple = lipgloss.Color("#7D56F4")
	green  = lipgloss.Color("#04B575")
	blue   = lipgloss.Color("#4A90E2")
	gray   = lipgloss.Color("#888888")
	dark   = lipgloss.Color("#3C3C3C")
	white  = lipgloss.Color("#FAFAFA")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white).
			Background(purple).
			Padding(0, 1)

	statStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	logStyle = lipgloss.NewStyle().
			Foreground(gray)

	infoStyle = lipgloss.NewStyle().
			Foreground(dark).
			Italic(true)

	viewportStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(0, 1)

	builderOutputStyle = lipgloss.NewStyle().
				Foreground(blue).
				Bold(true)
	
	builderInputLineStyle = lipgloss.NewStyle().
				Foreground(purple).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(dark)

	legendStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(purple).
			PaddingLeft(1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(purple)
)

type model struct {
	mode            viewMode
	ready           bool
	width           int
	height          int
	viewport        viewport.Model
	builderViewport viewport.Model
	textInput       textinput.Model
	engine          *filter.Engine
	logs            []string
	builderLogs     []string
	totalRequests   int
	totalRedactions int
	watchMode       bool
	redactResponses bool
	provider        string
	modelName       string
	availableLabels []string
	events          chan event.Event
	
	// Rules Editor State
	rules           []config.Rule
	selectedRuleIdx int
}

func NewModel(watchMode, redactResponses bool, provider, modelName string, labels []string, engine *filter.Engine) model {
	ti := textinput.New()
	ti.Placeholder = "Type text to test rules and press Enter..."
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 60

	// Get initial rules from engine
	rules := make([]config.Rule, len(engine.Rules))
	copy(rules, engine.Rules)

	return model{
		mode:            modeDashboard,
		logs:            make([]string, 0),
		builderLogs:     make([]string, 0),
		textInput:       ti,
		engine:          engine,
		watchMode:       watchMode,
		redactResponses: redactResponses,
		provider:        provider,
		modelName:       modelName,
		availableLabels: labels,
		events:          event.GlobalBus.Subscribe(),
		rules:           rules,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		waitForEvent(m.events),
		textinput.Blink,
	)
}

type eventMsg event.Event

func waitForEvent(ch chan event.Event) tea.Cmd {
	return func() tea.Msg {
		return eventMsg(<-ch)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.mode == modeDashboard {
				m.mode = modeCombined
				m.textInput.Focus()
			} else {
				m.mode = modeDashboard
				m.textInput.Blur()
			}
		case "w":
			if m.mode == modeDashboard {
				m.watchMode = !m.watchMode
				event.GlobalBus.Publish(event.Event{
					Type: event.TypeConfigChange,
					Data: m.watchMode,
				})
			}
		case "r":
			if m.mode == modeDashboard {
				m.redactResponses = !m.redactResponses
				event.GlobalBus.Publish(event.Event{
					Type: event.TypeRedactResponsesChange,
					Data: m.redactResponses,
				})
			}
		
		// Key handling for Combined View
		case "up":
			if m.mode == modeCombined && m.selectedRuleIdx > 0 {
				m.selectedRuleIdx--
				return m, nil
			}
		case "down":
			if m.mode == modeCombined && m.selectedRuleIdx < len(m.rules)-1 {
				m.selectedRuleIdx++
				return m, nil
			}
		case "ctrl+t":
			if m.mode == modeCombined && len(m.rules) > 0 {
				rule := &m.rules[m.selectedRuleIdx]
				if rule.Action == "REDACT" {
					rule.Action = "PSEUDONYMIZE"
				} else {
					rule.Action = "REDACT"
				}
				rule.Replacement = "" 
				m.engine.UpdateRules(m.rules)
				
				// Persist to config.yaml
				// We need the full config object, but for now we can update the rules in a fresh load
				if cfg, err := config.LoadConfig("config.yaml"); err == nil {
					cfg.Rules = m.rules
					config.SaveConfig("config.yaml", cfg)
				}
				
				return m, nil
			}

		case "enter":
			if m.mode == modeCombined && m.textInput.Value() != "" {
				input := m.textInput.Value()
				tagged := m.engine.DebugRedact(input)
				redacted := m.engine.Redact(input, "TEST", "BUILDER")
				
				m.addBuilderLog(fmt.Sprintf("> %s", input), m.builderViewport.Width)
				m.addBuilderLog(lipgloss.NewStyle().Foreground(gray).Render("  TAGGED:   ")+builderOutputStyle.Render(tagged), m.builderViewport.Width)
				m.addBuilderLog(lipgloss.NewStyle().Foreground(gray).Render("  REDACTED: ")+lipgloss.NewStyle().Foreground(green).Render(redacted), m.builderViewport.Width)
				m.addBuilderLog("", m.builderViewport.Width)
				
				m.textInput.Reset()
				return m, nil
			}
		}

	case eventMsg:
		switch msg.Type {
		case event.TypeRequestStart:
			m.totalRequests++
			data := msg.Data.(event.RequestData)
			m.addLog(fmt.Sprintf("[%s] %s %s", msg.ReqID, data.Method, data.Host), m.viewport.Width)
		case event.TypeDetection:
			m.totalRedactions++
			data := msg.Data.(event.DetectionData)
			m.addLog(logStyle.Render(fmt.Sprintf("  └─ [%s] Found %s: %s", data.Context, data.RuleType, data.Original)), m.viewport.Width)
		}
		cmds = append(cmds, waitForEvent(m.events))

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		headerHeight := 4
		footerHeight := 4
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width-4, msg.Height-verticalMarginHeight-2)
			m.viewport.SetContent("Waiting for traffic...")
			
			// Builder viewport takes ~60% of width
			m.builderViewport = viewport.New(int(float64(msg.Width)*0.6), msg.Height-verticalMarginHeight-6)
			m.builderViewport.SetContent("Rule Builder: Type and press Enter.")
			
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = msg.Height - verticalMarginHeight - 2
			
			m.builderViewport.Width = int(float64(msg.Width)*0.6)
			m.builderViewport.Height = msg.Height - verticalMarginHeight - 6
		}
		m.textInput.Width = m.builderViewport.Width - 10
	}

	if m.mode == modeCombined {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
		m.builderViewport, cmd = m.builderViewport.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) addLog(input string, width int) {
	m.logs = m.wrapAndAppend(m.logs, input, width, true)
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}

func (m *model) addBuilderLog(input string, width int) {
	m.builderLogs = m.wrapAndAppend(m.builderLogs, input, width, false)
	m.builderViewport.SetContent(strings.Join(m.builderLogs, "\n"))
	m.builderViewport.GotoBottom()
}

func (m *model) wrapAndAppend(logList []string, input string, width int, withTimestamp bool) []string {
	var prefix string
	if withTimestamp {
		prefix = fmt.Sprintf("[%s] ", time.Now().Format("15:04:05"))
	}
	indent := strings.Repeat(" ", len(prefix))
	
	lines := strings.Split(input, "\n")
	availableWidth := width - len(prefix) - 2
	if availableWidth < 10 {
		availableWidth = 40
	}

	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" && i > 0 {
			logList = append(logList, indent)
			continue
		}

		for len(line) > 0 {
			chunkLen := availableWidth
			if len(line) < chunkLen {
				chunkLen = len(line)
			}
			
			chunk := line[:chunkLen]
			if i == 0 && len(line) == len(lines[0]) && withTimestamp {
				logList = append(logList, prefix+chunk)
			} else {
				logList = append(logList, indent+chunk)
			}
			line = line[chunkLen:]
		}
	}

	if len(logList) > 1000 {
		logList = logList[len(logList)-1000:]
	}
	return logList
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing Dashboard..."
	}

	if m.mode == modeCombined {
		return m.combinedView()
	}
	return m.dashboardView()
}

func (m model) dashboardView() string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render("NobodyProx Dashboard"),
		"  ",
		statStyle.Render(fmt.Sprintf("Requests: %d", m.totalRequests)),
		"  ",
		statStyle.Render(fmt.Sprintf("Redactions: %d", m.totalRedactions)),
		"  ",
		fmt.Sprintf("Mode: %s", getModeName(m.watchMode)),
		"  ",
		fmt.Sprintf("Response Filter: %s", getOnOff(m.redactResponses)),
	)

	status := lipgloss.JoinHorizontal(lipgloss.Top,
		infoStyle.Render(fmt.Sprintf("Provider: %s", m.provider)),
		" | ",
		infoStyle.Render(fmt.Sprintf("Model: %s", m.modelName)),
		" | ",
		infoStyle.Render(fmt.Sprintf("Labels: %s", strings.Join(m.availableLabels, ", "))),
	)

	footer := helpStyle.Render(" [tab] Rule Center  [w] Watch  [r] Resp Redact  [ctrl+c] Quit")

	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s", 
		header, 
		viewportStyle.Render(m.viewport.View()), 
		status,
		footer)
}

func (m model) combinedView() string {
	header := titleStyle.Render("NobodyProx Rule Center")
	
	// Determine shared height for both containers
	sharedHeight := m.builderViewport.Height

	// --- Left Side: Builder ---
	leftWidth := int(float64(m.width) * 0.6)
	builderView := viewportStyle.Width(leftWidth).Height(sharedHeight).Render(m.builderViewport.View())
	inputArea := "\n" + builderInputLineStyle.Render(" Input: ") + m.textInput.View()
	
	legend := legendStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			builderOutputStyle.Render("TAGGED:   ")+"Raw model findings",
			lipgloss.NewStyle().Foreground(green).Render("REDACTED: ")+"Result after rules",
		),
	)
	
	leftSide := lipgloss.JoinVertical(lipgloss.Left, builderView, inputArea, legend)

	// --- Right Side: Rules Editor ---
	rightWidth := m.width - leftWidth - 10
	var rb strings.Builder
	for i, rule := range m.rules {
		pref := "  "
		style := logStyle
		if i == m.selectedRuleIdx {
			pref = "> "
			style = selectedStyle
		}
		
		line := fmt.Sprintf("%s%-12s | %s", pref, rule.Name, rule.Action)
		rb.WriteString(style.Render(line) + "\n")
	}
	
	rightSide := viewportStyle.
		Width(rightWidth).
		Height(sharedHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, "Active Rules:\n", rb.String()))

	// --- Layout ---
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftSide, rightSide)
	
	status := infoStyle.Render(fmt.Sprintf("Active Labels: %s", strings.Join(m.availableLabels, ", ")))
	footer := helpStyle.Render(" [tab] Dashboard  [up/down] Select Rule  [ctrl+t] Toggle Action  [ctrl+c] Quit")

	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s", 
		header, 
		mainContent,
		status,
		footer)
}

func getModeName(watch bool) string {
	if watch {
		return "WATCH (Audit)"
	}
	return "FILTERING (Active)"
}

func getOnOff(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

func Start() error {
	return nil
}
