package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/creack/pty"
)

// Kind identifies the agent type.
type Kind int

const (
	KindClaude Kind = iota
	KindOpenCode
)

func (k Kind) String() string {
	switch k {
	case KindClaude:
		return "claude"
	case KindOpenCode:
		return "opencode"
	default:
		return "unknown"
	}
}

// Status represents the runtime state of an agent.
type Status int

const (
	StatusIdle Status = iota
	StatusRunning
	StatusDone
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusIdle:
		return "idle"
	case StatusRunning:
		return "running"
	case StatusDone:
		return "done"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// OutputLine is one line emitted by an agent.
type OutputLine struct {
	AgentID   string
	Text      string
	Timestamp time.Time
}

// Agent manages a single agent subprocess.
type Agent struct {
	ID           string
	Kind         Kind
	WorkspaceDir string
	Binary       string
	Status       Status
	StatusErr    error

	OutputCh chan OutputLine

	cmd    *exec.Cmd
	ptmx   *os.File
	mu     sync.RWMutex
	lines  []OutputLine
	doneCh chan struct{}
}

// New creates an unstarted agent.
func New(id string, kind Kind, binary, workspaceDir string) *Agent {
	return &Agent{
		ID:           id,
		Kind:         kind,
		Binary:       binary,
		WorkspaceDir: workspaceDir,
		Status:       StatusIdle,
		OutputCh:     make(chan OutputLine, 256),
		doneCh:       make(chan struct{}),
	}
}

// Start launches the agent subprocess and begins draining output.
func (a *Agent) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Status == StatusRunning {
		return fmt.Errorf("agent %s is already running", a.ID)
	}

	a.cmd = exec.Command(a.Binary)
	a.cmd.Dir = a.WorkspaceDir

	ptmx, err := pty.Start(a.cmd)
	if err != nil {
		a.Status = StatusError
		a.StatusErr = fmt.Errorf("pty start %s: %w", a.Binary, err)
		return a.StatusErr
	}
	a.ptmx = ptmx
	a.Status = StatusRunning

	go a.drain(ptmx)
	go a.wait()

	return nil
}

func (a *Agent) drain(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		raw := scanner.Text()
		clean := stripansi.Strip(raw)
		clean = strings.TrimRight(clean, "\r\n")
		if clean == "" {
			continue
		}
		line := OutputLine{
			AgentID:   a.ID,
			Text:      clean,
			Timestamp: time.Now(),
		}
		a.mu.Lock()
		a.lines = append(a.lines, line)
		a.mu.Unlock()

		select {
		case a.OutputCh <- line:
		default:
		}
	}
}

func (a *Agent) wait() {
	err := a.cmd.Wait()
	a.mu.Lock()
	if a.ptmx != nil {
		a.ptmx.Close()
	}
	if err != nil && a.Status != StatusDone {
		a.Status = StatusError
		a.StatusErr = err
	} else {
		a.Status = StatusDone
	}
	a.mu.Unlock()
	close(a.OutputCh)
	close(a.doneCh)
}

// Send writes a message to the agent's stdin followed by a newline.
func (a *Agent) Send(msg string) error {
	a.mu.RLock()
	ptmx := a.ptmx
	status := a.Status
	a.mu.RUnlock()

	if status != StatusRunning {
		return fmt.Errorf("agent %s is not running (status: %s)", a.ID, status)
	}
	if ptmx == nil {
		return fmt.Errorf("agent %s has no stdin", a.ID)
	}

	_, err := fmt.Fprintln(ptmx, msg)
	return err
}

// Kill terminates the agent subprocess.
func (a *Agent) Kill() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cmd == nil || a.cmd.Process == nil {
		return nil
	}
	if err := a.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("kill agent %s: %w", a.ID, err)
	}
	a.Status = StatusDone
	return nil
}

// Output returns a thread-safe copy of all output lines.
func (a *Agent) Output() []OutputLine {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]OutputLine, len(a.lines))
	copy(out, a.lines)
	return out
}

// LastN returns the last n output lines.
func (a *Agent) LastN(n int) []OutputLine {
	lines := a.Output()
	if n >= len(lines) {
		return lines
	}
	return lines[len(lines)-n:]
}

// IsRunning reports whether the agent is currently running.
func (a *Agent) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Status == StatusRunning
}
