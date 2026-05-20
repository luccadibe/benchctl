//go:build unit

package internal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/luccadibe/benchctl/internal/config"
)

func TestCaptureGitMetadata(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}

	runDir := t.TempDir()
	cfg := config.New("git", runDir)
	metadata, err := CaptureGitMetadata(context.Background(), cfg, runDir)
	if err != nil {
		t.Fatalf("capture git metadata: %v", err)
	}
	if metadata == nil {
		t.Fatalf("expected git metadata")
	}
	if metadata.Commit == "" {
		t.Fatalf("expected commit")
	}
	if !metadata.Dirty {
		t.Fatalf("expected dirty worktree")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
