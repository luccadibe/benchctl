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
	"sort"
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
		return nil, err
	}
	metadata.Git = gitMetadata
	if gitMetadata != nil {
		logger.Info("git metadata captured", "commit", gitMetadata.Commit, "branch", gitMetadata.Branch, "dirty", gitMetadata.Dirty)
	}

	backgroundMgr := newBackgroundManager(logger)

	stageErr := executeStages(ctx, cfg, runID, runDir, logger, logWriter, metadata, backgroundMgr, envVars)
	stopErr := backgroundMgr.StopAll(ctx, runDir)
	if stageErr != nil {
		if stopErr != nil {
			logErrorf(logger, "stopping background stages: %v", stopErr)
		}
		return nil, fmt.Errorf("workflow failed: %w", errors.Join(stageErr, stopErr))
	}
	if stopErr != nil {
		return nil, fmt.Errorf("stop background stages: %w", stopErr)
	}

	metadata.EndTime = time.Now()
	if err := saveMetadata(metadata, runDir); err != nil {
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

	envPrefix := buildEnvPrefix(runID, runDir, cfg, envVars)
	consoleSink := resolveConsoleWriter()
	stdoutSink := consoleSink
	stderrSink := consoleSink
	usePTY := consoleSink != nil
	logStageOutput := consoleSink == nil || !writersReferToSameFD(consoleSink, logWriter)

	for i, stage := range cfg.Stages {
		if stage.Skip {
			logger.Info("stage skipped", "stage", stage.Name, "index", i+1, "total", len(cfg.Stages))
			continue
		}
		logger.Info("stage started", "stage", stage.Name, "index", i+1, "total", len(cfg.Stages))
		hostAliases := resolveStageHosts(stage)
		for _, hostAlias := range hostAliases {
			host, ok := cfg.Hosts[hostAlias]
			if !ok {
				if hostAlias != "local" {
					return fmt.Errorf("stage %s references unknown host %s", stage.Name, hostAlias)
				}
				host = config.Host{}
			}

			client, err := openExecutionClient(host)
			if err != nil {
				return fmt.Errorf("error creating execution client for stage %s: %w", stage.Name, err)
			}

			commandBody, err := prepareStageCommand(ctx, stage, host, runID, client)
			if err != nil {
				_ = client.Close()
				return err
			}

			commandBody = wrapWithShell(commandBody, resolveStageShell(cfg, stage))

			if stage.Background {
				pid, err := startBackgroundStage(ctx, client, envPrefix, commandBody, stage)
				_ = client.Close()
				if err != nil {
					return err
				}
				backgroundMgr.Add(backgroundStage{stage: stage, hostAlias: hostAlias, host: host, pid: pid})
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
				return fmt.Errorf("stage %s failed: %w (exit code: %d)", stage.Name, err, result.ExitCode)
			}

			if logStageOutput {
				logger.Info("stage output", "stage", stage.Name, "output", result.Output)
			}
			logger.Info("stage completed", "stage", stage.Name, "exit_code", result.ExitCode)

			if stage.HealthCheck != nil {
				if err := runHealthCheck(ctx, client, stage, logger); err != nil {
					_ = client.Close()
					return err
				}
			}

			if len(stage.Outputs) > 0 {
				if err := collectStageOutputs(ctx, client, runDir, stage, logger, hostAlias); err != nil {
					_ = client.Close()
					return err
				}
			}

			_ = client.Close()
		}
	}

	return nil
}

func resolveStageHosts(stage config.Stage) []string {
	if len(stage.Hosts) > 0 {
		return stage.Hosts
	}
	if strings.TrimSpace(stage.Host) != "" {
		return []string{stage.Host}
	}
	return []string{"local"}
}

func prepareStageCommand(ctx context.Context, stage config.Stage, host config.Host, runID string, client execution.ExecutionClient) (string, error) {
	if strings.TrimSpace(stage.Command) != "" {
		return stage.Command, nil
	}
	if strings.TrimSpace(stage.Script) == "" {
		return "", fmt.Errorf("stage %s has no command or script", stage.Name)
	}

	if strings.TrimSpace(host.IP) == "" {
		return fmt.Sprintf("bash ./%s", stage.Script), nil
	}

	localScriptPath := stage.Script
	if !filepath.IsAbs(localScriptPath) {
		if abs, err := filepath.Abs(localScriptPath); err == nil {
			localScriptPath = abs
		}
	}
	remoteScriptPath := filepath.Join("/tmp", fmt.Sprintf("benchctl-%s-%s", runID, filepath.Base(localScriptPath)))
	if err := client.Upload(ctx, localScriptPath, remoteScriptPath); err != nil {
		return "", fmt.Errorf("failed to upload script for stage %s: %w", stage.Name, err)
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
		return fmt.Errorf("error parsing health check timeout for stage %s: %w", stage.Name, err)
	}
	logger.Info("health check started", "stage", stage.Name, "type", hc.Type)
	switch hc.Type {
	case "port":
		healthy, err := CallWithRetry(ctx, func() (bool, error) {
			return client.CheckPort(ctx, hc.Target, timeout)
		}, hc.Retries, time.Second)
		if err != nil {
			return fmt.Errorf("health check for stage %s failed: %w", stage.Name, err)
		}
		if !healthy {
			return fmt.Errorf("health check for stage %s failed: port %s is not listening", stage.Name, hc.Target)
		}
		logger.Info("health check passed", "stage", stage.Name, "type", hc.Type, "target", hc.Target)
	default:
		return fmt.Errorf("unknown health check type for stage %s: %s", stage.Name, hc.Type)
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
	handlers := []slog.Handler{newConsoleHandler(console, levelVar, color)}
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

const (
	EnvRunID      = "BENCHCTL_RUN_ID"
	EnvOutputDir  = "BENCHCTL_OUTPUT_DIR"
	EnvRunDir     = "BENCHCTL_RUN_DIR"
	EnvConfigPath = "BENCHCTL_CONFIG_PATH"
	EnvBenchctl   = "BENCHCTL_BIN"
	DefaultShell  = "bash -lic"
)

func resolveStageShell(cfg *config.Config, stage config.Stage) string {
	shell := strings.TrimSpace(stage.Shell)
	if shell == "" {
		shell = strings.TrimSpace(cfg.Benchmark.Shell)
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

func buildEnvPrefix(runID, runDir string, cfg *config.Config, envVars map[string]string) string {
	configPath := os.Getenv(EnvConfigPath)
	exePath, _ := os.Executable()
	exports := []string{
		EnvRunID + "=" + shellQuote(runID),
		EnvOutputDir + "=" + shellQuote(cfg.Benchmark.OutputDir),
		EnvRunDir + "=" + shellQuote(runDir),
	}
	if strings.TrimSpace(configPath) != "" {
		exports = append(exports, EnvConfigPath+"="+shellQuote(configPath))
	}
	if strings.TrimSpace(exePath) != "" {
		exports = append(exports, EnvBenchctl+"="+shellQuote(exePath))
	}
	if len(envVars) > 0 {
		keys := make([]string, 0, len(envVars))
		for key := range envVars {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			exports = append(exports, key+"="+shellQuote(envVars[key]))
		}
	}
	return "export " + strings.Join(exports, " ") + "; "
}
