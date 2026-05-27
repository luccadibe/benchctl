//go:build integration

package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luccadibe/benchctl/internal"
	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/internal/execution"
)

const (
	workflowTestDir = "./testdata/workflow"
	testOutputDir   = "/tmp/benchctl-test-output"
	testTimeout     = 30 * time.Second
)

var customMetadata = map[string]string{
	"test_metadata": "test_value",
}

func setupWorkflowTest(t *testing.T) {
	t.Helper()
	os.RemoveAll(testOutputDir)
	os.RemoveAll("/tmp/benchctl-test-collected.txt")
	t.Cleanup(func() {
		os.RemoveAll(testOutputDir)
		os.RemoveAll("/tmp/benchctl-test-collected.txt")
	})
}

func loadWorkflowConfig(t *testing.T, filename string) *config.Config {
	t.Helper()
	path := filepath.Join(workflowTestDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read workflow config %s: %v", filename, err)
	}

	cfg, err := config.ParseYAML(data)
	if err != nil {
		t.Fatalf("failed to parse workflow config %s: %v", filename, err)
	}

	return cfg
}

func runWorkflow(t *testing.T, cfg *config.Config) *internal.RunResult {
	t.Helper()
	setupWorkflowTest(t)

	result, err := internal.RunWorkflow(context.Background(), cfg, customMetadata, nil)
	if err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	return result
}

func assertRunMetadata(t *testing.T, result *internal.RunResult, benchmarkName string) {
	t.Helper()

	metadataPath := filepath.Join(result.RunDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata.json: %v", err)
	}

	var metadata internal.RunMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata.BenchmarkName != benchmarkName {
		t.Fatalf("benchmark_name = %q, want %q", metadata.BenchmarkName, benchmarkName)
	}
	if metadata.Custom["test_metadata"] != "test_value" {
		t.Fatalf("expected custom metadata to be persisted")
	}
	if result.RunID == "" {
		t.Fatal("expected non-empty run id")
	}
}

func TestWorkflowMultiStage(t *testing.T) {
	cfg := loadWorkflowConfig(t, "multi_stage.yaml")
	result := runWorkflow(t, cfg)
	assertRunMetadata(t, result, "multi-stage-test")
}

func TestWorkflowWithHealthCheck(t *testing.T) {
	setupWorkflowTest(t)

	cfg := loadWorkflowConfig(t, "with_healthcheck.yaml")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	if _, err := internal.RunWorkflow(ctx, cfg, customMetadata, nil); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	host := config.Host{
		IP:       "localhost",
		Port:     2222,
		Username: "testuser",
		KeyFile:  "./testdata/ssh/test_key",
	}
	client, err := execution.NewSSHClient(host)
	if err != nil {
		t.Logf("warning: failed to create cleanup client: %v", err)
		return
	}
	defer client.Close()

	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = client.RunCommand(cleanupCtx, execution.CommandRequest{Command: "pkill -f 'nc -l -p 8080' || true"})
}

func TestWorkflowWithOutput(t *testing.T) {
	cfg := loadWorkflowConfig(t, "with_output.yaml")
	result := runWorkflow(t, cfg)
	assertRunMetadata(t, result, cfg.Benchmark.Name)

	collectedFile := filepath.Join(result.RunDir, "test_output.txt")
	data, err := os.ReadFile(collectedFile)
	if err != nil {
		t.Fatalf("read collected file: %v", err)
	}
	if !strings.Contains(string(data), "test output data") {
		t.Fatalf("expected collected file to contain marker, got: %s", string(data))
	}
}

func TestWorkflowCommandWithSpecialCharacters(t *testing.T) {
	cfg := loadWorkflowConfig(t, "special_chars.yaml")
	result := runWorkflow(t, cfg)
	assertRunMetadata(t, result, "special-chars-test")
}

func TestWorkflowRemoteScriptExecution(t *testing.T) {
	cfg := loadWorkflowConfig(t, "remote_script.yaml")
	result := runWorkflow(t, cfg)
	assertRunMetadata(t, result, cfg.Benchmark.Name)

	localCollected := filepath.Join(result.RunDir, "remote_created.txt")
	data, err := os.ReadFile(localCollected)
	if err != nil {
		t.Fatalf("read collected remote-script file: %v", err)
	}
	if !strings.Contains(string(data), "remote-script-ok") {
		t.Fatalf("expected collected file to contain marker; got: %s", string(data))
	}
}

func TestWorkflowBackgroundStage(t *testing.T) {
	cfg := loadWorkflowConfig(t, "background.yaml")
	result := runWorkflow(t, cfg)
	assertRunMetadata(t, result, cfg.Benchmark.Name)

	monitorPath := filepath.Join(result.RunDir, "monitor.csv")
	data, err := os.ReadFile(monitorPath)
	if err != nil {
		t.Fatalf("failed to read background monitor output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected monitor.csv to have header plus data rows, got %d lines", len(lines))
	}
	if lines[0] != "timestamp,counter" {
		t.Fatalf("unexpected header in monitor.csv: %s", lines[0])
	}
	if lines[1] == lines[len(lines)-1] {
		t.Errorf("expected counter values to change across samples, got %q", lines[1])
	}

	workloadPath := filepath.Join(result.RunDir, "workload.txt")
	if _, err := os.Stat(workloadPath); err != nil {
		t.Fatalf("expected workload output to exist: %v", err)
	}
}

func TestWorkflowCommandFailure(t *testing.T) {
	setupWorkflowTest(t)

	cfg, err := config.ParseYAML([]byte(`
benchmark:
  name: command-failure-test
  output_dir: /tmp/benchctl-test-output
hosts:
  test-host:
    ip: localhost
    port: 2222
    username: testuser
    key_file: ./testdata/ssh/test_key
stages:
  - name: failing-command
    host: test-host
    command: exit 1
`))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	_, err = internal.RunWorkflow(context.Background(), cfg, customMetadata, nil)
	if err == nil {
		t.Fatal("expected workflow to fail on non-zero exit")
	}
}
