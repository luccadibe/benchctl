package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/internal/execution"
	"github.com/luccadibe/benchctl/internal/plot"
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

// RunWorkflow executes a benchmark workflow with run ID tracking
func RunWorkflow(ctx context.Context, cfg *config.Config, customMetadata map[string]string, envVars map[string]string) {
	// Generate run ID and create run directory
	runID, err := generateRunID(cfg.Benchmark.OutputDir)
	if err != nil {
		log.Fatalf("Failed to generate run ID: %v", err)
	}

	runDir := filepath.Join(cfg.Benchmark.OutputDir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		log.Fatalf("Failed to create run directory: %v", err)
	}

	metadata := &RunMetadata{
		RunID:         runID,
		BenchmarkName: cfg.Benchmark.Name,
		StartTime:     time.Now(),
		Config:        cfg,
		Hosts:         cfg.Hosts,
		Custom:        customMetadata,
	}

	logger, logWriter, closeLogger := createLogger(cfg)
	defer closeLogger()
	logger.Printf("Run ID: %s", runID)
	logger.Printf("Results will be saved to: %s", runDir)

	backgroundMgr := newBackgroundManager(logger)

	stageErr := executeStages(ctx, cfg, runID, runDir, logger, logWriter, metadata, backgroundMgr, envVars)
	stopErr := backgroundMgr.StopAll(ctx, runDir)
	if stageErr != nil {
		if stopErr != nil {
			logger.Printf("ERROR stopping background stages: %v", stopErr)
		}
		logger.Fatalf("workflow failed: %v", stageErr)
	}
	if stopErr != nil {
		logger.Fatalf("failed to stop background stages: %v", stopErr)
	}

	if err := generatePlotsForRun(ctx, cfg, runDir, logger); err != nil {
		logger.Fatalf("%v", err)
	}

	metadata.EndTime = time.Now()
	if err := saveMetadata(metadata, runDir); err != nil {
		logger.Fatalf("%s", err)
	}

	logger.Printf("Workflow completed successfully!")
	logger.Printf("Results saved to: %s", runDir)
}

// executeStages executes the stages of the benchmark
// it returns an error if any stage fails,
// nil if all stages succeed.
func executeStages(
	ctx context.Context,
	cfg *config.Config,
	runID, runDir string,
	logger *log.Logger,
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
			logger.Printf("Skipping stage: %d/%d %s", i+1, len(cfg.Stages), stage.Name)
			continue
		}
		logger.Printf("Executing stage: %d/%d %s", i+1, len(cfg.Stages), stage.Name)
		hostAliases := resolveStageHosts(stage)
		for hostIndex, hostAlias := range hostAliases {
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
				logger.Printf("Stage %s is running in background", stage.Name)
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
					logger.Printf("Stage %s captured output:\n%s", stage.Name, result.Output)
				}
				_ = client.Close()
				return fmt.Errorf("stage %s failed: %w (exit code: %d)", stage.Name, err, result.ExitCode)
			}

			if logStageOutput {
				logger.Printf("Stage %s output: %s", stage.Name, result.Output)
			}
			logger.Printf("Stage %s completed (exit code: %d)", stage.Name, result.ExitCode)

			if stage.AppendMetadata {
				if hostIndex == 0 {
					if len(hostAliases) > 1 {
						logger.Printf("WARNING: stage %s append_metadata enabled with multiple hosts; only first host will be used", stage.Name)
					}
					if err := appendStageMetadata(stage, metadata, result.Output); err != nil {
						logger.Printf("WARNING: %v", err)
						logger.Printf("Stage %s output was: %s", stage.Name, result.Output)
					} else {
						logger.Printf("Stage %s metadata appended successfully", stage.Name)
					}
				}
			}

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

// appendStageMetadata appends the stage's JSON marshalled output to the run metadata.
func appendStageMetadata(stage config.Stage, metadata *RunMetadata, output string) error {
	var out map[string]any
	dec := json.NewDecoder(strings.NewReader(output))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return fmt.Errorf("stage %s append_metadata enabled but output is not valid JSON: %w", stage.Name, err)
	}
	if metadata.Custom == nil {
		metadata.Custom = map[string]string{}
	}
	for k, v := range out {
		switch t := v.(type) {
		case json.Number:
			metadata.Custom[k] = t.String()
		case string:
			metadata.Custom[k] = t
		default:
			b, _ := json.Marshal(t)
			metadata.Custom[k] = string(b)
		}
	}
	return nil
}

