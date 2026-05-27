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

func TestExecuteStagesFailsFastAcrossHosts(t *testing.T) {
	tempDir := t.TempDir()
	runDir := filepath.Join(tempDir, "run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("failed to create run dir: %v", err)
	}

	counterPath := filepath.Join(tempDir, "counter.txt")
	command := "count=0; if [ -f '" + counterPath + "' ]; then count=$(cat '" + counterPath + "'); fi; count=$((count+1)); echo $count > '" + counterPath + "'; if [ $count -eq 2 ]; then exit 1; fi"

	cfg := &config.Config{
		Benchmark: config.Benchmark{
			Name:      "multi-host",
			OutputDir: runDir,
		},
		Hosts: map[string]config.Host{
			"local": {},
		},
		Stages: []config.Stage{
			{
				Name:    "fail-fast",
				Hosts:   []string{"local", "local", "local"},
				Command: command,
			},
		},
	}

	metadata := &RunMetadata{
		RunID:         "1",
		BenchmarkName: "multi-host",
		Hosts:         cfg.Hosts,
		Custom:        map[string]string{},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	backgroundMgr := newBackgroundManager(logger)
	ctx := context.Background()

	err := executeStages(ctx, cfg, "1", runDir, logger, io.Discard, metadata, backgroundMgr, nil)
	if err == nil {
		t.Fatalf("expected executeStages to fail on second host")
	}

	data, readErr := os.ReadFile(counterPath)
	if readErr != nil {
		t.Fatalf("failed reading counter: %v", readErr)
	}
	if string(data) != "2\n" && string(data) != "2" {
		t.Fatalf("expected only two executions before failure, got %q", string(data))
	}
}

func TestExecuteStagesSkipsMarkedStage(t *testing.T) {
	tempDir := t.TempDir()
	runDir := filepath.Join(tempDir, "run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("failed to create run dir: %v", err)
	}

	counterPath := filepath.Join(tempDir, "counter.txt")
	command := "count=0; if [ -f '" + counterPath + "' ]; then count=$(cat '" + counterPath + "'); fi; count=$((count+1)); echo $count > '" + counterPath + "'"

	cfg := &config.Config{
		Benchmark: config.Benchmark{
			Name:      "skip-stage",
			OutputDir: runDir,
		},
		Hosts: map[string]config.Host{
			"local": {},
		},
		Stages: []config.Stage{
			{
				Name:    "first",
				Command: command,
			},
			{
				Name:    "second",
				Command: command,
				Skip:    true,
			},
		},
	}

	metadata := &RunMetadata{
		RunID:         "1",
		BenchmarkName: "skip-stage",
		Hosts:         cfg.Hosts,
		Custom:        map[string]string{},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	backgroundMgr := newBackgroundManager(logger)
	ctx := context.Background()

	err := executeStages(ctx, cfg, "1", runDir, logger, io.Discard, metadata, backgroundMgr, nil)
	if err != nil {
		t.Fatalf("unexpected error executing stages: %v", err)
	}

	data, readErr := os.ReadFile(counterPath)
	if readErr != nil {
		t.Fatalf("failed reading counter: %v", readErr)
	}
	if string(data) != "1\n" && string(data) != "1" {
		t.Fatalf("expected only one execution, got %q", string(data))
	}
}

func TestExecuteStagesRunsCasesWithEnv(t *testing.T) {
	tempDir := t.TempDir()
	runDir := filepath.Join(tempDir, "run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("failed to create run dir: %v", err)
	}

	outputPath := filepath.Join(tempDir, "cases.txt")
	cfg := &config.Config{
		Benchmark: config.Benchmark{Name: "cases", OutputDir: runDir},
		Hosts:     map[string]config.Host{"local": {}},
		Cases: []config.Case{
			{Name: "a", Env: map[string]string{"ENGINE": "engine-a"}},
			{Name: "b", Env: map[string]string{"ENGINE": "engine-b"}},
		},
		Stages: []config.Stage{
			{Name: "all", Command: "printf \"%s:%s\\n\" \"$BENCHCTL_CASE_NAME\" \"$ENGINE\" >> '" + outputPath + "'"},
			{Name: "only-b", ExecuteOnlyFor: "b", Command: "printf \"only:%s\\n\" \"$BENCHCTL_CASE_NAME\" >> '" + outputPath + "'"},
		},
	}

	metadata := &RunMetadata{RunID: "1", BenchmarkName: "cases", Hosts: cfg.Hosts, Custom: map[string]string{}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	backgroundMgr := newBackgroundManager(logger)

	if err := executeStages(context.Background(), cfg, "1", runDir, logger, io.Discard, metadata, backgroundMgr, nil); err != nil {
		t.Fatalf("unexpected error executing stages: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(data)
	for _, want := range []string{"a:engine-a", "b:engine-b", "only:b"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
	if strings.Contains(got, "only:a") {
		t.Fatalf("execute_only_for stage ran for wrong case: %q", got)
	}
}
