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
)

const (
	workflowTestDir = "./testdata/workflow"
	testOutputDir   = "/tmp/benchctl-test-output"
	testTimeout     = 30 * time.Second
)

var (
	customMetadata = map[string]string{
		"test_metadata": "test_value",
	}
)

func setupTest(t *testing.T) {
	t.Helper()

	// Clean up any previous test outputs
	os.RemoveAll(testOutputDir)
	os.RemoveAll("/tmp/benchctl-test-collected.txt")
}

func teardownTest(t *testing.T) {
	t.Helper()
	// Clean up test outputs
	os.RemoveAll(testOutputDir)
	os.RemoveAll("/tmp/benchctl-test-collected.txt")
}

func loadWorkflowConfig(t *testing.T, filename string) *internal.Config {
	t.Helper()
	path := filepath.Join(workflowTestDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read workflow config %s: %v", filename, err)
	}

	cfg, err := internal.ParseYAML(data)
	if err != nil {
		t.Fatalf("failed to parse workflow config %s: %v", filename, err)
	}

	return cfg
}

func TestWorkflowSimple(t *testing.T) {
	setupTest(t)
	defer teardownTest(t)

	cfg := loadWorkflowConfig(t, "simple.yaml")

	// Run workflow - this will use log.Fatal on error, so we can't capture it easily
	// For now, we just ensure it doesn't panic
	internal.RunWorkflow(context.Background(), cfg, customMetadata)

	// Should not panic
}

func TestWorkflowMultiStage(t *testing.T) {
	setupTest(t)
	defer teardownTest(t)

	cfg := loadWorkflowConfig(t, "multi_stage.yaml")
	internal.RunWorkflow(context.Background(), cfg, customMetadata)

	// Should not panic
}

func TestWorkflowWithHealthCheck(t *testing.T) {
	setupTest(t)
	defer teardownTest(t)

	cfg := loadWorkflowConfig(t, "with_healthcheck.yaml")

	// Create context with timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Start workflow in goroutine since it might take time
	done := make(chan bool)
	go func() {
		internal.RunWorkflow(context.Background(), cfg, customMetadata)
		done <- true
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		// Workflow completed
	case <-ctx.Done():
		t.Fatal("workflow timed out")
	}

	// Clean up: kill the nc process
	host := internal.Host{
		IP:       "localhost",
		Port:     2222,
		Username: "testuser",
		KeyFile:  "./testdata/ssh/test_key",
	}
	client, err := internal.NewSSHClient(host)
	if err != nil {
		t.Logf("warning: failed to create cleanup client: %v", err)
	} else {
		defer client.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.RunCommand(cleanupCtx, "pkill -f 'nc -l -p 8080' || true")
	}
}

func TestWorkflowWithOutput(t *testing.T) {
	setupTest(t)
	defer teardownTest(t)

	cfg := loadWorkflowConfig(t, "with_output.yaml")
	internal.RunWorkflow(context.Background(), cfg, customMetadata)

	// Verify file was actually copied
	if _, err := os.Stat("/tmp/benchctl-test-collected.txt"); os.IsNotExist(err) {
		t.Error("expected collected file to exist")
	} else {
		// Verify file contents
		data, err := os.ReadFile("/tmp/benchctl-test-collected.txt")
		if err != nil {
			t.Errorf("failed to read collected file: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "test output data") {
			t.Errorf("expected file to contain 'test output data', got: %s", content)
		}
	}
}
func TestWorkflowCommandWithSpecialCharacters(t *testing.T) {
	setupTest(t)
	defer teardownTest(t)
	cfg := loadWorkflowConfig(t, "special_chars.yaml")
	internal.RunWorkflow(context.Background(), cfg, customMetadata)

	// should not panic
}

func TestWorkflowRemoteScriptExecution(t *testing.T) {
	setupTest(t)
	defer teardownTest(t)

	cfg := loadWorkflowConfig(t, "remote_script.yaml")
	internal.RunWorkflow(context.Background(), cfg, customMetadata)

	// Verify file created by the remote script was copied locally
	localCollected := "/tmp/benchctl-test-remote-script.txt"
	if _, err := os.Stat(localCollected); os.IsNotExist(err) {
		t.Fatalf("expected remote-script collected file to exist: %s", localCollected)
	}
	data, err := os.ReadFile(localCollected)
	if err != nil {
		t.Fatalf("failed reading collected remote-script file: %v", err)
	}
	if !strings.Contains(string(data), "remote-script-ok") {
		t.Errorf("expected collected file to contain marker; got: %s", string(data))
	}
}

func TestWorkflowAppendMetadata(t *testing.T) {
	setupTest(t)
	defer teardownTest(t)

	cfg := loadWorkflowConfig(t, "append_metadata.yaml")
	internal.RunWorkflow(context.Background(), cfg, customMetadata)

	// Inspect metadata.json under the first run directory
	mdPath := filepath.Join(testOutputDir, "1", "metadata.json")
	b, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed to read metadata.json: %v", err)
	}
	var md internal.RunMetadata
	if err := json.Unmarshal(b, &md); err != nil {
		t.Fatalf("failed to unmarshal metadata.json: %v", err)
	}
	got := md.Custom
	// Expect keys emitted by the append_metadata stage (stringified values)
	if got["am_key"] != "am_value" {
		t.Errorf("expected custom.am_key=am_value, got %q", got["am_key"])
	}
	if got["am_num"] != "42" {
		t.Errorf("expected custom.am_num=\"42\", got %q", got["am_num"])
	}
}
