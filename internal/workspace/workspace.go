package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Mode represents the type of workspace.
type Mode int

const (
	ModeIsolated Mode = iota
	ModeShared
)

func (m Mode) String() string {
	switch m {
	case ModeIsolated:
		return "Isolated"
	case ModeShared:
		return "Shared"
	default:
		return "Unknown"
	}
}

// Workspace represents a working directory that an agent can attach to.
type Workspace struct {
	ID      string
	Name    string
	Mode    Mode
	Dir     string
	Branch  string
	Agents  []string
	mu      sync.RWMutex
}

// Manager handles workspace lifecycle.
type Manager struct {
	projectRoot  string
	worktreeBase string
	workspaces   []*Workspace
	mu           sync.RWMutex
}

// NewManager creates a workspace manager rooted at projectRoot.
func NewManager(projectRoot, worktreeBase string) *Manager {
	return &Manager{
		projectRoot:  projectRoot,
		worktreeBase: worktreeBase,
		workspaces:   []*Workspace{},
	}
}

// CreateIsolated adds a git worktree and registers it.
func (m *Manager) CreateIsolated(name, branch string) (*Workspace, error) {
	if name == "" {
		return nil, fmt.Errorf("workspace name cannot be empty")
	}
	if branch == "" {
		branch = name
	}

	dir := filepath.Join(m.worktreeBase, name)
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("worktree directory already exists: %s", dir)
	}

	cmd := exec.Command("git", "worktree", "add", dir, "-b", branch)
	cmd.Dir = m.projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree add failed: %w\n%s", err, string(out))
	}

	ws := &Workspace{
		ID:     name,
		Name:   name,
		Mode:   ModeIsolated,
		Dir:    dir,
		Branch: branch,
	}

	m.mu.Lock()
	m.workspaces = append(m.workspaces, ws)
	m.mu.Unlock()

	return ws, nil
}

// CreateShared registers a shared directory as a workspace.
func (m *Manager) CreateShared(name, dir string) (*Workspace, error) {
	if name == "" {
		return nil, fmt.Errorf("workspace name cannot be empty")
	}
	if dir == "" {
		return nil, fmt.Errorf("workspace directory cannot be empty")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("workspace directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace path %s is not a directory", dir)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace directory: %w", err)
	}

	ws := &Workspace{
		ID:   name,
		Name: name,
		Mode: ModeShared,
		Dir:  absDir,
	}

	m.mu.Lock()
	m.workspaces = append(m.workspaces, ws)
	m.mu.Unlock()

	return ws, nil
}

// Remove deletes a workspace and, for isolated workspaces, removes the git worktree.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, ws := range m.workspaces {
		if ws.ID != id {
			continue
		}

		if ws.Mode == ModeIsolated {
			cmd := exec.Command("git", "worktree", "remove", ws.Dir, "--force")
			cmd.Dir = m.projectRoot
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git worktree remove failed: %w\n%s", err, string(out))
			}
		}

		m.workspaces = append(m.workspaces[:i], m.workspaces[i+1:]...)
		return nil
	}

	return fmt.Errorf("workspace not found: %s", id)
}

// List returns all workspaces.
func (m *Manager) List() []*Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Workspace, len(m.workspaces))
	copy(out, m.workspaces)
	return out
}

// Get returns a workspace by ID or nil if not found.
func (m *Manager) Get(id string) *Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ws := range m.workspaces {
		if ws.ID == id {
			return ws
		}
	}
	return nil
}

// AttachAgent records that an agent is using this workspace.
func (ws *Workspace) AttachAgent(agentID string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	for _, id := range ws.Agents {
		if id == agentID {
			return
		}
	}
	ws.Agents = append(ws.Agents, agentID)
}

// DetachAgent removes an agent from this workspace.
func (ws *Workspace) DetachAgent(agentID string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	filtered := ws.Agents[:0]
	for _, id := range ws.Agents {
		if id != agentID {
			filtered = append(filtered, id)
		}
	}
	ws.Agents = filtered
}

// ListAgents returns the agents attached to this workspace.
func (ws *Workspace) ListAgents() []string {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	out := make([]string, len(ws.Agents))
	copy(out, ws.Agents)
	return out
}
