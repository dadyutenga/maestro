package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/biglitecode/maestro/internal/agent"
	"github.com/biglitecode/maestro/internal/broker"
	"github.com/biglitecode/maestro/internal/config"
	"github.com/biglitecode/maestro/internal/workspace"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Panel identifies the focused UI panel.
type Panel int

const (
	PanelWorkspaces Panel = iota
	PanelClaude
	PanelOpenCode
	PanelBroker
	PanelInput
)

func (p Panel) String() string {
	switch p {
	case PanelWorkspaces:
		return "Workspaces"
	case PanelClaude:
		return "Claude"
	case PanelOpenCode:
		return "OpenCode"
	case PanelBroker:
		return "Broker"
	case PanelInput:
		return "Input"
	default:
		return "Unknown"
	}
}

// workspaceState holds temporary workspace creation inputs.
type workspaceState struct {
	name   string
	mode   workspace.Mode
	branch string
	dir    string
}

// InputMode tracks the workspace creation wizard state.
type InputMode int

const (
	InputNormal InputMode = iota
	InputCreatingName
	InputCreatingMode
	InputCreatingBranch
)

// AgentOutputMsg is emitted when an agent produces output.
type AgentOutputMsg struct {
	Line agent.OutputLine
}

// BrokerMsgMsg is emitted when the broker logs a message.
type BrokerMsgMsg struct {
	Message broker.Message
}

// AgentExitedMsg is emitted when an agent process exits.
type AgentExitedMsg struct {
	AgentID string
}

// ErrorMsg surfaces async errors.
type ErrorMsg struct {
	Err error
}

// WorkspaceCreatedMsg is emitted when a workspace is created.
type WorkspaceCreatedMsg struct {
	Workspace *workspace.Workspace
	Err       error
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg               *config.Config
	wsManager         *workspace.Manager
	claude            *agent.Agent
	opencode          *agent.Agent
	broker            *broker.Broker

	keys              KeyMap
	focusedPanel      Panel
	lastFocusedAgent  Panel
	inputMode         InputMode
	creatingWorkspace workspaceState

	selectedWorkspace int

	claudeVP    viewport.Model
	opencodeVP  viewport.Model
	brokerVP    viewport.Model
	inputBar    textarea.Model

	termWidth   int
	termHeight  int

	err         error
	statusText  string
}

// NewModel builds a fresh model.
func NewModel(cfg *config.Config, wsManager *workspace.Manager) Model {
	m := Model{
		cfg:              cfg,
		wsManager:        wsManager,
		keys:             DefaultKeyMap(),
		focusedPanel:     PanelWorkspaces,
		lastFocusedAgent: PanelClaude,
	}

	m.claudeVP = viewport.New(0, 0)
	m.claudeVP.SetContent("")

	m.opencodeVP = viewport.New(0, 0)
	m.opencodeVP.SetContent("")

	m.brokerVP = viewport.New(0, 0)
	m.brokerVP.SetContent("")

	m.inputBar = textarea.New()
	m.inputBar.Placeholder = "type a task or command..."
	m.inputBar.SetHeight(2)
	m.inputBar.ShowLineNumbers = false
	m.inputBar.CharLimit = 0

	return m
}

// Init returns the initial command batch.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		listenAgent(m.claude),
		listenAgent(m.opencode),
		listenBroker(m.broker),
	)
}

func listenAgent(a *agent.Agent) tea.Cmd {
	return func() tea.Msg {
		if a == nil {
			return nil
		}
		line, ok := <-a.OutputCh
		if !ok {
			return AgentExitedMsg{AgentID: a.ID}
		}
		return AgentOutputMsg{Line: line}
	}
}

func listenBroker(b *broker.Broker) tea.Cmd {
	return func() tea.Msg {
		if b == nil {
			return nil
		}
		msg, ok := <-b.MsgCh()
		if !ok {
			return nil
		}
		return BrokerMsgMsg{Message: msg}
	}
}

func currentGitBranch(root string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func (m Model) activeAgentForPanel(p Panel) *agent.Agent {
	switch p {
	case PanelClaude:
		return m.claude
	case PanelOpenCode:
		return m.opencode
	default:
		return nil
	}
}

func (m Model) panelForAgent(a *agent.Agent) Panel {
	if a == nil {
		return PanelClaude
	}
	switch a.Kind {
	case agent.KindClaude:
		return PanelClaude
	case agent.KindOpenCode:
		return PanelOpenCode
	default:
		return PanelClaude
	}
}

func (m Model) runningAgentsCount() int {
	count := 0
	if m.claude != nil && m.claude.IsRunning() {
		count++
	}
	if m.opencode != nil && m.opencode.IsRunning() {
		count++
	}
	return count
}

func (m *Model) setError(err error) {
	m.err = err
	if err != nil {
		m.statusText = fmt.Sprintf("error: %v", err)
	}
}
