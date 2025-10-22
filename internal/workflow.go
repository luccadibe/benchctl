package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
func RunWorkflow(ctx context.Context, cfg *config.Config, customMetadata map[string]string) {
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

	// go through the stages and execute them
	for i, stage := range cfg.Stages {
		logger.Printf("Executing stage: %d/%d %s", i+1, len(cfg.Stages), stage.Name)
		host, ok := cfg.Hosts[stage.Host]
		if !ok {
			logger.Fatalf("stage %s references unknown host %s", stage.Name, stage.Host)
		}

		// Determine if this is a local host (no IP specified) or remote host
		var c execution.ExecutionClient
		var err error

		if host.IP == "" {
			// Local execution
			c = execution.NewLocalClient()
		} else {
			// Remote execution via SSH
			c, err = execution.NewSSHClient(host)
			if err != nil {
				logger.Fatalf("error creating ssh client: %v", err)
			}
		}

		// prepare env exports for this stage
		envPrefix := buildEnvPrefix(runID, runDir, cfg)

		consoleSink := resolveConsoleWriter()
		stdoutSink := consoleSink
		stderrSink := consoleSink
		usePTY := consoleSink != nil
		logStageOutput := consoleSink == nil || !writersReferToSameFD(consoleSink, logWriter)

		var lastOutput string
		var commandToRun string
		if stage.Command != "" {
			commandToRun = envPrefix + stage.Command
		} else if stage.Script != "" {
			// If remote host, upload the script first, then execute it
			command := ""
			if host.IP == "" {
				// Local execution, run relative path as before
				command = fmt.Sprintf("bash ./%s", stage.Script)
			} else {
				// Remote host: copy script to a temp path and execute there
				localScriptPath := stage.Script
				if !filepath.IsAbs(localScriptPath) {
					abs, err := filepath.Abs(localScriptPath)
					if err == nil {
						localScriptPath = abs
					}
				}
				remoteScriptPath := filepath.Join("/tmp", fmt.Sprintf("benchctl-%s-%s", runID, filepath.Base(localScriptPath)))
				if err := c.Upload(ctx, localScriptPath, remoteScriptPath); err != nil {
					logger.Fatalf("failed to upload script for stage %s: %v", stage.Name, err)
				}
				// ensure executable and run
				command = fmt.Sprintf("chmod +x '%s' && bash '%s'", remoteScriptPath, remoteScriptPath)
			}
			commandToRun = envPrefix + command
		} else {
			logger.Fatalf("stage %s has no command or script", stage.Name)
		}

		result, err := c.RunCommand(ctx, execution.CommandRequest{
			Command: commandToRun,
			Stdout:  stdoutSink,
			Stderr:  stderrSink,
			UsePTY:  usePTY,
		})
		if err != nil {
			if logStageOutput && strings.TrimSpace(result.Output) != "" {
				logger.Printf("Stage %s captured output:\n%s", stage.Name, result.Output)
			}
			logger.Fatalf("Stage %s failed: %v (exit code: %d)", stage.Name, err, result.ExitCode)
		}
		lastOutput = result.Output
		if logStageOutput {
			logger.Printf("Stage %s output: %s", stage.Name, result.Output)
		}
		logger.Printf("Stage %s completed (exit code: %d)", stage.Name, result.ExitCode)

		// If append_metadata is true, parse stdout as JSON and append into metadata.Custom
		if stage.AppendMetadata {
			var out map[string]any
			dec := json.NewDecoder(strings.NewReader(lastOutput))
			dec.UseNumber()
			if err := dec.Decode(&out); err != nil {
				logger.Printf("WARNING: Stage %s append_metadata enabled but output is not valid JSON: %v", stage.Name, err)
				logger.Printf("Stage %s output was: %s", stage.Name, lastOutput)
			} else {
				if metadata.Custom == nil {
					metadata.Custom = map[string]string{}
				}
				for k, v := range out {
					// Stringify values to store in Custom map[string]string
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
				logger.Printf("Stage %s metadata appended successfully", stage.Name)
			}
		}

		// if stage has a health check run it (use CallWithRetry)
		if stage.HealthCheck != nil {
			hc := stage.HealthCheck
			timeout, err := time.ParseDuration(hc.Timeout)
			if err != nil {
				logger.Fatalf("error parsing health check timeout: %v", err)
			}
			logger.Printf("Running health check: %s", hc.Type)
			switch hc.Type {
			case "port":
				healthy, err := CallWithRetry(ctx, func() (bool, error) {
					return c.CheckPort(ctx, hc.Target, timeout)
				}, hc.Retries, time.Second)
				if err != nil {
					logger.Fatalf("Health check for stage %s failed: %v", stage.Name, err)
				}
				if !healthy {
					logger.Fatalf("Health check for stage %s failed: port %s is not listening", stage.Name, hc.Target)
				}
				logger.Printf("Health check for stage %s passed: port %s is listening", stage.Name, hc.Target)
			default:
				// for now just port
				logger.Fatalf("Unknown health check type for stage %s: %s", stage.Name, hc.Type)
			}
		}

		// if stage has outputs, collect them
		if stage.Outputs != nil {
			for _, output := range stage.Outputs {
				remotePath := output.RemotePath
				localPath := output.LocalPath

				// If no local path specified, use run directory
				if localPath == "" {
					localPath = filepath.Join(runDir, output.Name+".csv")
				} else {
					// Make path relative to run directory if it's not absolute
					if !filepath.IsAbs(localPath) {
						localPath = filepath.Join(runDir, localPath)
					}
				}

				err := c.Scp(ctx, remotePath, localPath)
				if err != nil {
					logger.Fatalf("Failed to collect output %s for stage %s: %v", output.Name, stage.Name, err)
				}
				logger.Printf("Collected output %s: %s -> %s", output.Name, remotePath, localPath)

				// Warn if timestamp columns lack explicit format
				if output.DataSchema != nil {
					for _, col := range output.DataSchema.Columns {
						if col.Type == config.DataTypeTimestamp && strings.TrimSpace(col.Format) == "" {
							logger.Printf("WARNING: data_schema.%s has type=timestamp without format; falling back to auto-detection which may be slower/ambiguous", col.Name)
						}
					}
				}
			}
		}

		c.Close()
	}

	// plots
	if cfg.Plots != nil {
		plotsDir := filepath.Join(runDir, "plots")
		if err := os.MkdirAll(plotsDir, 0755); err != nil {
			logger.Fatalf("Failed to create plots directory: %v", err)
		}

		for _, plt := range cfg.Plots {
			logger.Printf("Generating plot: %s", plt.Name)
			var dataSourcePath string
			var matchedOutput *config.Output
			for _, stage := range cfg.Stages {
				if stage.Outputs == nil {
					continue
				}
				for _, output := range stage.Outputs {
					if output.Name == plt.Source {
						// Use the actual collected path
						if output.LocalPath == "" {
							dataSourcePath = filepath.Join(runDir, output.Name+".csv")
						} else if !filepath.IsAbs(output.LocalPath) {
							dataSourcePath = filepath.Join(runDir, output.LocalPath)
						} else {
							dataSourcePath = output.LocalPath
						}
						matchedOutput = &output
						break
					}
				}
				break
			}

			if dataSourcePath == "" {
				logger.Fatalf("plot %s references unknown output %s", plt.Name, plt.Source)
			}

			// Resolve export path: use config if provided, else default under run plots dir
			var plotExportPath string
			if plt.ExportPath != "" {
				if filepath.IsAbs(plt.ExportPath) {
					plotExportPath = plt.ExportPath
				} else {
					plotExportPath = filepath.Join(runDir, plt.ExportPath)
				}
			} else {
				plotExportPath = filepath.Join(plotsDir, plt.Name+".png")
			}

			// Ensure export directory exists
			exportDir := filepath.Dir(plotExportPath)
			if err := os.MkdirAll(exportDir, 0755); err != nil {
				logger.Fatalf("Failed to create plot export directory: %v", err)
			}

			// Make paths absolute for external engines
			absDataPath, err := filepath.Abs(dataSourcePath)
			if err == nil {
				dataSourcePath = absDataPath
			}
			absExportPath, err := filepath.Abs(plotExportPath)
			if err == nil {
				plotExportPath = absExportPath
			}

			// Generate the plot
			var plotter plot.Plotter
			// Attach timestamp from data_schema (if available) for seaborn engine
			plotToRender := plt
			if plt.Engine == "" || plt.Engine == "seaborn" {
				if matchedOutput != nil && matchedOutput.DataSchema != nil {
					// Find the column matching plot.X
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
			}
			switch plt.Engine {
			case "", "seaborn":
				plotter = plot.NewSeabornPlotter()
			case "gonum":
				plotter = plot.NewGonumPlotter()
			default:
				logger.Fatalf("unknown plot engine: %s", plt.Engine)
			}
			if err := plotter.GeneratePlot(ctx, plotToRender, dataSourcePath, plotExportPath); err != nil {
				logger.Fatalf("failed to generate plot %s: %v", plt.Name, err)
			}
		}
	}

	metadata.EndTime = time.Now()
	if err := saveMetadata(metadata, runDir); err != nil {
		logger.Fatalf("%s", err)
	}

	logger.Printf("Workflow completed successfully!")
	logger.Printf("Results saved to: %s", runDir)
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
)

func buildEnvPrefix(runID, runDir string, cfg *config.Config) string {
	configPath := os.Getenv(EnvConfigPath)
	exePath, _ := os.Executable()
	exports := []string{
		EnvRunID + "='" + runID + "'",
		EnvOutputDir + "='" + cfg.Benchmark.OutputDir + "'",
		EnvRunDir + "='" + runDir + "'",
	}
	if strings.TrimSpace(configPath) != "" {
		exports = append(exports, EnvConfigPath+"='"+configPath+"'")
	}
	if strings.TrimSpace(exePath) != "" {
		exports = append(exports, EnvBenchctl+"='"+exePath+"'")
	}
	return "export " + strings.Join(exports, " ") + "; "
}
