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
	"github.com/luccadibe/benchctl/internal/execution"
)

func TestExpandTemplate(t *testing.T) {
	env := map[string]string{
		"BENCH_PLATFORM": "openfaas",
		"BENCHCTL_HOST":  "eval-vm",
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "braced variable", input: "${BENCH_PLATFORM}-sustained", want: "openfaas-sustained"},
		{name: "plain variable", input: "$BENCHCTL_HOST-metrics", want: "eval-vm-metrics"},
		{name: "literal dollar", input: "cost is $$100", want: "cost is $100"},
		{name: "no variables", input: "static-name", want: "static-name"},
		{name: "unknown variable", input: "${UNKNOWN}/x", wantErr: "undefined variable UNKNOWN"},
		{
			name:    "multiple unknown variables deduped",
			input:   "${MISSING}/x/${OTHER}/${MISSING}",
			wantErr: "undefined variable MISSING, OTHER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandTemplate(tt.input, env)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("expandTemplate: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveOutput(t *testing.T) {
	cfg := &config.Config{Benchmark: config.Benchmark{OutputDir: "./results"}}
	env := buildStageEnv(
		"3",
		"/tmp/results/3",
		cfg,
		nil,
		config.Case{Name: "openfaas", Env: map[string]string{"BENCH_PLATFORM": "openfaas"}},
		"",
	)

	t.Run("expanded name and path", func(t *testing.T) {
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
	})

	t.Run("empty name after expansion", func(t *testing.T) {
		emptyEnv := map[string]string{"EMPTY": ""}
		_, err := resolveOutput(config.Output{
			Name:       "${EMPTY}",
			RemotePath: "/tmp/out.csv",
		}, emptyEnv)
		if err == nil {
			t.Fatal("expected error for empty expanded name")
		}
		if !strings.Contains(err.Error(), "name is empty after expansion") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty remote path after expansion", func(t *testing.T) {
		emptyEnv := map[string]string{"EMPTY": ""}
		_, err := resolveOutput(config.Output{
			Name:       "metrics",
			RemotePath: "${EMPTY}",
		}, emptyEnv)
		if err == nil {
			t.Fatal("expected error for empty expanded remote path")
		}
		if !strings.Contains(err.Error(), "remote_path is empty after expansion") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCollectStageOutputs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := execution.NewLocalClient()
	t.Cleanup(func() { _ = client.Close() })

	t.Run("host env in output name", func(t *testing.T) {
		remoteDir := t.TempDir()
		runDir := t.TempDir()
		remotePath := filepath.Join(remoteDir, "metrics.csv")
		if err := os.WriteFile(remotePath, []byte("sample"), 0644); err != nil {
			t.Fatalf("write remote file: %v", err)
		}

		stage := config.Stage{
			Name:  "collect",
			Hosts: []string{"host-a"},
			Outputs: []config.Output{{
				Name:       "${BENCHCTL_HOST}-metrics",
				RemotePath: remotePath,
			}},
		}
		env := map[string]string{EnvHost: "host-a"}

		if err := collectStageOutputs(context.Background(), client, runDir, stage, logger, env); err != nil {
			t.Fatalf("collectStageOutputs: %v", err)
		}
		expected := filepath.Join(runDir, "host-a-metrics.csv")
		if _, err := os.Stat(expected); err != nil {
			t.Fatalf("expected %s: %v", expected, err)
		}
	})

	t.Run("case env in output name and path", func(t *testing.T) {
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
			Outputs: []config.Output{{
				Name:       "${BENCH_PLATFORM}-sustained",
				RemotePath: filepath.Join(remoteDir, "${BENCH_PLATFORM}-sustained.csv"),
			}},
		}

		if err := collectStageOutputs(context.Background(), client, runDir, stage, logger, env); err != nil {
			t.Fatalf("collectStageOutputs: %v", err)
		}
		expected := filepath.Join(runDir, "openfaas-sustained.csv")
		if _, err := os.Stat(expected); err != nil {
			t.Fatalf("expected %s: %v", expected, err)
		}
	})
}
