package ui

import (
	"fmt"
	"strings"

	"github.com/biglitecode/maestro/internal/agent"
	"github.com/biglitecode/maestro/internal/broker"
	"github.com/biglitecode/maestro/internal/workspace"

	"github.com/charmbracelet/lipgloss"
)

func panelStyle(focused bool, color lipgloss.Color) lipgloss.Style {
	border := lipgloss.NormalBorder()
	if focused {
		return lipgloss.NewStyle().
			Border(border).
			BorderForeground(ColorBorderFocus).
			BorderBackground(color).
			Padding(0, 1)
	}
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(ColorBorder).
		Padding(0, 1)
}

func renderWorkspacePanel(m Model, width, height int) string {
	style := panelStyle(m.focusedPanel == PanelWorkspaces, ColorStatus).Width(width).Height(height)
	var sb strings.Builder
	sb.WriteString("Workspaces\n")
	sb.WriteString(strings.Repeat("─", width-2))
	sb.WriteByte('\n')

	wss := m.wsManager.List()
	if len(wss) == 0 {
		sb.WriteString("(no workspaces)\n")
		sb.WriteString("press 'n' to create one")
		return style.Render(sb.String())
	}

	for i, ws := range wss {
		cursor := "  "
		if i == m.selectedWorkspace {
			cursor = "> "
		}
		attached := "○"
		if len(ws.ListAgents()) > 0 {
			attached = "●"
		}
		line := fmt.Sprintf("%s%s %s (%s", cursor, attached, ws.Name, ws.Mode)
		if ws.Mode == workspace.ModeIsolated {
			line += fmt.Sprintf(", %s", ws.Branch)
		}
		line += ")\n"
		sb.WriteString(line)
	}

	return style.Render(sb.String())
}

func renderAgentPanel(m Model, a *agent.Agent, title string, color lipgloss.Color, width, height int) string {
	panel := PanelClaude
	if a != nil && a.Kind == agent.KindOpenCode {
		panel = PanelOpenCode
	}
	style := panelStyle(m.focusedPanel == panel, color).Width(width).Height(height)

	var sb strings.Builder
	status := "idle"
	if a != nil {
		status = a.Status.String()
	}
	sb.WriteString(fmt.Sprintf("%s [%s]\n", title, status))
	sb.WriteString(strings.Repeat("─", width-2))
	sb.WriteByte('\n')

	var content string
	if panel == PanelClaude {
		content = m.claudeVP.View()
	} else {
		content = m.opencodeVP.View()
	}
	if content == "" {
		content = "(no output yet)"
	}
	sb.WriteString(content)

	return style.Render(sb.String())
}

func renderBrokerPanel(m Model, width, height int) string {
	style := panelStyle(m.focusedPanel == PanelBroker, ColorBroker).Width(width).Height(height)
	var sb strings.Builder
	sb.WriteString("Broker\n")
	sb.WriteString(strings.Repeat("─", width-2))
	sb.WriteByte('\n')

	content := m.brokerVP.View()
	if content == "" {
		content = "(no messages)"
	}
	sb.WriteString(content)

	return style.Render(sb.String())
}

func renderInputBar(m Model, width int) string {
	style := panelStyle(m.focusedPanel == PanelInput, ColorForeground).Width(width)
	var sb strings.Builder
	sb.WriteString(m.inputBar.View())
	return style.Render(sb.String())
}

func renderStatusBar(m Model, width int) string {
	branch := currentGitBranch(m.cfg.Project.Root)
	text := fmt.Sprintf(" %s | branch: %s | agents: %d | focus: %s ",
		m.cfg.Project.Name,
		branch,
		m.runningAgentsCount(),
		m.focusedPanel,
	)
	if m.err != nil {
		return lipgloss.NewStyle().
			Width(width).
			Background(ColorError).
			Foreground(ColorBackground).
			Render(text + "| " + m.statusText)
	}
	return lipgloss.NewStyle().
		Width(width).
		Background(ColorStatus).
		Foreground(ColorBackground).
		Render(text)
}

func brokerMessages(m Model) []broker.Message {
	if m.broker == nil {
		return nil
	}
	return m.broker.Log()
}
