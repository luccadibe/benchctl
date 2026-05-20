package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/luccadibe/benchctl/internal/config"
)

// GitMetadata describes the source repository state captured for a run.
type GitMetadata struct {
	Root            string `json:"root"`
	Commit          string `json:"commit"`
	Branch          string `json:"branch,omitempty"`
	RemoteURL       string `json:"remote_url,omitempty"`
	Dirty           bool   `json:"dirty"`
	StatusPorcelain string `json:"status_porcelain,omitempty"`
	PatchPath       string `json:"patch_path,omitempty"`
}

// CaptureGitMetadata captures git metadata for the current working directory.
func CaptureGitMetadata(ctx context.Context, cfg *config.Config, runDir string) (*GitMetadata, error) {
	gitConfig := cfg.Benchmark.Git
	if gitConfig != nil && gitConfig.Capture != nil && !*gitConfig.Capture {
		return nil, nil
	}

	root, err := gitOutput(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil
	}
	root = strings.TrimSpace(root)

	commit, err := gitOutput(ctx, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("capture git commit: %w", err)
	}
	branch, _ := gitOutput(ctx, "branch", "--show-current")
	remote, _ := gitOutput(ctx, "config", "--get", "remote.origin.url")
	status, err := gitOutput(ctx, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("capture git status: %w", err)
	}

	metadata := &GitMetadata{
		Root:            root,
		Commit:          strings.TrimSpace(commit),
		Branch:          strings.TrimSpace(branch),
		RemoteURL:       strings.TrimSpace(remote),
		StatusPorcelain: strings.TrimSpace(status),
	}
	metadata.Dirty = metadata.StatusPorcelain != ""

	if gitConfig != nil && gitConfig.RequireClean && metadata.Dirty {
		return nil, fmt.Errorf("git worktree is dirty")
	}
	if gitConfig != nil && gitConfig.SavePatch && metadata.Dirty {
		patch, err := gitOutput(ctx, "diff", "--binary")
		if err != nil {
			return nil, fmt.Errorf("capture git patch: %w", err)
		}
		patchPath := filepath.Join(runDir, "git.patch")
		if err := os.WriteFile(patchPath, []byte(patch), 0644); err != nil {
			return nil, fmt.Errorf("write git patch: %w", err)
		}
		metadata.PatchPath = filepath.Base(patchPath)
	}

	return metadata, nil
}

func gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