// runHealthCheck runs the health check for a stage.
func runHealthCheck(ctx context.Context, client execution.ExecutionClient, stage config.Stage, logger *log.Logger) error {
	hc := stage.HealthCheck
	timeout, err := time.ParseDuration(hc.Timeout)
	if err != nil {
		return fmt.Errorf("error parsing health check timeout for stage %s: %w", stage.Name, err)
	}
	logger.Printf("Running health check: %s", hc.Type)
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
		logger.Printf("Health check for stage %s passed: port %s is listening", stage.Name, hc.Target)
	default:
		return fmt.Errorf("unknown health check type for stage %s: %s", stage.Name, hc.Type)
	}
	return nil
}

func generatePlotsForRun(ctx context.Context, cfg *config.Config, runDir string, logger *log.Logger) error {
	if len(cfg.Plots) == 0 {
		return nil
	}

	for _, plt := range cfg.Plots {
		logger.Printf("Generating plot: %s", plt.Name)
		var dataSourcePath string
		var matchedOutput *config.Output
		for _, stage := range cfg.Stages {
			if stage.Outputs == nil {
				continue
			}
			hostAlias := resolveStageHosts(stage)[0]
			for _, output := range stage.Outputs {
				if output.Name == plt.Source {
					filename := outputFilename(output, stage, hostAlias)
					dataSourcePath = filepath.Join(runDir, filename)
					copy := output
					matchedOutput = &copy
					break
				}
			}
			if dataSourcePath != "" {
				break
			}
		}

		if dataSourcePath == "" {
			return fmt.Errorf("plot %s references unknown output %s", plt.Name, plt.Source)
		}

		// Construct plot filename as plot.Name + "." + plot.Format
		plotFormat := plt.Format
		if plotFormat == "" {
			plotFormat = "png"
		}
		plotExportPath := filepath.Join(runDir, plt.Name+"."+plotFormat)

		if absData, err := filepath.Abs(dataSourcePath); err == nil {
			dataSourcePath = absData
		}
		if absExport, err := filepath.Abs(plotExportPath); err == nil {
			plotExportPath = absExport
		}

		plotToRender := plt
		if (plt.Engine == "" || plt.Engine == "seaborn") && matchedOutput != nil && matchedOutput.DataSchema != nil {
			for _, col := range matchedOutput.DataSchema.Columns {
				if col.Name == plt.X && col.Type == config.DataTypeTimestamp {
					if plotToRender.Options == nil {
						plotToRender.Options = map[string]any{}
					}
					if strings.TrimSpace(col.Format) != "" {
						plotToRender.Options["x_time_format"] = strings.ToLower(col.Format)
					}
					if strings.TrimSpace(col.Unit) != "" {
						plotToRender.Options["x_time_unit"] = strings.ToLower(col.Unit)
					}
					break
				}
			}
		}

		var plotter plot.Plotter
		switch plt.Engine {
		case "", "seaborn":
			plotter = plot.NewSeabornPlotter()
		case "gonum":
			plotter = plot.NewGonumPlotter()
		default:
			return fmt.Errorf("unknown plot engine: %s", plt.Engine)
		}

		if err := plotter.GeneratePlot(ctx, plotToRender, dataSourcePath, plotExportPath); err != nil {
			return fmt.Errorf("failed to generate plot %s: %w", plt.Name, err)
		}
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
func createLogger(cfg *config.Config) (*log.Logger, io.Writer, func()) {
	if cfg.Benchmark.Logging != nil {
		if cfg.Benchmark.Logging.Path != "" {
			file, err := os.Create(cfg.Benchmark.Logging.Path)
			if err != nil {
				log.Fatalf("error creating log file: %v", err)
			}
			return log.New(file, "", log.LstdFlags), file, func() {
				_ = file.Close()
			}
		}
	}
	stdout := os.Stdout
	return log.New(stdout, "", log.LstdFlags), stdout, func() {}
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
