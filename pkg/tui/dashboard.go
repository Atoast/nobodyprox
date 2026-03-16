package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nobodyprox/nobodyprox/pkg/event"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

type viewMode int

const (
	modeDashboard viewMode = iota
	modeBuilder
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	statStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3C3C3C")).
			Italic(true)

	builderInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true)

	builderOutputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#4A90E2")).
				Bold(true)
)

type model struct {
	mode            viewMode
	ready           bool
	viewport        viewport.Model
	textInput       textinput.Model
	engine          *filter.Engine
	logs            []string
	totalRequests   int
	totalRedactions int
	watchMode       bool
	provider        string
	modelName       string
	availableLabels []string
	events          chan event.Event
	builderResult   string
}

func NewModel(watchMode bool, provider, modelName string, labels []string, engine *filter.Engine) model {
	ti := textinput.New()
	ti.Placeholder = "Type text to test rules..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 60

	return model{
		mode:            modeDashboard,
		logs:            make([]string, 0),
		textInput:       ti,
		engine:          engine,
		watchMode:       watchMode,
		provider:        provider,
		modelName:       modelName,
		availableLabels: labels,
		events:          event.GlobalBus.Subscribe(),
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
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.mode == modeDashboard {
				m.mode = modeBuilder
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
		headerHeight := 4
		footerHeight := 3
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.SetContent("Waiting for traffic...")
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height-verticalMarginHeight
		}
		m.textInput.Width = msg.Width - 10
	}

	if m.mode == modeBuilder {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
		m.builderResult = m.engine.DebugRedact(m.textInput.Value())
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) addLog(input string, width int) {
	timestamp := time.Now().Format("15:04:05")
	prefix := fmt.Sprintf("[%s] ", timestamp)
	indent := strings.Repeat(" ", len(prefix))
	
	lines := strings.Split(input, "\n")
	availableWidth := width - len(prefix) - 2
	if availableWidth < 10 {
		availableWidth = 40
	}

	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" && i > 0 {
			m.logs = append(m.logs, indent)
			continue
		}

		for len(line) > 0 {
			chunkLen := availableWidth
			if len(line) < chunkLen {
				chunkLen = len(line)
			}
			
			chunk := line[:chunkLen]
			if i == 0 && len(line) == len(lines[0]) {
				m.logs = append(m.logs, prefix+chunk)
			} else {
				m.logs = append(m.logs, indent+chunk)
			}
			line = line[chunkLen:]
		}
	}

	if len(m.logs) > 1000 {
		m.logs = m.logs[len(m.logs)-1000:]
	}
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing Dashboard..."
	}

	if m.mode == modeBuilder {
		return m.builderView()
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
	)

	status := lipgloss.JoinHorizontal(lipgloss.Top,
		infoStyle.Render(fmt.Sprintf("Provider: %s", m.provider)),
		" | ",
		infoStyle.Render(fmt.Sprintf("Model: %s", m.modelName)),
		" | ",
		infoStyle.Render(fmt.Sprintf("Labels: %s", strings.Join(m.availableLabels, ", "))),
	)

	footer := "\n [tab] Rule Builder  [w] Toggle Watch  [q] Quit"

	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s", 
		header, 
		m.viewport.View(), 
		status,
		footer)
}

func (m model) builderView() string {
	header := titleStyle.Render("NobodyProx Rule Builder")
	
	inputArea := fmt.Sprintf(
		"Test String:\n%s\n\nResult:\n%s",
		m.textInput.View(),
		builderOutputStyle.Render(m.builderResult),
	)

	status := infoStyle.Render(fmt.Sprintf("Available Labels: %s", strings.Join(m.availableLabels, ", ")))
	
	footer := "\n [tab] Back to Dashboard  [q] Quit"

	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s", 
		header, 
		inputArea,
		status,
		footer)
}

func getModeName(watch bool) string {
	if watch {
		return "WATCH (Audit)"
	}
	return "FILTERING (Active)"
}

func Start() error {
	return nil
}
