package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/biglitecode/maestro/internal/agent"
	"github.com/biglitecode/maestro/internal/broker"
	"github.com/biglitecode/maestro/internal/workspace"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.recalcLayout()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case AgentOutputMsg:
		return m.handleAgentOutput(msg)

	case BrokerMsgMsg:
		return m.handleBrokerMsg(msg)

	case AgentExitedMsg:
		m.setError(fmt.Errorf("agent %s exited", msg.AgentID))
		return m, nil

	case ErrorMsg:
		m.setError(msg.Err)
		return m, nil

	case WorkspaceCreatedMsg:
		if msg.Err != nil {
			m.setError(msg.Err)
			return m, nil
		}
		m.inputMode = InputNormal
		m.creatingWorkspace = workspaceState{}
		m.inputBar.Reset()
		m.inputBar.Placeholder = "type a task or command..."
		m.focusedPanel = PanelWorkspaces
		m.statusText = fmt.Sprintf("workspace created: %s", msg.Workspace.Name)
		return m, nil
	}

	var cmd tea.Cmd
	if m.focusedPanel == PanelInput {
		m.inputBar, cmd = m.inputBar.Update(msg)
	}
	return m, cmd
}

func (m *Model) recalcLayout() {
	if m.termHeight < 12 {
		return
	}
	if m.termWidth < 40 {
		return
	}

	leftW := int(float64(m.termWidth) * 0.20)
	if leftW < 20 {
		leftW = 20
	}
	rightW := m.termWidth - leftW - 1
	agentW := rightW / 2

	statusH := 1
	brokerH := 6
	inputH := 3
	contentH := m.termHeight - statusH - brokerH - inputH - 1
	if contentH < 5 {
		contentH = 5
	}

	m.claudeVP.Width = agentW - 2
	m.claudeVP.Height = contentH - 2
	m.opencodeVP.Width = agentW - 2
	m.opencodeVP.Height = contentH - 2
	m.brokerVP.Width = m.termWidth - 2
	m.brokerVP.Height = brokerH - 2
	m.inputBar.SetWidth(m.termWidth - 2)
	m.inputBar.SetHeight(inputH - 1)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) {
		if m.claude != nil {
			m.claude.Kill()
		}
		if m.opencode != nil {
			m.opencode.Kill()
		}
		return m, tea.Quit
	}

	if key.Matches(msg, m.keys.NextPanel) {
		m.focusedPanel = nextPanel(m.focusedPanel)
		if m.focusedPanel == PanelClaude || m.focusedPanel == PanelOpenCode {
			m.lastFocusedAgent = m.focusedPanel
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.PrevPanel) {
		m.focusedPanel = prevPanel(m.focusedPanel)
		if m.focusedPanel == PanelClaude || m.focusedPanel == PanelOpenCode {
			m.lastFocusedAgent = m.focusedPanel
		}
		return m, nil
	}

	if m.focusedPanel == PanelInput {
		return m.handleInputKey(msg)
	}

	switch m.focusedPanel {
	case PanelWorkspaces:
		return m.handleWorkspaceKey(msg)
	case PanelClaude, PanelOpenCode:
		return m.handleAgentKey(msg)
	case PanelBroker:
		return m.handleBrokerKey(msg)
	}

	return m, nil
}

func nextPanel(p Panel) Panel {
	switch p {
	case PanelWorkspaces:
		return PanelClaude
	case PanelClaude:
		return PanelOpenCode
	case PanelOpenCode:
		return PanelBroker
	case PanelBroker:
		return PanelInput
	case PanelInput:
		return PanelWorkspaces
	}
	return PanelWorkspaces
}

func prevPanel(p Panel) Panel {
	switch p {
	case PanelWorkspaces:
		return PanelInput
	case PanelClaude:
		return PanelWorkspaces
	case PanelOpenCode:
		return PanelClaude
	case PanelBroker:
		return PanelOpenCode
	case PanelInput:
		return PanelBroker
	}
	return PanelWorkspaces
}

