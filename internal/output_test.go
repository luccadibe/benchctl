package internal

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/internal/execution"
)

func TestExpandTemplate(t *testing.T) {
	env := map[string]string{
		"BENCH_PLATFORM": "openfaas",
		"BENCHCTL_HOST":  "eval-vm",
	}

	got, err := expandTemplate("${BENCH_PLATFORM}-sustained", env)
	if err != nil {
		t.Fatalf("expandTemplate: %v", err)
	}
	if got != "openfaas-sustained" {
		t.Fatalf("got %q, want openfaas-sustained", got)
	}

	if _, err := expandTemplate("${UNKNOWN}/x", env); err == nil {
		t.Fatal("expected error for unknown variable")
	}
}

func TestExpandTemplateLiteralDollar(t *testing.T) {
	env := map[string]string{"X": "y"}
	got, err := expandTemplate("cost is $$100", env)
	if err != nil {
		t.Fatalf("expandTemplate: %v", err)
	}
	if got != "cost is $100" {
		t.Fatalf("got %q, want cost is $100", got)
	}
}

func TestResolveOutputUsesExpandedNameForLocalFile(t *testing.T) {
	cfg := &config.Config{Benchmark: config.Benchmark{OutputDir: "./results"}}
	env := buildStageEnv(
		"3",
		"/tmp/results/3",
		cfg,
		nil,
		config.Case{Name: "openfaas", Env: map[string]string{"BENCH_PLATFORM": "openfaas"}},
		"",
	)

	resolved, err := resolveOutput(config.Output{
		Name:       "${BENCH_PLATFORM}-sustained",
		RemotePath: "/tmp/results/${BENCH_PLATFORM}-sustained.csv",
	}, env)
	if err != nil {
		t.Fatalf("resolveOutput: %v", err)
	}
	if resolved.remotePath != "/tmp/results/openfaas-sustained.csv" {
		t.Fatalf("remote_path = %q", resolved.remotePath)
	}
	if resolved.localFilename != "openfaas-sustained.csv" {
		t.Fatalf("localFilename = %q, want openfaas-sustained.csv", resolved.localFilename)
	}
}

func TestCollectStageOutputsNoHostSuffix(t *testing.T) {
	remoteDir := t.TempDir()
	runDir := t.TempDir()
	remotePath := filepath.Join(remoteDir, "metrics.csv")
	if err := os.WriteFile(remotePath, []byte("sample"), 0644); err != nil {
		t.Fatalf("write remote file: %v", err)
	}

	stage := config.Stage{
		Name:  "collect",
		Hosts: []string{"host-a"},
		Outputs: []config.Output{
			{
				Name:       "${BENCHCTL_HOST}-metrics",
				RemotePath: remotePath,
			},
		},
	}

	env := map[string]string{EnvHost: "host-a"}
	client := execution.NewLocalClient()
	defer func() { _ = client.Close() }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := collectStageOutputs(context.Background(), client, runDir, stage, logger, env); err != nil {
		t.Fatalf("collectStageOutputs: %v", err)
	}

	expected := filepath.Join(runDir, "host-a-metrics.csv")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected %s: %v", expected, err)
	}
}

func TestCollectStageOutputsCaseEnv(t *testing.T) {
	remoteDir := t.TempDir()
	runDir := t.TempDir()
	remotePath := filepath.Join(remoteDir, "openfaas-sustained.csv")
	if err := os.WriteFile(remotePath, []byte("csv"), 0644); err != nil {
		t.Fatalf("write remote file: %v", err)
	}

	cfg := &config.Config{Benchmark: config.Benchmark{Name: "bench", OutputDir: runDir}}
	env := buildStageEnv("1", runDir, cfg, nil, config.Case{
		Name: "openfaas",
		Env:  map[string]string{"BENCH_PLATFORM": "openfaas"},
	}, "eval-vm")

	stage := config.Stage{
		Name: "run",
		Outputs: []config.Output{
			{
				Name:       "${BENCH_PLATFORM}-sustained",
				RemotePath: filepath.Join(remoteDir, "${BENCH_PLATFORM}-sustained.csv"),
			},
		},
	}

	client := execution.NewLocalClient()
	defer func() { _ = client.Close() }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := collectStageOutputs(context.Background(), client, runDir, stage, logger, env); err != nil {
		t.Fatalf("collectStageOutputs: %v", err)
	}

	expected := filepath.Join(runDir, "openfaas-sustained.csv")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected %s: %v", expected, err)
	}
}
