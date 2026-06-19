package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Project holds the project-level configuration.
type Project struct {
	Name string `toml:"name"`
	Root string `toml:"root"`
}

// Agents holds paths to the agent binaries.
type Agents struct {
	Claude   string `toml:"claude"`
	OpenCode string `toml:"opencode"`
}

// Workspace holds workspace-level configuration.
type Workspace struct {
	WorktreeBase string `toml:"worktree_base"`
}

// Config is the top-level configuration for maestro.
type Config struct {
	Project   Project   `toml:"project"`
	Agents    Agents    `toml:"agents"`
	Workspace Workspace `toml:"workspace"`
}

// Load reads and parses a .maestro.toml file. If path is empty or the file does
// not exist, it returns a sensible default config rooted at the current
// working directory.
func Load(path string) (*Config, error) {
	if path == "" {
		path = ".maestro.toml"
	}

	cfg := &Config{
		Project: Project{
			Name: "maestro",
			Root: ".",
		},
		Agents: Agents{
			Claude:   "claude",
			OpenCode: "opencode",
		},
		Workspace: Workspace{
			WorktreeBase: ".maestro-worktrees",
		},
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := cfg.normalize("."); err != nil {
			return nil, err
		}
		return cfg, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat config %s: %w", path, err)
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}

	root := filepath.Dir(path)
	if cfg.Project.Root != "" {
		if filepath.IsAbs(cfg.Project.Root) {
			root = cfg.Project.Root
		} else {
			root = filepath.Join(root, cfg.Project.Root)
		}
	}

	if err := cfg.normalize(root); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) normalize(root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return fmt.Errorf("project root %s: %w", absRoot, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("project root %s is not a directory", absRoot)
	}

	c.Project.Root = absRoot
	if c.Project.Name == "" {
		c.Project.Name = filepath.Base(absRoot)
	}

	if !filepath.IsAbs(c.Workspace.WorktreeBase) {
		c.Workspace.WorktreeBase = filepath.Join(absRoot, c.Workspace.WorktreeBase)
	}

	if err := os.MkdirAll(c.Workspace.WorktreeBase, 0o755); err != nil {
		return fmt.Errorf("create worktree base %s: %w", c.Workspace.WorktreeBase, err)
	}

	if c.Agents.Claude == "" {
		c.Agents.Claude = "claude"
	}
	if c.Agents.OpenCode == "" {
		c.Agents.OpenCode = "opencode"
	}

	return nil
}
