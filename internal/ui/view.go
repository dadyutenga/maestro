package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the full TUI layout.
func (m Model) View() string {
	if m.termWidth == 0 || m.termHeight == 0 {
		return "Initializing Maestro..."
	}

	statusH := 1
	brokerH := 6
	inputH := 3
	contentH := m.termHeight - statusH - brokerH - inputH - 1
	if contentH < 5 {
		contentH = 5
	}

	leftW := int(float64(m.termWidth) * 0.20)
	if leftW < 20 {
		leftW = 20
	}
	rightW := m.termWidth - leftW - 1
	agentW := rightW / 2

	status := renderStatusBar(m, m.termWidth)
	workspaces := renderWorkspacePanel(m, leftW, contentH)
	claude := renderAgentPanel(m, m.claude, "Claude", ColorClaude, agentW, contentH)
	opencode := renderAgentPanel(m, m.opencode, "OpenCode", ColorOpenCode, agentW, contentH)
	broker := renderBrokerPanel(m, m.termWidth, brokerH)
	input := renderInputBar(m, m.termWidth)

	agentsRow := lipgloss.JoinHorizontal(lipgloss.Top, claude, opencode)
	contentRow := lipgloss.JoinHorizontal(lipgloss.Top, workspaces, agentsRow)

	return strings.Join([]string{
		status,
		contentRow,
		broker,
		input,
	}, "\n")
}
