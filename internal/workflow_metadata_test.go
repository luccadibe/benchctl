//go:build unit

package internal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luccadibe/benchctl/internal/config"
)

func TestRunWorkflowPersistsExpandedMetadata(t *testing.T) {
	outputDir := t.TempDir()
	cfg := &config.Config{
		Benchmark: config.Benchmark{
			Name:      "metadata-expand",
			OutputDir: outputDir,
		},
		Hosts: map[string]config.Host{"local": {}},
		Cases: []config.Case{{
			Name: "openfaas",
			Env:  map[string]string{"BENCH_PLATFORM": "openfaas"},
		}},
		Stages: []config.Stage{{
			Name:    "run",
			Command: "echo ${BENCH_PLATFORM} > /tmp/${BENCH_PLATFORM}-out.txt",
			Outputs: []config.Output{{
				Name:       "${BENCH_PLATFORM}-out",
				RemotePath: "/tmp/${BENCH_PLATFORM}-out.txt",
			}},
		}},
	}

	result, err := RunWorkflow(context.Background(), cfg, map[string]string{
		"platform": "${BENCH_PLATFORM}",
	}, nil)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(result.RunDir, "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata.json: %v", err)
	}

	var saved RunMetadata
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	output := saved.Config.Stages[0].Outputs[0]
	if output.Name != "openfaas-out" {
		t.Fatalf("persisted output name = %q", output.Name)
	}
	if !strings.Contains(saved.Config.Stages[0].Command, "openfaas") {
		t.Fatalf("persisted command = %q", saved.Config.Stages[0].Command)
	}
	if saved.Custom["platform"] != "openfaas" {
		t.Fatalf("persisted custom platform = %q", saved.Custom["platform"])
	}
}

func TestRunWorkflowFailureSavesMetadata(t *testing.T) {
	outputDir := t.TempDir()
	cfg := &config.Config{
		Benchmark: config.Benchmark{
			Name:      "failure-metadata",
			OutputDir: outputDir,
		},
		Hosts: map[string]config.Host{"local": {}},
		Cases: []config.Case{{}},
		Stages: []config.Stage{{
			Name:    "failing-stage",
			Command: "false",
		}},
	}

	result, err := RunWorkflow(context.Background(), cfg, map[string]string{
		"platform": "test-failure",
	}, nil)
	if err == nil {
		t.Fatalf("expected error from workflow execution, got nil")
	}

	if result == nil {
		t.Fatalf("expected result to be non-nil even on execution error")
	}

	if result.Metadata == nil {
		t.Fatalf("expected result.Metadata to be non-nil")
	}

	if result.Metadata.Status != "failed" {
		t.Fatalf("expected status to be 'failed', got %q", result.Metadata.Status)
	}

	if !strings.Contains(result.Metadata.Error, "failing-stage failed") {
		t.Fatalf("expected error message to contain 'failing-stage failed', got %q", result.Metadata.Error)
	}

	// Verify metadata.json was saved on disk
	data, err := os.ReadFile(filepath.Join(result.RunDir, "metadata.json"))
	if err != nil {
		t.Fatalf("failed to read metadata.json from disk: %v", err)
	}

	var saved RunMetadata
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("failed to unmarshal saved metadata.json: %v", err)
	}

	if saved.Status != "failed" {
		t.Errorf("expected saved metadata status to be 'failed', got %q", saved.Status)
	}

	if saved.Custom["platform"] != "test-failure" {
		t.Errorf("expected saved custom platform metadata to be 'test-failure', got %q", saved.Custom["platform"])
	}
}