func (m Model) handleWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.NewWorkspace) {
		m.inputMode = InputCreatingName
		m.inputBar.Placeholder = "workspace name:"
		m.inputBar.Reset()
		m.focusedPanel = PanelInput
		return m, m.inputBar.Focus()
	}

	if key.Matches(msg, m.keys.DeleteWorkspace) {
		wss := m.wsManager.List()
		if len(wss) == 0 || m.selectedWorkspace >= len(wss) {
			return m, nil
		}
		ws := wss[m.selectedWorkspace]
		if err := m.wsManager.Remove(ws.ID); err != nil {
			m.setError(err)
		} else {
			m.statusText = fmt.Sprintf("removed workspace %s", ws.Name)
			if m.selectedWorkspace > 0 {
				m.selectedWorkspace--
			}
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.SpawnAgent) {
		wss := m.wsManager.List()
		if len(wss) == 0 || m.selectedWorkspace >= len(wss) {
			m.setError(fmt.Errorf("no workspace selected"))
			return m, nil
		}
		ws := wss[m.selectedWorkspace]

		var kind agent.Kind
		var binary string
		if m.claude == nil || !m.claude.IsRunning() {
			kind = agent.KindClaude
			binary = m.cfg.Agents.Claude
		} else if m.opencode == nil || !m.opencode.IsRunning() {
			kind = agent.KindOpenCode
			binary = m.cfg.Agents.OpenCode
		} else {
			m.setError(fmt.Errorf("both agents already running"))
			return m, nil
		}

		id := kind.String()
		a := agent.New(id, kind, binary, ws.Dir)
		if err := a.Start(); err != nil {
			m.setError(err)
			return m, nil
		}
		ws.AttachAgent(id)

		if kind == agent.KindClaude {
			m.claude = a
		} else {
			m.opencode = a
		}
		m.broker = broker.New(m.claude, m.opencode)
		m.statusText = fmt.Sprintf("spawned %s on %s", id, ws.Name)
		return m, tea.Batch(listenAgent(a), listenBroker(m.broker))
	}

	switch msg.String() {
	case "up", "k":
		if m.selectedWorkspace > 0 {
			m.selectedWorkspace--
		}
	case "down", "j":
		wss := m.wsManager.List()
		if m.selectedWorkspace < len(wss)-1 {
			m.selectedWorkspace++
		}
	}

	return m, nil
}

func (m Model) handleAgentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := m.activeAgentForPanel(m.focusedPanel)

	if key.Matches(msg, m.keys.KillAgent) {
		if a == nil {
			return m, nil
		}
		if err := a.Kill(); err != nil {
			m.setError(err)
		} else {
			m.statusText = fmt.Sprintf("killed %s", a.ID)
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Review) {
		if m.broker == nil {
			m.setError(fmt.Errorf("broker not initialized"))
			return m, nil
		}
		var fromID, toID string
		if m.focusedPanel == PanelClaude {
			fromID = "claude"
			toID = "opencode"
		} else {
			fromID = "opencode"
			toID = "claude"
		}
		if err := m.broker.Review(fromID, toID); err != nil {
			m.setError(err)
		} else {
			m.statusText = fmt.Sprintf("review %s -> %s", fromID, toID)
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Broadcast) {
		if m.broker == nil {
			m.setError(fmt.Errorf("broker not initialized"))
			return m, nil
		}
		value := m.inputBar.Value()
		if value == "" {
			m.setError(fmt.Errorf("input bar is empty"))
			return m, nil
		}
		if err := m.broker.Send("user", "broadcast", value); err != nil {
			m.setError(err)
		} else {
			m.statusText = "broadcast sent"
			m.inputBar.Reset()
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Clear) {
		if m.focusedPanel == PanelClaude {
			m.claudeVP.SetContent("")
		} else {
			m.opencodeVP.SetContent("")
		}
		return m, nil
	}

	if m.focusedPanel == PanelClaude {
		m.claudeVP, _ = m.claudeVP.Update(msg)
	} else {
		m.opencodeVP, _ = m.opencodeVP.Update(msg)
	}

	return m, nil
}

func (m Model) handleBrokerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.brokerVP, _ = m.brokerVP.Update(msg)
	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Send) {
		return m.submitInput()
	}

	var cmd tea.Cmd
	m.inputBar, cmd = m.inputBar.Update(msg)
	return m, cmd
}

