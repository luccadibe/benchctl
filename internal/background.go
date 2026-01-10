package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
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
	hostAlias string
	host      config.Host
	pid       string
}

// backgroundManager coordinates background stages
type backgroundManager struct {
	logger *log.Logger
	stages []backgroundStage
}

func newBackgroundManager(logger *log.Logger) *backgroundManager {
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
	m.logger.Printf("Stopping background stage %s (PID %s)", record.stage.Name, record.pid)

	client, err := openExecutionClient(record.host)
	if err != nil {
		return fmt.Errorf("background stage %s: %w", record.stage.Name, err)
	}
	defer client.Close()

	if err := terminatePID(ctx, client, record.stage.Name, record.pid, m.logger); err != nil {
		return err
	}

	if len(record.stage.Outputs) > 0 {
		if err := collectStageOutputs(ctx, client, runDir, record.stage, m.logger, record.hostAlias); err != nil {
			m.logger.Printf("WARNING: background stage %s outputs failed to collect: %v", record.stage.Name, err)
		}
	}

	return nil
}

// terminatePID sends a SIGTERM to the process and waits for it to exit.
func terminatePID(ctx context.Context, client execution.ExecutionClient, stageName, pid string, logger *log.Logger) error {
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
			logger.Printf("WARNING: background stage %s still running: %v", stageName, err)
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

func outputFilename(output config.Output, stage config.Stage, hostAlias string) string {
	ext := filepath.Ext(output.RemotePath)
	filename := output.Name + ext
	if len(stage.Hosts) > 0 {
		filename = output.Name + "__" + hostAlias + ext
	}
	return filename
}

func collectStageOutputs(ctx context.Context, client execution.ExecutionClient, runDir string, stage config.Stage, logger *log.Logger, hostAlias string) error {
	for _, output := range stage.Outputs {
		remotePath := output.RemotePath
		filename := outputFilename(output, stage, hostAlias)
		localPath := filepath.Join(runDir, filename)
		if err := client.Scp(ctx, remotePath, localPath); err != nil {
			return fmt.Errorf("failed to collect output %s for stage %s: %w", output.Name, stage.Name, err)
		}
		logger.Printf("Collected output %s: %s -> %s", output.Name, remotePath, localPath)
		if output.DataSchema != nil {
			for _, col := range output.DataSchema.Columns {
				if col.Type == config.DataTypeTimestamp && strings.TrimSpace(col.Format) == "" {
					logger.Printf("WARNING: data_schema.%s has type=timestamp without format; falling back to auto-detection which may be slower/ambiguous", col.Name)
				}
			}
		}
	}
	return nil
}
