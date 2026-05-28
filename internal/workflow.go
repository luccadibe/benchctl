package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/internal/execution"
	"golang.org/x/term"
)

// RunMetadata holds metadata about a benchmark run
type RunMetadata struct {
	RunID         string                 `json:"run_id"`
	BenchmarkName string                 `json:"benchmark_name"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Config        *config.Config         `json:"config"`
	Hosts         map[string]config.Host `json:"hosts"`
	Cases         []config.Case          `json:"cases,omitempty"`
	Custom        map[string]string      `json:"custom,omitempty"`
	Git           *GitMetadata           `json:"git,omitempty"`
}

// RunResult describes a completed workflow invocation.
type RunResult struct {
	RunID    string
	RunDir   string
	Metadata *RunMetadata
}

// generateRunID creates a unique run ID as a simple increasing counter
func generateRunID(outputDir string) (string, error) {
	runNum := 1
	for {
		runID := fmt.Sprintf("%d", runNum)
		runDir := filepath.Join(outputDir, runID)
		if _, err := os.Stat(runDir); os.IsNotExist(err) {
			return runID, nil
		}
		runNum++
	}
}

// RunWorkflow executes a benchmark workflow with run ID tracking.
func RunWorkflow(ctx context.Context, cfg *config.Config, customMetadata map[string]string, envVars map[string]string) (*RunResult, error) {
	// Generate run ID and create run directory
	runID, err := generateRunID(cfg.Benchmark.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("generate run id: %w", err)
	}

	runDir := filepath.Join(cfg.Benchmark.OutputDir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("create run directory: %w", err)
	}

	metadata := &RunMetadata{
		RunID:         runID,
		BenchmarkName: cfg.Benchmark.Name,
		StartTime:     time.Now(),
		Config:        cfg,
		Hosts:         cfg.Hosts,
		Cases:         cfg.Cases,
		Custom:        customMetadata,
	}

	logger, logWriter, closeLogger, err := createLogger(cfg, runDir)
	if err != nil {
		return nil, err
	}
	defer closeLogger()
	logger.Info("run started", "run_id", runID, "run_dir", runDir)
	gitMetadata, err := CaptureGitMetadata(ctx, cfg, runDir)
	if err != nil {
		logError(logger, "git metadata capture failed", err, "run_id", runID)
		return nil, err
	}
	metadata.Git = gitMetadata
	if gitMetadata != nil {
		logger.Info("git metadata captured", "commit", gitMetadata.Commit, "branch", gitMetadata.Branch, "dirty", gitMetadata.Dirty)
	}

	backgroundMgr := newBackgroundManager(logger)

	stageErr := executeStages(ctx, cfg, runID, runDir, logger, logWriter, metadata, backgroundMgr, envVars)
	stopErr := backgroundMgr.StopAll(ctx, runDir)
	cleanupErr := executeCleanup(ctx, cfg, runID, runDir, logger, logWriter, envVars)
	joined := errors.Join(stageErr, stopErr, cleanupErr)
	if joined != nil {
		logError(logger, "workflow failed", joined, "run_id", runID)
		return nil, fmt.Errorf("workflow failed: %w", joined)
	}

	metadata.EndTime = time.Now()
	if err := saveMetadata(metadata, runDir); err != nil {
		logError(logger, "save metadata failed", err, "run_id", runID)
		return nil, err
	}

	logger.Info("workflow completed", "run_id", runID, "run_dir", runDir)
	return &RunResult{RunID: runID, RunDir: runDir, Metadata: metadata}, nil
}

// executeStages executes the stages of the benchmark
// it returns an error if any stage fails,
// nil if all stages succeed.
func executeStages(
	ctx context.Context,
	cfg *config.Config,
	runID, runDir string,
	logger *slog.Logger,
	logWriter io.Writer,
	metadata *RunMetadata,
	backgroundMgr *backgroundManager,
	envVars map[string]string,
) error {
	if len(cfg.Stages) == 0 {
		return nil
	}

	consoleSink := resolveConsoleWriter()
	stdoutSink := consoleSink
	stderrSink := consoleSink
	usePTY := consoleSink != nil
	logStageOutput := consoleSink == nil || !writersReferToSameFD(consoleSink, logWriter)

	for _, benchmarkCase := range workflowCases(cfg) {
		for i, stage := range cfg.Stages {
			if stage.Skip {
				logger.Info("stage skipped", "stage", stage.Name, "case", benchmarkCase.Name, "index", i+1, "total", len(cfg.Stages))
				continue
			}
			if !stageAppliesToCase(stage, benchmarkCase) {
				logger.Info("stage skipped for case", "stage", stage.Name, "case", benchmarkCase.Name)
				continue
			}
			logger.Info("stage started", "stage", stage.Name, "case", benchmarkCase.Name, "index", i+1, "total", len(cfg.Stages))
			hostAliases := resolveStageHosts(stage)
			for _, hostAlias := range hostAliases {
				host, ok := cfg.Hosts[hostAlias]
				if !ok {
					if hostAlias != "local" {
						err := fmt.Errorf("stage %s references unknown host %s", stage.Name, hostAlias)
						logError(logger, "stage failed", err, "stage", stage.Name, "case", benchmarkCase.Name, "host", hostAlias)
						return err
					}
					host = config.Host{}
				}

				client, err := openExecutionClient(host)
				if err != nil {
					err = fmt.Errorf("error creating execution client for stage %s: %w", stage.Name, err)
					logError(logger, "stage failed", err, "stage", stage.Name, "case", benchmarkCase.Name, "host", hostAlias)
					return err
				}

				commandBody, err := prepareStageCommand(ctx, stage, host, runID, client)
				if err != nil {
					_ = client.Close()
					logError(logger, "stage failed", err, "stage", stage.Name, "case", benchmarkCase.Name, "host", hostAlias)
					return err
				}

				commandBody = wrapWithShell(commandBody, resolveStageShell(cfg, stage))

				stageEnv := buildStageEnv(runID, runDir, cfg, envVars, benchmarkCase, hostAlias)
				envPrefix := envPrefixFromMap(stageEnv)

				if stage.Background {
					pid, err := startBackgroundStage(ctx, client, envPrefix, commandBody, stage)
					_ = client.Close()
					if err != nil {
						logError(logger, "stage failed", err, "stage", stage.Name, "case", benchmarkCase.Name, "host", hostAlias)
						return err
					}
					backgroundMgr.Add(backgroundStage{stage: stage, host: host, outputEnv: stageEnv, pid: pid})
					logger.Info("stage running in background", "stage", stage.Name)
					continue
				}

				result, err := client.RunCommand(ctx, execution.CommandRequest{
					Command: envPrefix + commandBody,
					Stdout:  stdoutSink,
					Stderr:  stderrSink,
					UsePTY:  usePTY,
				})
				if err == nil && result.ExitCode != 0 {
					err = fmt.Errorf("command exited with code %d", result.ExitCode)
				}
				if err != nil {
					if logStageOutput && strings.TrimSpace(result.Output) != "" {
						logger.Info("stage captured output", "stage", stage.Name, "output", result.Output)
					}
					_ = client.Close()
					stageErr := fmt.Errorf("stage %s failed: %w (exit code: %d)", stage.Name, err, result.ExitCode)
					logError(logger, "stage failed", stageErr, "stage", stage.Name, "case", benchmarkCase.Name, "host", hostAlias, "exit_code", result.ExitCode)
					return stageErr
				}

				if logStageOutput {
					logger.Info("stage output", "stage", stage.Name, "output", result.Output)
				}
				logger.Info("stage completed", "stage", stage.Name, "exit_code", result.ExitCode)

				if stage.HealthCheck != nil {
					if err := runHealthCheck(ctx, client, stage, logger); err != nil {
						_ = client.Close()
						logError(logger, "health check failed", err, "stage", stage.Name, "case", benchmarkCase.Name, "host", hostAlias)
						return err
					}
				}

				if len(stage.Outputs) > 0 {
					if err := collectStageOutputs(ctx, client, runDir, stage, logger, stageEnv); err != nil {
						_ = client.Close()
						logError(logger, "stage failed", err, "stage", stage.Name, "case", benchmarkCase.Name, "host", hostAlias)
						return err
					}
				}

				_ = client.Close()
			}
		}
	}

	return nil
}

// executeCleanup runs workflow cleanup steps after all stages finish, even on stage failure.
func executeCleanup(
	ctx context.Context,
	cfg *config.Config,
	runID, runDir string,
	logger *slog.Logger,
	logWriter io.Writer,
	envVars map[string]string,
) error {
	if len(cfg.Cleanup) == 0 {
		return nil
	}

	consoleSink := resolveConsoleWriter()
	stdoutSink := consoleSink
	stderrSink := consoleSink
	usePTY := consoleSink != nil
	logStepOutput := consoleSink == nil || !writersReferToSameFD(consoleSink, logWriter)

	for i, step := range cfg.Cleanup {
		logger.Info("cleanup started", "cleanup", step.Name, "index", i+1, "total", len(cfg.Cleanup))
		hostAliases := resolveCommandHosts(step.Host, step.Hosts)
		for _, hostAlias := range hostAliases {
			host, ok := cfg.Hosts[hostAlias]
			if !ok {
				if hostAlias != "local" {
					err := fmt.Errorf("cleanup %s references unknown host %s", step.Name, hostAlias)
					logError(logger, "cleanup failed", err, "cleanup", step.Name, "host", hostAlias)
					return err
				}
				host = config.Host{}
			}

			client, err := openExecutionClient(host)
			if err != nil {
				err = fmt.Errorf("error creating execution client for cleanup %s: %w", step.Name, err)
				logError(logger, "cleanup failed", err, "cleanup", step.Name, "host", hostAlias)
				return err
			}

			commandBody, err := prepareNamedCommand(ctx, step.Name, step.Command, step.Script, host, runID, client, "cleanup")
			if err != nil {
				_ = client.Close()
				logError(logger, "cleanup failed", err, "cleanup", step.Name, "host", hostAlias)
				return err
			}

			commandBody = wrapWithShell(commandBody, resolveCleanupShell(cfg, step))
			stepEnv := buildStageEnv(runID, runDir, cfg, envVars, config.Case{}, hostAlias)
			envPrefix := envPrefixFromMap(stepEnv)

			result, err := client.RunCommand(ctx, execution.CommandRequest{
				Command: envPrefix + commandBody,
				Stdout:  stdoutSink,
				Stderr:  stderrSink,
				UsePTY:  usePTY,
			})
			if err == nil && result.ExitCode != 0 {
				err = fmt.Errorf("command exited with code %d", result.ExitCode)
			}
			if err != nil {
				if logStepOutput && strings.TrimSpace(result.Output) != "" {
					logger.Info("cleanup captured output", "cleanup", step.Name, "output", result.Output)
				}
				_ = client.Close()
				cleanupErr := fmt.Errorf("cleanup %s failed: %w (exit code: %d)", step.Name, err, result.ExitCode)
				logError(logger, "cleanup failed", cleanupErr, "cleanup", step.Name, "host", hostAlias, "exit_code", result.ExitCode)
				return cleanupErr
			}

			if logStepOutput {
				logger.Info("cleanup output", "cleanup", step.Name, "output", result.Output)
			}
			logger.Info("cleanup completed", "cleanup", step.Name, "exit_code", result.ExitCode)
			_ = client.Close()
		}
	}

	return nil
}

func workflowCases(cfg *config.Config) []config.Case {
	if len(cfg.Cases) == 0 {
		return []config.Case{{}}
	}
	return cfg.Cases
}

func stageAppliesToCase(stage config.Stage, benchmarkCase config.Case) bool {
	return strings.TrimSpace(stage.ExecuteOnlyFor) == "" || stage.ExecuteOnlyFor == benchmarkCase.Name
}

func resolveStageHosts(stage config.Stage) []string {
	return resolveCommandHosts(stage.Host, stage.Hosts)
}

func resolveCommandHosts(host string, hosts []string) []string {
	if len(hosts) > 0 {
		return hosts
	}
	if strings.TrimSpace(host) != "" {
		return []string{host}
	}
	return []string{"local"}
}

func prepareStageCommand(ctx context.Context, stage config.Stage, host config.Host, runID string, client execution.ExecutionClient) (string, error) {
	return prepareNamedCommand(ctx, stage.Name, stage.Command, stage.Script, host, runID, client, "stage")
}

func prepareNamedCommand(ctx context.Context, name, command, script string, host config.Host, runID string, client execution.ExecutionClient, kind string) (string, error) {
	if strings.TrimSpace(command) != "" {
		return command, nil
	}
	if strings.TrimSpace(script) == "" {
		return "", fmt.Errorf("%s %s has no command or script", kind, name)
	}

	if strings.TrimSpace(host.IP) == "" {
		return fmt.Sprintf("bash ./%s", script), nil
	}

	localScriptPath := script
	if !filepath.IsAbs(localScriptPath) {
		if abs, err := filepath.Abs(localScriptPath); err == nil {
			localScriptPath = abs
		}
	}
	remoteScriptPath := filepath.Join("/tmp", fmt.Sprintf("benchctl-%s-%s", runID, filepath.Base(localScriptPath)))
	if err := client.Upload(ctx, localScriptPath, remoteScriptPath); err != nil {
		return "", fmt.Errorf("failed to upload script for %s %s: %w", kind, name, err)
	}
	return fmt.Sprintf("chmod +x '%s' && bash '%s'", remoteScriptPath, remoteScriptPath), nil
}

// startBackgroundStage starts a background stage by running the command in a new process group.
// this is kind of a hack but it makes it easier to monitor and implement them
func startBackgroundStage(ctx context.Context, client execution.ExecutionClient, envPrefix, commandBody string, stage config.Stage) (string, error) {
	backgroundCommand := fmt.Sprintf("%s setsid sh -c %s >/dev/null 2>&1 & echo $!", envPrefix, shellQuote(commandBody))
	result, err := client.RunCommand(ctx, execution.CommandRequest{Command: backgroundCommand})
	if err != nil {
		return "", fmt.Errorf("stage %s failed to start background command: %w", stage.Name, err)
	}
	pid := parsePID(result.Output)
	if pid == "" {
		return "", fmt.Errorf("stage %s failed to start background command: pid not captured", stage.Name)
	}
	return pid, nil
}

// runHealthCheck runs the health check for a stage.
func runHealthCheck(ctx context.Context, client execution.ExecutionClient, stage config.Stage, logger *slog.Logger) error {
	hc := stage.HealthCheck
	timeout, err := time.ParseDuration(hc.Timeout)
	if err != nil {
		err = fmt.Errorf("error parsing health check timeout for stage %s: %w", stage.Name, err)
		logError(logger, "health check failed", err, "stage", stage.Name)
		return err
	}
	logger.Info("health check started", "stage", stage.Name, "type", hc.Type)
	switch hc.Type {
	case "port":
		healthy, err := CallWithRetry(ctx, func() (bool, error) {
			return client.CheckPort(ctx, hc.Target, timeout)
		}, hc.Retries, time.Second)
		if err != nil {
			err = fmt.Errorf("health check for stage %s failed: %w", stage.Name, err)
			logError(logger, "health check failed", err, "stage", stage.Name, "type", hc.Type)
			return err
		}
		if !healthy {
			err := fmt.Errorf("health check for stage %s failed: port %s is not listening", stage.Name, hc.Target)
			logError(logger, "health check failed", err, "stage", stage.Name, "type", hc.Type, "target", hc.Target)
			return err
		}
		logger.Info("health check passed", "stage", stage.Name, "type", hc.Type, "target", hc.Target)
	default:
		err := fmt.Errorf("unknown health check type for stage %s: %s", stage.Name, hc.Type)
		logError(logger, "health check failed", err, "stage", stage.Name, "type", hc.Type)
		return err
	}
	return nil
}

// shellQuote quotes a command string for shell execution.
func shellQuote(cmd string) string {
	if cmd == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(cmd, "'", "'\"'\"'") + "'"
}

// parsePID parses the PID from the output of a command.
func parsePID(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// createLogger creates a logger based on the logging configuration and returns a logger, a writer, and a function to close the writer.
func createLogger(cfg *config.Config, runDir string) (*slog.Logger, io.Writer, func(), error) {
	level := slog.LevelInfo
	if cfg.Benchmark.Logging != nil {
		switch strings.ToLower(strings.TrimSpace(cfg.Benchmark.Logging.Level)) {
		case "debug":
			level = slog.LevelDebug
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}
	levelVar := new(slog.LevelVar)
	levelVar.Set(level)

	console := os.Stdout
	color := term.IsTerminal(int(console.Fd()))
	timeFormat := defaultConsoleTimeFormat
	if cfg.Benchmark.Logging != nil {
		if tf := strings.TrimSpace(cfg.Benchmark.Logging.TimeFormat); tf != "" {
			timeFormat = tf
		}
	}
	handlers := []slog.Handler{newConsoleHandler(console, levelVar, color, timeFormat)}
	closeFunc := func() {}
	logWriter := io.Writer(console)

	logPath := filepath.Join(runDir, "benchctl.ndjson")
	if cfg.Benchmark.Logging != nil {
		if strings.TrimSpace(cfg.Benchmark.Logging.Path) != "" {
			logPath = cfg.Benchmark.Logging.Path
		}
	}
	file, err := os.Create(logPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create log file: %w", err)
	}
	handlers = append(handlers, slog.NewJSONHandler(file, &slog.HandlerOptions{Level: levelVar}))
	closeFunc = func() { _ = file.Close() }
	logWriter = file

	return slog.New(slog.NewMultiHandler(handlers...)), logWriter, closeFunc, nil
}

// saveMetadata saves the metadata to a file
func saveMetadata(metadata *RunMetadata, runDir string) error {
	metadataPath := filepath.Join(runDir, "metadata.json")
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %v", err)
	}
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		return fmt.Errorf("failed to save metadata: %v", err)
	}
	return nil
}

// resolveConsoleWriter returns the console writer based on the terminal status of stdout
func resolveConsoleWriter() io.Writer {
	if os.Stdout != nil && term.IsTerminal(int(os.Stdout.Fd())) {
		return os.Stdout
	}
	return nil
}

// writersReferToSameFD checks if two writers refer to the same file descriptor
func writersReferToSameFD(a, b io.Writer) bool {
	fa, okA := a.(*os.File)
	fb, okB := b.(*os.File)
	if okA && okB {
		return fa.Fd() == fb.Fd()
	}
	return false
}

func resolveStageShell(cfg *config.Config, stage config.Stage) string {
	return resolveItemShell(cfg.Benchmark.Shell, stage.Shell)
}

func resolveCleanupShell(cfg *config.Config, step config.Cleanup) string {
	return resolveItemShell(cfg.Benchmark.Shell, step.Shell)
}

func resolveItemShell(benchmarkShell, itemShell string) string {
	shell := strings.TrimSpace(itemShell)
	if shell == "" {
		shell = strings.TrimSpace(benchmarkShell)
	}
	if shell == "" {
		shell = DefaultShell
	}
	return shell
}

func wrapWithShell(command, shell string) string {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		shell = DefaultShell
	}
	return fmt.Sprintf("%s %s", shell, shellQuote(command))
}

