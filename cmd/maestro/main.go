package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/biglitecode/maestro/internal/config"
	"github.com/biglitecode/maestro/internal/ui"
	"github.com/biglitecode/maestro/internal/workspace"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var (
		projectRoot = flag.String("project", ".", "path to project root")
		configPath  = flag.String("config", "", "path to .maestro.toml (default: <project>/.maestro.toml)")
	)
	flag.Parse()

	if err := run(*projectRoot, *configPath); err != nil {
		fmt.Fprintf(os.Stderr, "maestro: %v\n", err)
		os.Exit(1)
	}
}

func run(projectRoot, configPath string) error {
	if configPath == "" {
		configPath = findConfig(projectRoot)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !isGitRepo(cfg.Project.Root) {
		return fmt.Errorf("%s is not inside a git worktree", cfg.Project.Root)
	}

	wsManager := workspace.NewManager(cfg.Project.Root, cfg.Workspace.WorktreeBase)

	model := ui.NewModel(cfg, wsManager)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}

func findConfig(projectRoot string) string {
	path := projectRoot
	if path == "" {
		path = "."
	}
	candidate := path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		candidate = path
	}
	cfgFile := candidate + "/.maestro.toml"
	if _, err := os.Stat(cfgFile); err == nil {
		return cfgFile
	}
	return ".maestro.toml"
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
