package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExecutionClient defines the interface for executing commands on hosts
type ExecutionClient interface {
	RunCommand(ctx context.Context, command string) (string, int, error)
	CheckPort(ctx context.Context, port string, timeout time.Duration) (bool, error)
	Scp(ctx context.Context, remotePath, localPath string) error
	Close() error
}

// RunMetadata holds metadata about a benchmark run
type RunMetadata struct {
	RunID         string            `json:"run_id"`
	BenchmarkName string            `json:"benchmark_name"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	Config        *Config           `json:"config"`
	Hosts         map[string]Host   `json:"hosts"`
	Custom        map[string]string `json:"custom,omitempty"`
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
func RunWorkflow(cfg *Config, customMetadata map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Generate run ID and create run directory
	runID, err := generateRunID(cfg.Benchmark.OutputDir)
	if err != nil {
		log.Fatalf("Failed to generate run ID: %v", err)
	}

	runDir := filepath.Join(cfg.Benchmark.OutputDir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		log.Fatalf("Failed to create run directory: %v", err)
	}

	// Create metadata
	metadata := &RunMetadata{
		RunID:         runID,
		BenchmarkName: cfg.Benchmark.Name,
		StartTime:     time.Now(),
		Config:        cfg,
		Hosts:         cfg.Hosts,
		Custom:        customMetadata,
	}

	logger := createLogger(cfg)
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
		var c ExecutionClient
		var err error

		if host.IP == "" {
			// Local execution
			c = NewLocalClient()
		} else {
			// Remote execution via SSH
			c, err = NewSSHClient(host)
			if err != nil {
				logger.Fatalf("error creating ssh client: %v", err)
			}
		}

		if stage.Command != "" {
			r, ec, err := c.RunCommand(ctx, stage.Command)
			logger.Printf("Stage %s output: %s", stage.Name, r)
			if err != nil {
				logger.Fatalf("Stage %s failed: %v (exit code: %d)", stage.Name, err, ec)
			}
		} else if stage.Script != "" {
			command := fmt.Sprintf("bash ./%s", stage.Script)
			r, ec, err := c.RunCommand(ctx, command)
			logger.Printf("Stage %s output: %s", stage.Name, r)
			if err != nil {
				logger.Fatalf("Stage %s failed: %v (exit code: %d)", stage.Name, err, ec)
			}
		} else {
			logger.Fatalf("stage %s has no command or script", stage.Name)
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
					for colName, col := range output.DataSchema.Columns {
						if col.Type == DataTypeTimestamp && strings.TrimSpace(col.Format) == "" {
							logger.Printf("WARNING: data_schema.%s has type=timestamp without format; falling back to auto-detection which may be slower/ambiguous", colName)
						}
					}
				}
			}
		}

		// Close the client
		c.Close()
	}

	// plots
	if cfg.Plots != nil {
		plotsDir := filepath.Join(runDir, "plots")
		if err := os.MkdirAll(plotsDir, 0755); err != nil {
			logger.Fatalf("Failed to create plots directory: %v", err)
		}

		for _, plot := range cfg.Plots {
			logger.Printf("Generating plot: %s", plot.Name)
			var dataSourcePath string
			var matchedOutput *Output
			for _, stage := range cfg.Stages {
				if stage.Outputs == nil {
					continue
				}
				for _, output := range stage.Outputs {
					if output.Name == plot.Source {
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
				logger.Fatalf("plot %s references unknown output %s", plot.Name, plot.Source)
			}

			// Resolve export path: use config if provided, else default under run plots dir
			var plotExportPath string
			if plot.ExportPath != "" {
				if filepath.IsAbs(plot.ExportPath) {
					plotExportPath = plot.ExportPath
				} else {
					plotExportPath = filepath.Join(runDir, plot.ExportPath)
				}
			} else {
				plotExportPath = filepath.Join(plotsDir, plot.Name+".png")
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
			var plotter Plotter
			// Attach timestamp hint from data_schema (if available) for seaborn engine
			plotToRender := plot
			if plot.Engine == "" || plot.Engine == "seaborn" {
				if matchedOutput != nil && matchedOutput.DataSchema != nil {
					if col, ok := matchedOutput.DataSchema.Columns[plot.X]; ok {
						if col.Type == DataTypeTimestamp {
							if plotToRender.Options == nil {
								plotToRender.Options = map[string]any{}
							}
							if strings.TrimSpace(col.Format) != "" {
								plotToRender.Options["x_time_format"] = strings.ToLower(col.Format)
							}
							if strings.TrimSpace(col.Unit) != "" {
								plotToRender.Options["x_time_unit"] = strings.ToLower(col.Unit)
							}
						}
					}
				}
			}
			switch plot.Engine {
			case "", "seaborn":
				plotter = NewSeabornPlotter()
			case "gonum":
				plotter = NewGonumPlotter()
			default:
				logger.Fatalf("unknown plot engine: %s", plot.Engine)
			}
			if err := plotter.GeneratePlot(ctx, plotToRender, dataSourcePath, plotExportPath); err != nil {
				logger.Fatalf("failed to generate plot %s: %v", plot.Name, err)
			}
		}
	}

	// Save metadata
	metadata.EndTime = time.Now()
	metadataPath := filepath.Join(runDir, "metadata.json")
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		logger.Fatalf("Failed to marshal metadata: %v", err)
	}
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		logger.Fatalf("Failed to save metadata: %v", err)
	}

	logger.Printf("Workflow completed successfully!")
	logger.Printf("Results saved to: %s", runDir)
}

// createLogger creates a logger based on the logging configuration.
func createLogger(cfg *Config) *log.Logger {
	if cfg.Benchmark.Logging != nil {
		if cfg.Benchmark.Logging.Path != "" {
			file, err := os.Create(cfg.Benchmark.Logging.Path)
			if err != nil {
				log.Fatalf("error creating log file: %v", err)
			}
			defer file.Close()
			return log.New(file, "", log.LstdFlags)
		}
		return log.New(os.Stdout, "", log.LstdFlags)
	}
	//default
	return log.New(os.Stdout, "", log.LstdFlags)
}
