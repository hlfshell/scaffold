package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	liveChromeStyle = lipgloss.NewStyle().Padding(1, 2)
	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	statusStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	tabStyle        = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("245"))
	activeTabStyle  = lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("63"))
	panelStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
	helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	successStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("57"))
)

type liveCommand struct{}

func (liveCommand) Run(app *App) error {
	program := tea.NewProgram(newModel(app), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

type tab int

const (
	tabStatus tab = iota
	tabResources
	tabEndpoints
	tabEnv
	tabCommands
)

var tabNames = []string{"status", "resources", "endpoints", "environment", "commands"}

type model struct {
	app             *App
	status          string
	tab             tab
	report          StatusReport
	details         string
	commandOutput   string
	selectedCommand int
	width           int
	height          int
	creating        bool
	cleaning        bool
	executing       bool
}

type createMsg struct {
	err error
}

type cleanupMsg struct {
	err error
}

type commandMsg struct {
	name   string
	output string
	err    error
}

type refreshMsg struct {
	report StatusReport
	err    error
}

type tickMsg struct{}

func newModel(app *App) model {
	return model{
		app:    app,
		status: "ready",
		tab:    tabStatus,
		width:  96,
		height: 28,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refresh(), tick())
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.refresh(), tick())
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			m.tab = (m.tab + 1) % tab(len(tabNames))
			return m, nil
		case "shift+tab", "left", "h":
			m.tab = (m.tab + tab(len(tabNames)) - 1) % tab(len(tabNames))
			return m, nil
		case "up", "k":
			if m.tab == tabCommands && m.selectedCommand > 0 {
				m.selectedCommand--
			}
			return m, nil
		case "down", "j":
			if m.tab == tabCommands && m.selectedCommand < len(m.app.commands)-1 {
				m.selectedCommand++
			}
			return m, nil
		case "enter":
			if m.tab == tabCommands {
				return m.startSelectedCommand()
			}
			return m, nil
		case "s":
			if m.creating {
				return m, nil
			}
			m.creating = true
			m.status = "starting"
			return m, m.create()
		case "d", "c":
			if m.cleaning {
				return m, nil
			}
			m.cleaning = true
			m.status = "cleaning"
			return m, m.cleanup()
		}
	case createMsg:
		m.creating = false
		if msg.err != nil {
			m.status = "start failed"
			m.details = msg.err.Error()
			return m, nil
		}
		m.status = "running"
		m.details = ""
		return m, m.refresh()
	case cleanupMsg:
		m.cleaning = false
		if msg.err != nil {
			m.status = "cleanup failed"
			m.details = msg.err.Error()
			return m, nil
		}
		m.status = "stopped"
		m.details = ""
		return m, m.refresh()
	case commandMsg:
		m.executing = false
		if msg.err != nil {
			m.status = "command failed"
			m.commandOutput = fmt.Sprintf("%s failed: %v\n%s", msg.name, msg.err, msg.output)
			return m, nil
		}
		m.status = "command complete"
		m.commandOutput = strings.TrimSpace(msg.output)
		if m.commandOutput == "" {
			m.commandOutput = msg.name + " completed"
		}
		return m, m.refresh()
	case refreshMsg:
		if msg.err != nil {
			m.status = "refresh failed"
			m.details = msg.err.Error()
			return m, nil
		}
		m.report = msg.report
		if m.creating || m.cleaning || m.executing {
			return m, nil
		}
		if msg.report.Known {
			if msg.report.Running {
				m.status = "running"
			} else {
				m.status = "stopped"
			}
		} else if m.status == "ready" || m.status == "refresh failed" {
			m.status = "unknown"
		}
	}

	return m, nil
}

func (m model) View() string {
	width := m.width - 4
	if width < 60 {
		width = 60
	}
	panelHeight := m.height - 10
	if panelHeight < 10 {
		panelHeight = 10
	}

	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		titleStyle.Render(m.app.name),
		"  ",
		statusStyle.Render(m.status),
	)

	panel := panelStyle.Width(width).Height(panelHeight).Render(m.page())
	help := helpStyle.Render("s start  d down  c cleanup  tab swap tab  q quit")
	if m.tab == tabCommands {
		help = helpStyle.Render("up/down select  enter run  s start  d down  tab swap tab  q quit")
	}

	body := strings.Join([]string{
		header,
		m.tabs(),
		panel,
		help,
	}, "\n\n")

	return liveChromeStyle.Render(body)
}

func (m model) tabs() string {
	parts := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		if tab(i) == m.tab {
			parts = append(parts, activeTabStyle.Render(name))
			continue
		}
		parts = append(parts, tabStyle.Render(name))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m model) page() string {
	switch m.tab {
	case tabStatus:
		if m.details != "" && (m.status == "start failed" || m.status == "cleanup failed" || m.status == "refresh failed") {
			return errorStyle.Render(m.details)
		}
		return statusText(m.report)
	case tabResources:
		var builder strings.Builder
		printResources(&builder, m.report.Resources)
		return strings.TrimRight(builder.String(), "\n")
	case tabEndpoints:
		return mapText("Endpoints", endpoints(m.app.service))
	case tabEnv:
		return mapText("Environment", env(m.app.service))
	case tabCommands:
		return m.commandsPage()
	default:
		return ""
	}
}

func (m model) commandsPage() string {
	if len(m.app.commands) == 0 {
		return "Commands:\n  none"
	}

	var builder strings.Builder
	builder.WriteString("Commands:\n")
	for i, command := range m.app.commands {
		line := fmt.Sprintf("  %s", command.Name)
		if command.Help != "" {
			line += "  " + command.Help
		}
		if i == m.selectedCommand {
			builder.WriteString(selectedStyle.Render(line))
		} else {
			builder.WriteString(line)
		}
		builder.WriteString("\n")
	}
	if m.executing {
		builder.WriteString("\n")
		builder.WriteString(successStyle.Render("running command..."))
	}
	if m.commandOutput != "" {
		builder.WriteString("\n\nLast output:\n")
		builder.WriteString(m.commandOutput)
	}

	return strings.TrimRight(builder.String(), "\n")
}

func (m model) startSelectedCommand() (tea.Model, tea.Cmd) {
	if m.executing || len(m.app.commands) == 0 {
		return m, nil
	}
	if m.selectedCommand >= len(m.app.commands) {
		m.selectedCommand = len(m.app.commands) - 1
	}

	command := m.app.commands[m.selectedCommand]
	m.executing = true
	m.status = "running " + command.Name
	return m, m.runCommand(command)
}

func (m model) create() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.app.timeout)
		defer cancel()

		return createMsg{err: create(ctx, m.app.service)}
	}
}

func (m model) cleanup() tea.Cmd {
	return func() tea.Msg {
		return cleanupMsg{err: down(context.Background(), m.app.service)}
	}
}

func (m model) runCommand(command Command) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.app.timeout)
		defer cancel()

		output := &strings.Builder{}
		err := runRegisteredCommandWithOutput(ctx, m.app, command, nil, output)

		return commandMsg{name: command.Name, output: output.String(), err: err}
	}
}

func (m model) refresh() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		report, err := status(ctx, m.app.service)
		return refreshMsg{report: report, err: err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func mapText(title string, values map[string]string) string {
	var builder strings.Builder
	printMap(&builder, title, values)
	return strings.TrimRight(builder.String(), "\n")
}
