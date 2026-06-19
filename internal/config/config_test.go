package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, ".maestro.toml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Project.Name == "" {
		t.Fatal("expected project name")
	}
	if cfg.Agents.Claude != "claude" {
		t.Fatalf("expected claude binary default, got %q", cfg.Agents.Claude)
	}
	if cfg.Agents.OpenCode != "opencode" {
		t.Fatalf("expected opencode binary default, got %q", cfg.Agents.OpenCode)
	}
}

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	content := `
[project]
name = "testproj"

[agents]
claude = "/usr/bin/claude"
opencode = "/usr/bin/opencode"
`
	path := filepath.Join(dir, ".maestro.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Project.Name != "testproj" {
		t.Fatalf("expected testproj, got %q", cfg.Project.Name)
	}
	if cfg.Agents.Claude != "/usr/bin/claude" {
		t.Fatalf("expected custom claude path, got %q", cfg.Agents.Claude)
	}
}
