package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nobodyprox/nobodyprox/pkg/event"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginTop(1)

	statStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3C3C3C")).
			Italic(true)
)

type model struct {
	ready           bool
	viewport        viewport.Model
	logs            []string
	totalRequests   int
	totalRedactions int
	watchMode       bool
	provider        string
	modelName       string
	events          chan event.Event
}

func NewModel(watchMode bool, provider, modelName string) model {
	return model{
		logs:      make([]string, 0),
		watchMode: watchMode,
		provider:  provider,
		modelName: modelName,
		events:    event.GlobalBus.Subscribe(),
	}
}

func (m model) Init() tea.Cmd {
	return waitForEvent(m.events)
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
		case "w":
			m.watchMode = !m.watchMode
			// In a real app we'd send a command back to the proxy here
			event.GlobalBus.Publish(event.Event{
				Type: event.TypeConfigChange,
				Data: m.watchMode,
			})
		}

	case eventMsg:
		switch msg.Type {
		case event.TypeRequestStart:
			m.totalRequests++
			data := msg.Data.(event.RequestData)
			m.addLog(fmt.Sprintf("[%s] %s %s", msg.ReqID, data.Method, data.Host))
		case event.TypeDetection:
			m.totalRedactions++
			data := msg.Data.(event.DetectionData)
			m.addLog(logStyle.Render(fmt.Sprintf("  └─ [%s] Found %s: %s", data.Context, data.RuleType, data.Original)))
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
	}

	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) addLog(s string) {
	m.logs = append(m.logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), s))
	if len(m.logs) > 500 {
		m.logs = m.logs[1:]
	}
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing Dashboard..."
	}

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
	)

	footer := "\n [w] Toggle Watch Mode  [q] Quit"

	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s", 
		header, 
		m.viewport.View(), 
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
	// This will be called from main.go
	return nil // Placeholder
}
