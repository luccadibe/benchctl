//go:build unit

package internal

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luccadibe/benchctl/internal/config"
)

func TestExecuteCleanupRunsAfterStageFailure(t *testing.T) {
	tempDir := t.TempDir()
	runDir := filepath.Join(tempDir, "run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("failed to create run dir: %v", err)
	}

	markerPath := filepath.Join(tempDir, "cleanup.ran")
	cfg := &config.Config{
		Benchmark: config.Benchmark{
			Name:      "cleanup-after-failure",
			OutputDir: runDir,
		},
		Hosts: map[string]config.Host{
			"local": {},
		},
		Stages: []config.Stage{
			{
				Name:    "fail",
				Command: "exit 1",
			},
		},
		Cleanup: []config.Cleanup{
			{
				Name:    "mark",
				Command: "touch '" + markerPath + "'",
			},
		},
	}

	metadata := &RunMetadata{
		RunID:         "1",
		BenchmarkName: cfg.Benchmark.Name,
		Hosts:         cfg.Hosts,
		Custom:        map[string]string{},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	backgroundMgr := newBackgroundManager(logger)
	ctx := context.Background()

	stageErr := executeStages(ctx, cfg, "1", runDir, logger, io.Discard, metadata, backgroundMgr, nil)
	if stageErr == nil {
		t.Fatal("expected stage failure")
	}

	if err := executeCleanup(ctx, cfg, "1", runDir, logger, io.Discard, nil); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}

	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("expected cleanup marker file: %v", err)
	}
}

func TestExecuteCleanupFailsFast(t *testing.T) {
	tempDir := t.TempDir()
	runDir := filepath.Join(tempDir, "run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("failed to create run dir: %v", err)
	}

	counterPath := filepath.Join(tempDir, "counter.txt")
	command := "count=0; if [ -f '" + counterPath + "' ]; then count=$(cat '" + counterPath + "'); fi; count=$((count+1)); echo $count > '" + counterPath + "'; if [ $count -eq 1 ]; then exit 1; fi"

	cfg := &config.Config{
		Benchmark: config.Benchmark{
			Name:      "cleanup-fail-fast",
			OutputDir: runDir,
		},
		Hosts: map[string]config.Host{
			"local": {},
		},
		Cleanup: []config.Cleanup{
			{Name: "first", Command: command},
			{Name: "second", Command: command},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := executeCleanup(context.Background(), cfg, "1", runDir, logger, io.Discard, nil)
	if err == nil {
		t.Fatal("expected cleanup failure")
	}

	data, readErr := os.ReadFile(counterPath)
	if readErr != nil {
		t.Fatalf("failed reading counter: %v", readErr)
	}
	if string(data) != "1\n" && string(data) != "1" {
		t.Fatalf("expected only first cleanup step to run, got %q", string(data))
	}
}

func TestRunWorkflowJoinsStageAndCleanupErrors(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "results")
	cfg := &config.Config{
		Benchmark: config.Benchmark{
			Name:      "joined-errors",
			OutputDir: outputDir,
			Git:       &config.GitConfig{Capture: config.Bool(false)},
		},
		Hosts: map[string]config.Host{
			"local": {},
		},
		Stages: []config.Stage{
			{Name: "fail", Command: "exit 1"},
		},
		Cleanup: []config.Cleanup{
			{Name: "also-fail", Command: "exit 2"},
		},
	}

	_, err := RunWorkflow(context.Background(), cfg, nil, nil)
	if err == nil {
		t.Fatal("expected workflow failure")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "workflow failed") {
		t.Fatalf("expected wrapped workflow error, got: %v", err)
	}
	if !strings.Contains(errMsg, "stage fail failed") {
		t.Fatalf("expected stage error in joined result, got: %v", err)
	}
	if !strings.Contains(errMsg, "cleanup also-fail failed") {
		t.Fatalf("expected cleanup error in joined result, got: %v", err)
	}
}

func TestExecuteCleanupUsesRunLevelEnvOnly(t *testing.T) {
	tempDir := t.TempDir()
	runDir := filepath.Join(tempDir, "run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("failed to create run dir: %v", err)
	}

	outputPath := filepath.Join(tempDir, "env.txt")
	cfg := &config.Config{
		Benchmark: config.Benchmark{
			Name:      "cleanup-env",
			OutputDir: runDir,
		},
		Hosts: map[string]config.Host{
			"local": {},
		},
		Cases: []config.Case{
			{Name: "a", Env: map[string]string{"ENGINE": "engine-a"}},
		},
		Cleanup: []config.Cleanup{
			{
				Name:    "capture-env",
				Command: "printf \"case=%s engine=%s host=%s\\n\" \"${BENCHCTL_CASE_NAME:-}\" \"${ENGINE:-}\" \"$BENCHCTL_HOST\" > '" + outputPath + "'",
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := executeCleanup(context.Background(), cfg, "1", runDir, logger, io.Discard, map[string]string{"EXTRA": "value"}); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "engine-a") {
		t.Fatalf("cleanup should not receive case env, got: %q", got)
	}
	if strings.Contains(got, "case=a") {
		t.Fatalf("cleanup should not receive case name, got: %q", got)
	}
	if !strings.Contains(got, "host=local") {
		t.Fatalf("expected host env in cleanup output, got: %q", got)
	}
}
