package broker

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/biglitecode/maestro/internal/agent"
)

// Message is a routed message between agents.
type Message struct {
	From      string
	To        string
	Text      string
	Timestamp time.Time
}

// Broker routes messages between agents and maintains a log.
type Broker struct {
	Claude   *agent.Agent
	OpenCode *agent.Agent

	log    []Message
	mu     sync.RWMutex
	msgCh  chan Message
}

// New creates a broker attached to the given agents.
func New(claude, opencode *agent.Agent) *Broker {
	return &Broker{
		Claude:   claude,
		OpenCode: opencode,
		log:      []Message{},
		msgCh:    make(chan Message, 256),
	}
}

func (b *Broker) target(id string) *agent.Agent {
	switch id {
	case "claude":
		return b.Claude
	case "opencode":
		return b.OpenCode
	default:
		return nil
	}
}

// Send routes a message to the target agent(s) and logs it.
func (b *Broker) Send(fromID, toID, text string) error {
	msg := Message{
		From:      fromID,
		To:        toID,
		Text:      text,
		Timestamp: time.Now(),
	}

	b.mu.Lock()
	b.log = append(b.log, msg)
	b.mu.Unlock()

	select {
	case b.msgCh <- msg:
	default:
	}

	if toID == "broadcast" {
		var errs []string
		if b.Claude != nil {
			if err := b.Claude.Send(text); err != nil {
				errs = append(errs, fmt.Sprintf("claude: %v", err))
			}
		}
		if b.OpenCode != nil {
			if err := b.OpenCode.Send(text); err != nil {
				errs = append(errs, fmt.Sprintf("opencode: %v", err))
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("broadcast failed: %s", strings.Join(errs, "; "))
		}
		return nil
	}

	t := b.target(toID)
	if t == nil {
		return fmt.Errorf("unknown target agent: %s", toID)
	}
	return t.Send(text)
}

// Review forwards the last N output lines from one agent to the other as a review prompt.
func (b *Broker) Review(fromID, toID string) error {
	src := b.target(fromID)
	if src == nil {
		return fmt.Errorf("unknown source agent: %s", fromID)
	}
	if b.target(toID) == nil {
		return fmt.Errorf("unknown target agent: %s", toID)
	}

	lines := src.LastN(50)
	if len(lines) == 0 {
		return fmt.Errorf("agent %s has no output to review", fromID)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("The following is output from %s. Please review it and give feedback:\n\n", fromID))
	for _, line := range lines {
		sb.WriteString(line.Text)
		sb.WriteByte('\n')
	}

	reviewMsg := sb.String()

	meta := Message{
		From:      fromID,
		To:        toID,
		Text:      "[review triggered]",
		Timestamp: time.Now(),
	}
	b.mu.Lock()
	b.log = append(b.log, meta)
	b.mu.Unlock()

	select {
	case b.msgCh <- meta:
	default:
	}

	return b.Send(fromID, toID, reviewMsg)
}

// Log returns a copy of the message log.
func (b *Broker) Log() []Message {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Message, len(b.log))
	copy(out, b.log)
	return out
}

// MsgCh returns the channel that emits new messages.
func (b *Broker) MsgCh() chan Message {
	return b.msgCh
}