func (m Model) submitInput() (tea.Model, tea.Cmd) {
	value := m.inputBar.Value()
	m.inputBar.Reset()

	switch m.inputMode {
	case InputNormal:
		if value == "" {
			return m, nil
		}
		target := m.activeAgentForPanel(m.lastFocusedAgent)
		if target == nil {
			m.setError(fmt.Errorf("no agent running for %s", m.lastFocusedAgent))
			return m, nil
		}
		if err := target.Send(value); err != nil {
			m.setError(err)
		} else {
			m.statusText = fmt.Sprintf("sent to %s", target.ID)
		}
		return m, nil

	case InputCreatingName:
		if value == "" {
			m.inputMode = InputNormal
			m.focusedPanel = PanelWorkspaces
			return m, nil
		}
		m.creatingWorkspace.name = value
		m.inputMode = InputCreatingMode
		m.inputBar.Placeholder = "mode: isolated (i) or shared (s):"
		m.inputBar.Reset()
		return m, m.inputBar.Focus()

	case InputCreatingMode:
		switch strings.ToLower(value) {
		case "i", "isolated":
			m.creatingWorkspace.mode = workspace.ModeIsolated
			m.inputMode = InputCreatingBranch
			m.inputBar.Placeholder = "branch name (enter for default):"
		case "s", "shared":
			m.creatingWorkspace.mode = workspace.ModeShared
			m.inputMode = InputCreatingBranch
			m.inputBar.Placeholder = "shared directory path:"
		default:
			m.setError(fmt.Errorf("invalid mode %q, use i/s", value))
			return m, nil
		}
		m.inputBar.Reset()
		return m, m.inputBar.Focus()

	case InputCreatingBranch:
		name := m.creatingWorkspace.name
		mode := m.creatingWorkspace.mode

		var ws *workspace.Workspace
		var err error
		return m, func() tea.Msg {
			if mode == workspace.ModeIsolated {
				branch := value
				if branch == "" {
					branch = name
				}
				ws, err = m.wsManager.CreateIsolated(name, branch)
			} else {
				dir := value
				if dir == "" {
					dir = m.cfg.Project.Root
				}
				ws, err = m.wsManager.CreateShared(name, dir)
			}
			return WorkspaceCreatedMsg{Workspace: ws, Err: err}
		}
	}

	return m, nil
}

func (m Model) handleAgentOutput(msg AgentOutputMsg) (tea.Model, tea.Cmd) {
	line := msg.Line
	text := fmt.Sprintf("[%s] %s\n", line.Timestamp.Format("15:04:05"), line.Text)

	if line.AgentID == "claude" {
		m.claudeVP.SetContent(m.claudeVP.View() + text)
		m.claudeVP.GotoBottom()
	} else if line.AgentID == "opencode" {
		m.opencodeVP.SetContent(m.opencodeVP.View() + text)
		m.opencodeVP.GotoBottom()
	}

	var cmds []tea.Cmd
	if m.claude != nil {
		cmds = append(cmds, listenAgent(m.claude))
	}
	if m.opencode != nil {
		cmds = append(cmds, listenAgent(m.opencode))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleBrokerMsg(msg BrokerMsgMsg) (tea.Model, tea.Cmd) {
	text := formatBrokerMsg(msg.Message)
	m.brokerVP.SetContent(m.brokerVP.View() + text + "\n")
	m.brokerVP.GotoBottom()
	return m, listenBroker(m.broker)
}

func formatBrokerMsg(msg broker.Message) string {
	return fmt.Sprintf("[%s] %s -> %s: %s", msg.Timestamp.Format("15:04:05"), msg.From, msg.To, msg.Text)
}

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}
