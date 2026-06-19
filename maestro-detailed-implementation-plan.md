# 🎼 Maestro — Detailed Implementation Plan
### Local Terminal Agent Manager for Claude Code & OpenCode
**By BIG LITE CODE | Iringa, Tanzania 🇹🇿**

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Technical Stack & Prerequisites](#2-technical-stack--prerequisites)
3. [Project Structure Deep Dive](#3-project-structure-deep-dive)
4. [Component Specifications](#4-component-specifications)
   - 4.1 Config Loader
   - 4.2 Workspace Manager
   - 4.3 Agent Runner
   - 4.4 Broker
   - 4.5 TUI — Model
   - 4.6 TUI — Panels & Rendering
   - 4.7 TUI — Keybindings & Update
5. [Data Flow & Communication](#5-data-flow--communication)
6. [Phase-by-Phase Build Plan](#6-phase-by-phase-build-plan)
7. [Deliverables Per Phase](#7-deliverables-per-phase)
8. [Testing Strategy](#8-testing-strategy)
9. [Known Challenges & Mitigations](#9-known-challenges--mitigations)

---

## 1. Project Overview

Maestro is a **single binary** Go CLI tool that runs in your terminal and gives you a split-panel TUI to:

- Spawn Claude Code and OpenCode as live subprocesses
- Manage git worktrees (isolated workspaces) or shared directories per session
- Route tasks and handoff messages between agents via an internal broker
- See both agents' output streams side by side in real time
- Trigger cross-agent code review with a single keypress

**Design constraints:**
- No network calls — 100% local
- No database — state lives in memory for the session duration
- No auth, no config server — one `.maestro.toml` per project
- Single binary output — `go build` produces one executable, nothing else

---

## 2. Technical Stack & Prerequisites

### Go Packages You Will Install

| Package | Version | Role |
|---|---|---|
| `charmbracelet/bubbletea` | v0.26.x | TUI event loop (the backbone) |
| `charmbracelet/lipgloss` | v0.11.x | Terminal styling — borders, colors, padding |
| `charmbracelet/bubbles` | v0.18.x | Pre-built components: viewport, textarea, spinner, list |
| `BurntSushi/toml` | v1.x | Parse `.maestro.toml` config file |

### System Prerequisites on Your Fedora 44

- Go 1.22+ installed (`go version`)
- `claude` CLI binary on PATH (from Anthropic)
- `opencode` binary on PATH
- `git` 2.x (for worktree commands)
- Terminal that supports 256 colors (your setup already does)

### Initialize The Project

```
mkdir maestro && cd maestro
go mod init github.com/biglitecode/maestro
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
go get github.com/charmbracelet/bubbles
go get github.com/BurntSushi/toml
```

---

## 3. Project Structure Deep Dive

Every file and what lives inside it:

```
maestro/
│
├── cmd/
│   └── maestro/
│       └── main.go
│           Purpose: Entry point only. Parses CLI flags (--project flag
│           to specify project root), loads config, initializes all
│           components, hands control to the TUI. Nothing else lives here.
│
├── internal/
│   │
│   ├── config/
│   │   └── config.go
│   │       Purpose: Defines the Config struct that maps to .maestro.toml.
│   │       Exposes a Load(path string) function that reads and parses
│   │       the toml file. Returns a Config or an error. No side effects.
│   │
│   ├── workspace/
│   │   └── workspace.go
│   │       Purpose: All git worktree operations. Knows how to create
│   │       isolated worktrees (git worktree add), list them, remove them,
│   │       and validate that a path is a proper git repo. Also handles
│   │       the "shared" mode which is just a directory pointer with no
│   │       git worktree involved.
│   │
│   ├── agent/
│   │   └── agent.go
│   │       Purpose: Manages one agent subprocess (either claude or
│   │       opencode). Owns the exec.Cmd, the stdin pipe for sending
│   │       messages, and goroutines that drain stdout/stderr into a
│   │       channel. Exposes Start(), Send(msg), Kill(), Output(), Status().
│   │
│   ├── broker/
│   │   └── broker.go
│   │       Purpose: The message bus. Holds references to both agents.
│   │       Exposes Send(from, to, msg) which logs the message and
│   │       forwards it to the target agent's stdin. Also exposes
│   │       Review(from, to) which grabs the last N output lines from
│   │       "from" agent, formats them as a review prompt, and sends
│   │       to "to" agent. Maintains a []Message log for the broker panel.
│   │
│   └── ui/
│       ├── model.go
│       │   Purpose: The root Bubbletea Model struct. Holds ALL state:
│       │   list of workspaces, both agent references, broker reference,
│       │   which panel is focused, input bar content, panel scroll
│       │   positions. This is the single source of truth for the TUI.
│       │
│       ├── update.go
│       │   Purpose: The Bubbletea Update(msg) function. Handles all
│       │   events: keypresses, agent output messages arriving on channels,
│       │   window resize events, workspace creation/deletion events.
│       │   Returns (Model, tea.Cmd) — never mutates state directly.
│       │
│       ├── view.go
│       │   Purpose: The Bubbletea View() function. Pure render — takes
│       │   model state and returns a string. Uses lipgloss to compose
│       │   the 4-panel layout. Never reads from channels or runs I/O.
│       │
│       ├── panels.go
│       │   Purpose: Individual panel renderers called by view.go.
│       │   renderWorkspacePanel(), renderAgentPanel(), renderBrokerPanel(),
│       │   renderInputBar(). Each takes the model and returns a lipgloss-
│       │   styled string for its section.
│       │
│       └── keys.go
│           Purpose: Defines all keybinding constants and the keymap
│           struct using bubbles/key package. Central place — if you
│           want to change a shortcut, this is the only file you touch.
│
├── .maestro.toml         ← example config committed to repo
├── go.mod
├── go.sum
├── Makefile              ← build/install shortcuts
└── README.md
```

---

## 4. Component Specifications

### 4.1 Config Loader (`internal/config/config.go`)

**What It Holds:**

The Config struct has three sections mirroring the toml file:

- `Project` section: name (string), root path (string)
- `Agents` section: path to claude binary, path to opencode binary
- `Workspace` section: where to put worktree directories (string path)

**What It Does:**

The `Load()` function receives a file path, reads the toml file, decodes it into the Config struct, validates that required fields are not empty, and returns the struct. If the file doesn't exist, it returns a sensible default config pointing to the current directory.

**Validation rules it enforces:**
- Project root must exist on disk
- Worktree base path: if it doesn't exist, Config creates it automatically
- Agent binary paths: if not specified, it assumes `claude` and `opencode` are on PATH (it doesn't validate PATH — that error surfaces when Agent tries to Start())

---

### 4.2 Workspace Manager (`internal/workspace/workspace.go`)

**The Workspace struct holds:**
- ID (a short slug like `feat-auth`)
- Display name
- Mode: either Isolated or Shared (an enum/const)
- Directory path (where the agent will run)
- Branch name (only meaningful for Isolated mode)
- Which agent(s) are attached to it (agent IDs)

**What It Does:**

`CreateIsolated(name, branch string)` — runs `git worktree add <worktree_base>/<name> -b <branch>` as an exec.Command. Returns the new Workspace or error. The worktree directory becomes the working directory for the agent attached to this workspace.

`CreateShared(name, dir string)` — no git commands. Just creates a Workspace struct pointing to `dir`. Validates the dir exists.

`Remove(id string)` — for Isolated mode: runs `git worktree remove <path> --force`. For Shared: just deletes the struct (does not touch the actual directory, since you might still be using it).

`List()` — returns all current workspaces as a slice.

**Important detail:** The workspace manager must run all git commands with the project root as the working directory (not the worktree path), because `git worktree add` must be run from the main repo.

---

### 4.3 Agent Runner (`internal/agent/agent.go`)

**The Agent struct holds:**
- ID string
- Kind (Claude or OpenCode — a typed const)
- WorkspaceDir (where to run — comes from Workspace)
- Status (Idle / Running / Done / Error — typed const)
- The exec.Cmd pointer
- The stdin WriteCloser pipe
- A slice of output lines (for scrollback)
- A mutex protecting the output slice and status
- An output channel (`chan OutputLine`) that the TUI listens to

**OutputLine struct holds:** agent ID, text string, timestamp.

**What It Does:**

`Start()` — builds the exec.Cmd for either `claude` or `opencode`, sets the working directory to WorkspaceDir, creates stdin/stdout/stderr pipes, starts the process, launches two goroutines (one draining stdout, one draining stderr — both send to the same OutputCh), launches a third goroutine that waits for process exit and updates Status.

`Send(msg string)` — acquires the mutex, writes `msg + newline` to stdin pipe. This is how you send tasks and broker messages to the agent.

`Kill()` — sends SIGKILL to the process, updates Status to Done.

`Output()` — returns a copy of the output slice (thread-safe, uses mutex).

`LastN(n int)` — convenience wrapper over Output() returning last N lines. Used by the Broker when building a review prompt.

**Goroutine model:**
Each agent has exactly 3 goroutines running while alive: stdout drain, stderr drain, exit watcher. All three are started in Start() and die when the process exits or is killed. No goroutine leaks because they all block on I/O that closes when the process dies.

---

### 4.4 Broker (`internal/broker/broker.go`)

**The Broker struct holds:**
- A reference to Agent A (Claude)
- A reference to Agent B (OpenCode)
- A `[]Message` log slice
- A mutex protecting the log
- A `chan Message` that the TUI listens to for updating the broker panel

**Message struct holds:** From (agent ID), To (agent ID or "broadcast"), Text, Timestamp.

**What It Does:**

`Send(fromID, toID, text string)` — creates a Message, appends to log, emits on channel, then calls the target agent's `Send()` with the text. If toID is "broadcast", calls Send() on both agents.

`Review(fromID, toID string)` — this is the core cross-review feature. It:
1. Gets LastN(50) output lines from the "from" agent
2. Formats them into a review prompt string: "The following is output from [agent]. Please review it and give feedback:" followed by the lines
3. Calls `Send(fromID, toID, prompt)` — which logs it in the broker panel AND sends it to the target agent's stdin

`Log()` — returns a copy of the message log for rendering.

**Why the Broker owns review logic and not the agent:** Agents are deliberately dumb — they just run a process and pipe I/O. The Broker is the intelligent coordinator. This separation means you can change review prompt format, add more agents, or change routing logic without touching agent code.

---

### 4.5 TUI — Model (`internal/ui/model.go`)

**The Model struct holds everything the TUI needs:**

- `workspaces []workspace.Workspace` — the list shown in the left panel
- `selectedWorkspace int` — cursor index in workspace list
- `agentClaude *agent.Agent` — may be nil if not spawned yet
- `agentOpenCode *agent.Agent` — may be nil if not spawned yet
- `broker *broker.Broker`
- `focusedPanel Panel` — enum: PanelWorkspaces / PanelClaude / PanelOpenCode / PanelBroker / PanelInput
- `claudeViewport viewport.Model` — bubbles viewport for Claude's scrollable output
- `opencodeViewport viewport.Model` — same for OpenCode
- `brokerViewport viewport.Model` — same for broker messages
- `workspaceList list.Model` — bubbles list component for workspace panel
- `inputBar textarea.Model` — bubbles textarea for input
- `termWidth, termHeight int` — updated on WindowSizeMsg
- `config *config.Config`
- `err error` — last error to display in status bar

**Model also defines** the `Init()` function which returns a `tea.Batch` of commands: one command per agent that listens on that agent's OutputCh and emits a Bubbletea message when output arrives. This is how agent output gets into the TUI event loop without blocking.

---

### 4.6 TUI — Panels & Rendering (`internal/ui/panels.go` + `view.go`)

**Overall layout strategy:**

The terminal is divided into rows and columns using lipgloss. The total width and height come from `termWidth` and `termHeight` on the model (updated via `tea.WindowSizeMsg`).

Layout math:
- Left column (Workspaces panel): 20% of terminal width
- Right area: 80% of terminal width, split 50/50 between Claude panel and OpenCode panel
- Broker panel: full width, fixed height of 6 lines below the agent panels
- Input bar: full width, fixed height of 3 lines at the very bottom
- Status bar: 1 line at the very top

**Panel borders:** Each panel has a lipgloss border. The focused panel gets a highlighted border color (cyan or green) so you always know where your input will go.

**Agent panel content:** Shows the last N lines of agent output that fit in the viewport height. Uses the bubbles `viewport` component so you can scroll up to see history.

**Workspace panel content:** Uses the bubbles `list` component. Each item shows: a dot indicator (filled if an agent is attached, empty if not), the workspace name, the mode (Isolated/Shared), and the branch name for isolated ones.

**Broker panel content:** Shows the message log as `[HH:MM] CC→OC: message text`. Scrollable via bubbles viewport.

**Status bar (top line):** Shows project name, current git branch of project root, how many agents are running, and keyboard hint reminders.

---

### 4.7 TUI — Keybindings & Update (`internal/ui/keys.go` + `update.go`)

**keys.go defines a keymap struct with these bindings:**

| Action | Key | Scope |
|---|---|---|
| Focus next panel | Tab | Global |
| Focus previous panel | Shift+Tab | Global |
| New workspace | n | When Workspaces panel focused |
| Delete workspace | d | When Workspaces panel focused |
| Spawn agent on workspace | s | When Workspaces panel focused |
| Kill focused agent | k | When agent panel focused |
| Review (send output to other agent) | r | When agent panel focused |
| Broadcast to both agents | b | When input bar focused |
| Clear agent panel | c | When agent panel focused |
| Scroll up | ↑ or k | When agent/broker panel focused |
| Scroll down | ↓ or j | When agent/broker panel focused |
| Send input | Enter | When input bar focused |
| Quit | q or Ctrl+C | Global |

**update.go handles these Bubbletea message types:**

`tea.KeyMsg` — keyboard events. Dispatches to the correct handler based on which panel is focused and which key was pressed.

`tea.WindowSizeMsg` — terminal was resized. Recalculates all panel dimensions, updates viewport sizes on model.

`AgentOutputMsg` (custom) — arrives when an agent emits a new output line. Updates the correct viewport content and returns a new command to keep listening on the channel (this is the polling loop that keeps agent output flowing into the TUI).

`WorkspaceCreatedMsg` (custom) — arrives after the user goes through the new-workspace creation flow. Adds the workspace to the model's list.

`ErrorMsg` (custom) — any error from any async operation. Sets `model.err` which the status bar displays.

**The new-workspace creation flow:** When user presses `n`, the input bar switches into a mini-wizard mode (asks workspace name, then mode, then branch name for isolated). This is managed by a small state machine on the model (`inputMode` field: Normal / CreatingWorkspaceName / CreatingWorkspaceMode / CreatingWorkspaceBranch). After collecting all inputs, it fires an `exec.Command` via a Bubbletea command and returns a `WorkspaceCreatedMsg`.

---

## 5. Data Flow & Communication

### How Agent Output Gets Into The TUI

This is the most important architectural detail. Bubbletea is single-threaded — its Update() runs on one goroutine. Agent subprocesses produce output on background goroutines. These two worlds connect via channels + Bubbletea commands:

1. Agent's stdout goroutine writes `OutputLine` to `agent.OutputCh` (buffered, capacity 256)
2. TUI has a `tea.Cmd` that blocks on `<-agent.OutputCh` and returns an `AgentOutputMsg` when a line arrives
3. Bubbletea's event loop receives `AgentOutputMsg`, calls Update(), which appends to the viewport and immediately re-issues the same `tea.Cmd` to keep listening
4. This creates a continuous polling loop with no busy-waiting

### How A Task Gets To An Agent

1. User types task in input bar, presses Enter
2. Update() sees `tea.KeyMsg{Type: tea.KeyEnter}` while `focusedPanel == PanelInput`
3. Checks which agent panel was focused before input bar (model tracks `lastFocusedAgent`)
4. Calls `agent.Send(inputBar.Value())`
5. Agent.Send() writes to stdin pipe → subprocess receives it → subprocess produces output → output flows back via the channel loop above

### How Cross-Review Works

1. User focuses Claude panel, presses `r`
2. Update() calls `broker.Review("claude", "opencode")`
3. Broker gets Claude's last 50 output lines
4. Broker formats them: "Please review this output from Claude Code and give feedback:\n<lines>"
5. Broker calls `opencode.Send(prompt)`
6. OpenCode subprocess receives the review prompt on stdin and starts responding
7. OpenCode's response flows back through its OutputCh → TUI shows it in OpenCode panel
8. Broker also logs a `Message{From:"claude", To:"opencode", Text:"[review triggered]"}` so broker panel shows what happened

---

## 6. Phase-by-Phase Build Plan

### Phase 1 — Project Skeleton (Day 1)

**Goal:** A TUI that boots, shows the 4-panel layout with placeholder content, and handles Tab to cycle focus and Q to quit. No real functionality — just the shell.

**What you build:**
- `go.mod` with all dependencies
- `cmd/maestro/main.go` — boots bubbletea with an empty model
- `internal/ui/model.go` — Model struct with just focusedPanel and terminal dimensions
- `internal/ui/view.go` — renders 4 static panels using lipgloss, highlights focused panel border
- `internal/ui/update.go` — handles Tab, Shift+Tab, Q, WindowSizeMsg
- `internal/ui/keys.go` — keymap definitions
- `Makefile` with `make build`, `make run`, `make install`

**How to verify:** Run `make run`, see the 4-panel layout, Tab cycles the highlighted border, Q quits cleanly.

---

### Phase 2 — Config & Workspace (Day 2)

**Goal:** Load `.maestro.toml`, show project name in status bar, create/list/delete workspaces in the left panel.

**What you build:**
- `internal/config/config.go` — Config struct + Load()
- `internal/workspace/workspace.go` — Workspace struct, CreateIsolated(), CreateShared(), Remove(), List()
- Wire config loading into main.go
- Wire workspace list into model + workspace panel rendering
- Implement `n` key flow (mini-wizard in input bar)
- Implement `d` key (delete selected workspace)

**How to verify:** Create a `.maestro.toml` in a test git repo. Run maestro. Press `n`, follow the wizard, see the workspace appear in the left panel. Press `d`, see it disappear. Verify with `git worktree list` in your terminal that worktrees were actually created/removed.

---

### Phase 3 — Agent Subprocess (Day 3-4)

**Goal:** Press `s` on a workspace, an agent spawns, its output streams into the correct panel in real time. Input bar sends text to the agent.

**What you build:**
- `internal/agent/agent.go` — full implementation: Start(), Send(), Kill(), Output(), LastN(), goroutines, OutputCh
- Add `AgentOutputMsg` custom message type to update.go
- Add the channel-listening tea.Cmd (the polling loop) to model Init()
- Wire `s` key: spawn agent on selected workspace
- Wire `k` key: kill agent
- Wire Enter in input bar: send to focused agent
- Update agent panel to display streaming output via viewport

**How to verify:** Spawn claude on a workspace. Type a simple task like "list files in this directory". See the response stream into the Claude panel. Try the same with opencode.

**Important:** At this phase you may find that `claude` CLI or `opencode` don't behave well with pure stdin/stdout pipes — they might use a PTY (pseudo-terminal). If that happens, you'll need to use a Go PTY library (`creack/pty`) to spawn them with a proper PTY. Plan for this as a contingency. Test early.

---

### Phase 4 — Broker & Cross-Review (Day 5)

**Goal:** Broker panel shows message log. `r` key triggers cross-review. Explicit message send between agents works.

**What you build:**
- `internal/broker/broker.go` — Broker struct, Send(), Review(), Log(), BrokerMsgCh
- Add `BrokerMsgMsg` custom message type and its polling loop in update.go
- Wire `r` key: call broker.Review() based on focused agent panel
- Wire `b` key: broker.Send() to both (broadcast)
- Render broker panel with message log + viewport scrolling

**How to verify:** Spawn both agents. Give Claude a task. When it responds, press `r` in Claude panel. See the review prompt appear in OpenCode panel's input. See the broker panel log the handoff. See OpenCode's response appear in its panel.

---

### Phase 5 — Polish & Ship (Day 6-7)

**Goal:** Handle all edge cases, make it feel professional, write README, tag v1.0.

**What you build/fix:**
- Error states: agent binary not found, git not a repo, worktree already exists — all surface in status bar, don't crash
- Agent status indicator: show spinner (bubbles spinner component) when agent is actively outputting
- `c` key: clear panel output (reset viewport content)
- Scrollback: ensure up/down arrow scrolls correctly in all panels
- Colors: define a consistent palette in a `theme.go` file (one source of truth for all lipgloss colors)
- Handle terminal resize gracefully at all times
- `Makefile` targets: `make build`, `make install`, `make clean`, `make release` (cross-compile for linux/amd64)
- README.md with install instructions, usage, screenshots (take with `vhs` or manually)
- Tag `v1.0.0` on git

---

## 7. Deliverables Per Phase

| Phase | Deliverables |
|---|---|
| 1 — Skeleton | Binary boots, 4-panel TUI renders, Tab/Q work, no crashes |
| 2 — Config/Workspace | `.maestro.toml` loads, workspaces create/delete, git worktrees verified |
| 3 — Agents | Both agents spawn, output streams live, input sends tasks, Kill works |
| 4 — Broker | Broker panel live, cross-review sends output between agents, broadcast works |
| 5 — Polish | Error handling, spinners, scroll, README, v1.0.0 tag, installable binary |

---

## 8. Testing Strategy

### Manual Testing (Primary)

Each phase ends with a manual checklist. You test against a real git repo on your Fedora machine with real `claude` and `opencode` binaries.

### Unit Tests (Secondary — Phase 5)

- `config_test.go` — test Load() with valid toml, missing file, invalid toml
- `workspace_test.go` — test Create/Remove in a temp git repo (use `os.MkdirTemp` + `git init`)
- `broker_test.go` — test message routing with mock agents (agent stubs that implement a Send() interface)
- `agent_test.go` — test that Start() fails gracefully when binary doesn't exist

### No TUI Unit Tests

Bubbletea's Update() and View() are hard to unit test meaningfully. Test them manually. The Bubbletea project itself recommends manual testing for TUI code.

---

## 9. Known Challenges & Mitigations

### Challenge 1: PTY vs Pipe

`claude` CLI and `opencode` may detect that stdout is not a terminal and either refuse to run, output garbage escape codes, or buffer output forever.

**Mitigation:** In Phase 3, first try plain stdin/stdout pipes. If agent output doesn't appear or the binary crashes, switch to `github.com/creack/pty` to spawn with a proper PTY. The rest of the architecture doesn't change — you just swap the pipe setup in `agent.go`.

### Challenge 2: Agent Output Parsing

Agents output human-readable text mixed with ANSI escape codes. Raw display in the TUI panel will look messy.

**Mitigation:** Strip ANSI codes before storing output lines. Use a simple regex or the `github.com/acarl005/stripansi` package. Store clean text. This also makes broker review prompts clean when forwarding output between agents.

### Challenge 3: Bubbletea & Blocking Channels

If the channel polling tea.Cmd blocks forever when an agent is idle, it still holds a goroutine. This is fine — Go goroutines are cheap. But make sure the agent OutputCh is always closed when the process exits so the goroutine unblocks and the tea.Cmd returns a "done" message instead of hanging.

**Mitigation:** In the exit watcher goroutine in agent.go, after `cmd.Wait()` returns, close the OutputCh. The polling tea.Cmd will receive the zero value and can return an `AgentExitedMsg` to the TUI.

### Challenge 4: Window Resize Recalculation

When the terminal resizes, all panel dimensions must be recalculated and all viewport sizes updated, or the layout breaks.

**Mitigation:** On `tea.WindowSizeMsg`, recalculate all panel widths and heights using the same math as in view.go, then call `.SetSize()` on all three viewports and the textarea. Do this in update.go before returning the new model.

### Challenge 5: Worktree In Non-Git Directory

If the user runs maestro in a directory that is not a git repo, `git worktree add` will fail.

**Mitigation:** In main.go, before starting the TUI, run `git rev-parse --is-inside-work-tree` as an exec.Command. If it fails, print a clear error message and exit before the TUI even starts.

---

*Maestro — Built by BIG LITE CODE | Iringa, Tanzania 🇹🇿*
*"One conductor. Two agents. Infinite output."*
