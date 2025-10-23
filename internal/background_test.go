package internal

import (
	"context"
	"io"
	"log"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/luccadibe/benchctl/internal/execution"
)

func TestTerminatePIDStopsProcess(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	client := execution.NewLocalClient()
	defer func() { _ = client.Close() }()

	pid := strconv.Itoa(cmd.Process.Pid)
	logger := log.New(io.Discard, "", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := terminatePID(ctx, client, "sleep-stage", pid, logger); err != nil {
		t.Fatalf("terminatePID returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-time.After(500 * time.Millisecond):
		t.Fatal("process still running after terminatePID")
	case <-done:
		// ok, process exited
	}
}
