package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/luccadibe/benchctl/internal/config"
)

// SyncResults pushes the benchmark output directory to the configured rclone remote.
func SyncResults(ctx context.Context, cfg *config.Config) error {
	if cfg.Benchmark.Sync == nil {
		return fmt.Errorf("benchmark.sync must be configured")
	}
	args, err := rcloneSyncArgs(cfg)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "rclone", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone sync failed: %w", err)
	}
	return nil
}

func rcloneSyncArgs(cfg *config.Config) ([]string, error) {
	sync := cfg.Benchmark.Sync
	if sync == nil {
		return nil, fmt.Errorf("benchmark.sync must be configured")
	}
	remote := strings.TrimSpace(sync.Remote)
	if remote == "" {
		return nil, fmt.Errorf("benchmark.sync.remote must be set")
	}
	args := []string{"sync", cfg.Benchmark.OutputDir, remote}
	args = append(args, sync.Args...)
	return args, nil
}
