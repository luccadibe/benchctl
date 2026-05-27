package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/internal/execution"
)

// BackgroundTerminationGrace defines how long we wait after SIGTERM
// before forcefully killing background stages.
const BackgroundTerminationGrace = 2 * time.Second

const backgroundCheckInterval = 200 * time.Millisecond

// backgroundStage tracks a running background stage.
type backgroundStage struct {
	stage     config.Stage
	host      config.Host
	outputEnv map[string]string
	pid       string
}

// backgroundManager coordinates background stages
type backgroundManager struct {
	logger *slog.Logger
	stages []backgroundStage
}

func newBackgroundManager(logger *slog.Logger) *backgroundManager {
	return &backgroundManager{logger: logger}
}

func (m *backgroundManager) Add(record backgroundStage) {
	m.stages = append(m.stages, record)
}

func (m *backgroundManager) StopAll(ctx context.Context, runDir string) error {
	if len(m.stages) == 0 {
		return nil
	}

	var combinedErr error
	for _, record := range m.stages {
		if err := m.stopStage(ctx, runDir, record); err != nil {
			combinedErr = errors.Join(combinedErr, err)
		}
	}
	return combinedErr
}

func (m *backgroundManager) stopStage(ctx context.Context, runDir string, record backgroundStage) error {
	m.logger.Info("stopping background stage", "stage", record.stage.Name, "pid", record.pid)

	client, err := openExecutionClient(record.host)
	if err != nil {
		err = fmt.Errorf("background stage %s: %w", record.stage.Name, err)
		m.logger.Error("background stage stop failed", "stage", record.stage.Name, "error", err)
		return err
	}
	defer client.Close()

	if err := terminatePID(ctx, client, record.stage.Name, record.pid, m.logger); err != nil {
		m.logger.Error("background stage stop failed", "stage", record.stage.Name, "pid", record.pid, "error", err)
		return err
	}

	if len(record.stage.Outputs) > 0 {
		if err := collectStageOutputs(ctx, client, runDir, record.stage, m.logger, record.outputEnv); err != nil {
			m.logger.Warn("background stage outputs failed to collect", "stage", record.stage.Name, "error", err)
		}
	}

	return nil
}

// terminatePID sends a SIGTERM to the process and waits for it to exit.
func terminatePID(ctx context.Context, client execution.ExecutionClient, stageName, pid string, logger *slog.Logger) error {
	termCmd := fmt.Sprintf("kill -TERM -%s >/dev/null 2>&1 || true", pid)
	_, _ = client.RunCommand(ctx, execution.CommandRequest{Command: termCmd, DisableCapture: true})

	select {
	case <-time.After(BackgroundTerminationGrace):
	case <-ctx.Done():
		return ctx.Err()
	}

	alive, err := processAlive(ctx, client, pid)
	if err != nil {
		return fmt.Errorf("background stage %s: %w", stageName, err)
	}

	if alive {
		killCmd := fmt.Sprintf("kill -KILL -%s >/dev/null 2>&1 || true", pid)
		_, _ = client.RunCommand(ctx, execution.CommandRequest{Command: killCmd, DisableCapture: true})
		if err := waitForExit(ctx, client, pid); err != nil {
			logger.Warn("background stage still running", "stage", stageName, "error", err)
		}
	}

	return nil
}

// processAlive checks if a process is still running by sending a SIG0.
func processAlive(ctx context.Context, client execution.ExecutionClient, pid string) (bool, error) {
	res, err := client.RunCommand(ctx, execution.CommandRequest{
		Command:        fmt.Sprintf("kill -0 -%s >/dev/null 2>&1", pid),
		DisableCapture: true,
	})
	if err != nil && res.ExitCode == -1 {
		return false, err
	}
	if res.ExitCode == 0 {
		return true, nil
	}
	res, err = client.RunCommand(ctx, execution.CommandRequest{
		Command:        fmt.Sprintf("kill -0 %s >/dev/null 2>&1", pid),
		DisableCapture: true,
	})
	if err != nil && res.ExitCode == -1 {
		return false, err
	}
	return res.ExitCode == 0, nil
}

// waitForExit waits for a process to exit.
func waitForExit(ctx context.Context, client execution.ExecutionClient, pid string) error {
	deadline := time.Now().Add(BackgroundTerminationGrace)
	for time.Now().Before(deadline) {
		alive, err := processAlive(ctx, client, pid)
		if err != nil {
			return err
		}
		if !alive {
			return nil
		}
		select {
		case <-time.After(backgroundCheckInterval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("process %s still running after SIGKILL", pid)
}

func openExecutionClient(host config.Host) (execution.ExecutionClient, error) {
	if strings.TrimSpace(host.IP) == "" {
		return execution.NewLocalClient(), nil
	}
	return execution.NewSSHClient(host)
}

