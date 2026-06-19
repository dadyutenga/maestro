package workspace

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config name: %v", err)
	}
	cmd = exec.Command("git", "commit", "-q", "--allow-empty", "-m", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

func TestCreateIsolated(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	base := filepath.Join(dir, "worktrees")
	mgr := NewManager(dir, base)

	ws, err := mgr.CreateIsolated("feat-auth", "feat-auth")
	if err != nil {
		t.Fatalf("CreateIsolated failed: %v", err)
	}
	if ws.Mode != ModeIsolated {
		t.Fatalf("expected isolated mode, got %v", ws.Mode)
	}

	list := mgr.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(list))
	}
}

func TestCreateShared(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared")
	if err := exec.Command("mkdir", "-p", shared).Run(); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mgr := NewManager(dir, filepath.Join(dir, "worktrees"))
	ws, err := mgr.CreateShared("common", shared)
	if err != nil {
		t.Fatalf("CreateShared failed: %v", err)
	}
	if ws.Mode != ModeShared {
		t.Fatalf("expected shared mode, got %v", ws.Mode)
	}
}
