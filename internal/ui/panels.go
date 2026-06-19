package ui

import (
	"fmt"
	"strings"

	"github.com/biglitecode/maestro/internal/agent"
	"github.com/biglitecode/maestro/internal/workspace"

	"github.com/charmbracelet/lipgloss"
)

var (
	borderThin = lipgloss.RoundedBorder()

	baseStyle = lipgloss.NewStyle().
			Border(borderThin).
			BorderForeground(ColorBorder).
			Padding(0)

	focusStyle = lipgloss.NewStyle().
			Border(borderThin).
			BorderForeground(ColorBorderFocus).
			Padding(0)
)

func renderWorkspacePanel(m Model, width, height int) string {
	focused := m.focusedPanel == PanelWorkspaces
	style := baseStyle
	if focused {
		style = focusStyle
	}
	style = style.Width(width).Height(height)

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorStatus).Render("Workspaces"))
	sb.WriteByte('\n')
	sb.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render(strings.Repeat("─", width-2)))
	sb.WriteByte('\n')

	wss := m.wsManager.List()
	if len(wss) == 0 {
		sb.WriteString("(empty) press n")
		return style.Render(sb.String())
	}

	for i, ws := range wss {
		cursor := "  "
		if i == m.selectedWorkspace {
			cursor = lipgloss.NewStyle().Foreground(ColorBorderFocus).Bold(true).Render("> ")
		}
		attached := lipgloss.NewStyle().Foreground(ColorMuted).Render("○")
		if len(ws.ListAgents()) > 0 {
			attached = lipgloss.NewStyle().Foreground(ColorSuccess).Render("●")
		}
		name := lipgloss.NewStyle().Bold(focused && i == m.selectedWorkspace).Render(ws.Name)
		mode := lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("%s", ws.Mode))
		line := fmt.Sprintf("%s%s %s (%s", cursor, attached, name, mode)
		if ws.Mode == workspace.ModeIsolated {
			line += fmt.Sprintf(", %s", ws.Branch)
		}
		line += ")\n"
		sb.WriteString(line)
	}

	return style.Render(sb.String())
}

func renderAgentPanel(m Model, a *agent.Agent, title string, color lipgloss.Color, panel Panel, width, height int) string {
	focused := m.focusedPanel == panel
	style := baseStyle
	if focused {
		style = focusStyle.BorderForeground(color)
	}
	style = style.Width(width).Height(height)

	var sb strings.Builder
	status := "idle"
	if a != nil {
		status = a.Status.String()
	}
	titleStr := lipgloss.NewStyle().Bold(true).Foreground(color).Render(fmt.Sprintf("%s [%s]", title, status))
	sb.WriteString(titleStr)
	sb.WriteByte('\n')
	sb.WriteString(lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("─", width-2)))
	sb.WriteByte('\n')

	var content string
	if panel == PanelClaude {
		content = m.claudeVP.View()
	} else {
		content = m.opencodeVP.View()
	}
	if content == "" {
		content = lipgloss.NewStyle().Foreground(ColorMuted).Render("(no output)")
	}
	sb.WriteString(content)

	return style.Render(sb.String())
}

func renderBrokerPanel(m Model, width, height int) string {
	focused := m.focusedPanel == PanelBroker
	style := baseStyle
	if focused {
		style = focusStyle.BorderForeground(ColorBroker)
	}
	style = style.Width(width).Height(height)

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorBroker).Render("Broker"))
	sb.WriteByte('\n')
	sb.WriteString(lipgloss.NewStyle().Foreground(ColorBroker).Render(strings.Repeat("─", width-2)))
	sb.WriteByte('\n')

	content := m.brokerVP.View()
	if content == "" {
		content = lipgloss.NewStyle().Foreground(ColorMuted).Render("(no messages)")
	}
	sb.WriteString(content)

	return style.Render(sb.String())
}

func renderInputBar(m Model, width int) string {
	focused := m.focusedPanel == PanelInput
	style := baseStyle
	if focused {
		style = focusStyle.BorderForeground(ColorBorderFocus)
	}
	style = style.Width(width)

	var sb strings.Builder
	sb.WriteString(m.inputBar.View())
	return style.Render(sb.String())
}

func renderStatusBar(m Model, width int, l layout) string {
	branch := currentGitBranch(m.cfg.Project.Root)
	focus := m.focusedPanel.String()

	left := fmt.Sprintf(" %s | %s | agents:%d ", m.cfg.Project.Name, branch, m.runningAgentsCount())
	right := fmt.Sprintf(" %dx%d | focus:%s ", l.termW, l.termH, focus)

	fillW := width - lipgloss.Width(left) - lipgloss.Width(right)
	if fillW < 0 {
		fillW = 0
	}

	bar := left + strings.Repeat(" ", fillW) + right

	if m.err != nil {
		return lipgloss.NewStyle().
			Width(width).
			Background(ColorError).
			Foreground(ColorBackground).
			Render(" " + m.statusText)
	}
	return lipgloss.NewStyle().
		Width(width).
		Background(ColorBackground).
		Foreground(ColorForeground).
		Render(bar)
}
