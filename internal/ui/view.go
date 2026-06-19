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

	l := computeLayout(m.termWidth, m.termHeight)

	status := renderStatusBar(m, l.termW, l)
	workspaces := renderWorkspacePanel(m, l.leftW, l.contentH)
	claude := renderAgentPanel(m, m.claude, "Claude", ColorClaude, PanelClaude, l.agent1W, l.contentH)
	opencode := renderAgentPanel(m, m.opencode, "OpenCode", ColorOpenCode, PanelOpenCode, l.agent2W, l.contentH)
	broker := renderBrokerPanel(m, l.termW, l.brokerH)
	input := renderInputBar(m, l.termW)

	agentsRow := lipgloss.JoinHorizontal(lipgloss.Top, claude, opencode)
	contentRow := lipgloss.JoinHorizontal(lipgloss.Top, workspaces, agentsRow)

	return strings.Join([]string{
		status,
		contentRow,
		broker,
		input,
	}, "\n")
}
