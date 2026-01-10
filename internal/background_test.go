package internal

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/internal/execution"
)

func TestTerminatePIDStopsProcess(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid not available")
	}

	pidFile := filepath.Join(t.TempDir(), "pid")
	cmd := exec.Command("sh", "-c", "setsid sleep 60 & echo $! > \""+pidFile+"\"")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	client := execution.NewLocalClient()
	defer func() { _ = client.Close() }()

	if err := cmd.Wait(); err != nil {
		t.Fatalf("failed to start setsid child: %v", err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("failed to read pid file: %v", err)
	}
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		t.Fatalf("expected pid in pid file")
	}

	logger := log.New(io.Discard, "", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := terminatePID(ctx, client, "sleep-stage", pid, logger); err != nil {
		t.Fatalf("terminatePID returned error: %v", err)
	}

	existsCmd := exec.Command("sh", "-c", "kill -0 -"+pid+" 2>/dev/null || kill -0 "+pid+" 2>/dev/null")
	if err := existsCmd.Run(); err == nil {
		t.Fatalf("expected process group %s to be terminated", pid)
	}
}

func TestCollectStageOutputsMultiHostSuffix(t *testing.T) {
	remoteDir := t.TempDir()
	runDir := t.TempDir()
	remotePath := filepath.Join(remoteDir, "metrics.csv")
	if err := os.WriteFile(remotePath, []byte("sample"), 0644); err != nil {
		t.Fatalf("failed to write remote file: %v", err)
	}

	stage := config.Stage{
		Name:  "collect",
		Hosts: []string{"host-a", "host-b"},
		Outputs: []config.Output{
			{
				Name:       "metrics",
				RemotePath: remotePath,
			},
		},
	}

	client := execution.NewLocalClient()
	defer func() { _ = client.Close() }()

	logger := log.New(io.Discard, "", 0)
	ctx := context.Background()
	if err := collectStageOutputs(ctx, client, runDir, stage, logger, "host-a"); err != nil {
		t.Fatalf("collectStageOutputs failed: %v", err)
	}

	expected := filepath.Join(runDir, "metrics__host-a.csv")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
}

func TestTerminatePIDStopsProcessGroup(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid not available")
	}

	pidFile := filepath.Join(t.TempDir(), "pid")
	cmd := exec.Command("sh", "-c", "setsid sh -c 'sleep 60 & wait' & echo $! > \""+pidFile+"\"")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start process group: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("failed to start setsid child: %v", err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("failed to read pid file: %v", err)
	}
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		t.Fatalf("expected pid in pid file")
	}

	client := execution.NewLocalClient()
	defer func() { _ = client.Close() }()

	logger := log.New(io.Discard, "", 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := terminatePID(ctx, client, "bg-stage", pid, logger); err != nil {
		t.Fatalf("terminatePID returned error: %v", err)
	}

	existsCmd := exec.Command("sh", "-c", "kill -0 -"+pid+" 2>/dev/null || kill -0 "+pid+" 2>/dev/null")
	if err := existsCmd.Run(); err == nil {
		t.Fatalf("expected process group %s to be terminated", pid)
	}
}
