package internal

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

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

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
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
