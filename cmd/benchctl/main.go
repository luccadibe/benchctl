package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/luccadibe/benchctl/internal"
	"github.com/luccadibe/benchctl/internal/config"
	"github.com/urfave/cli/v3"
)

var configFlag = &cli.StringFlag{
	Name:    "config",
	Usage:   "path to the configuration file",
	Value:   "benchmark.yaml",
	Aliases: []string{"c"},
}

var timeoutFlag = &cli.DurationFlag{
	Name:    "timeout",
	Usage:   "timeout for the benchmark (default: no timeout)",
	Aliases: []string{"t"},
}

var verboseFlag = &cli.BoolFlag{
	Name:    "verbose",
	Usage:   "verbose output",
	Value:   false,
	Aliases: []string{"v"},
}

var metadataFlag = &cli.StringSliceFlag{
	Name:    "metadata",
	Usage:   "Custom metadata in the format 'key=value' (can be used multiple times)",
	Aliases: []string{"m"},
}
var environmentFlag = &cli.StringSliceFlag{
	Name:    "environment",
	Usage:   "Environment variable in the format 'KEY=VALUE' (can be used multiple times)",
	Aliases: []string{"e"},
}
var skipFlag = &cli.StringSliceFlag{
	Name:  "skip",
	Usage: "Skip stages by name (can be used multiple times)",
}
var uiAddrFlag = &cli.StringFlag{
	Name:  "addr",
	Usage: "Address to serve the UI (host:port)",
	Value: "127.0.0.1:8787",
}

func main() {

	cmd := &cli.Command{
		Name:  "benchctl",
		Usage: "Manage benchmark workflows",
		Flags: []cli.Flag{
			configFlag,
			verboseFlag,
		},
		Commands: []*cli.Command{
			// run
			{
				Name:  "run",
				Usage: "Run a benchmark",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfgFile := cmd.String(configFlag.Name)
					if env := os.Getenv("BENCHCTL_CONFIG_PATH"); strings.TrimSpace(env) != "" {
						cfgFile = env
					}
					cfg, err := parseConfig(cfgFile)
					if err != nil {
						return err
					}

					skipStages := cmd.StringSlice(skipFlag.Name)
					if err := applySkipFlags(cfg, skipStages); err != nil {
						return err
					}

					md := cmd.StringSlice(metadataFlag.Name)
					customMetadata, err := parseMetadata(md)
					if err != nil {
						return err
					}
					envEntries := cmd.StringSlice(environmentFlag.Name)
					envVars, err := parseEnvironment(envEntries)
					if err != nil {
						return err
					}
					timeout := cmd.Duration(timeoutFlag.Name)
					timeoutProvided := cmd.IsSet(timeoutFlag.Name)

					var subCtx context.Context
					var cancel context.CancelFunc
					if timeoutProvided && timeout > 0 {
						subCtx, cancel = context.WithTimeout(ctx, timeout)
						defer cancel()
					} else {
						subCtx = ctx
					}

					internal.RunWorkflow(subCtx, cfg, customMetadata, envVars)
					return nil
				},
				Flags: []cli.Flag{
					metadataFlag,
					environmentFlag,
					skipFlag,
					timeoutFlag,
				},
			},
			// init
			{
				Name:  "init",
				Usage: "Initialize your benchmark configuration file",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := cmd.String("name")
					file := cmd.String("file")
					cfg := config.GetDefaultConfigFile()
					cfg = strings.Replace(cfg, "my-benchmark", name, 1)
					err := os.WriteFile(file, []byte(cfg), 0644)
					if err != nil {
						return err
					}
					return nil
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "name",
						Usage: "Name of the benchmark",
						Value: "my-benchmark",
					},
					&cli.StringFlag{
						Name:  "file",
						Usage: "Output file name",
						Value: "benchmark.yaml",
					},
				},
			},
			// inspect
			{
				Name:  "inspect",
				Usage: "Inspect a benchmark run",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfgFile := cmd.String(configFlag.Name)
					if env := os.Getenv("BENCHCTL_CONFIG_PATH"); strings.TrimSpace(env) != "" {
						cfgFile = env
					}
					cfg, err := parseConfig(cfgFile)
					if err != nil {
						return err
					}
					runId := cmd.Args().Get(0)
					if runId == "" {
						return fmt.Errorf("run-id is required")
					}
					runPath := filepath.Join(cfg.Benchmark.OutputDir, runId)
					fmt.Println(internal.InspectRun(runPath, cmd.Bool(verboseFlag.Name)))
					return nil
				},
				Flags: []cli.Flag{
					configFlag,
				},
			},
			// edit
			{
				Name:  "edit",
				Usage: "Edit a benchmark run's metadata",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfgFile := cmd.String(configFlag.Name)
					if env := os.Getenv("BENCHCTL_CONFIG_PATH"); strings.TrimSpace(env) != "" {
						cfgFile = env
					}
					cfg, err := parseConfig(cfgFile)
					if err != nil {
						return err
					}
					runId := cmd.Args().Get(0)
					if runId == "" {
						return fmt.Errorf("run-id is required")
					}
					runPath := filepath.Join(cfg.Benchmark.OutputDir, runId)
					md := cmd.StringSlice(metadataFlag.Name)
					extraMd, err := parseMetadata(md)
					if err != nil {
						return err
					}
					err = internal.AddMetadata(runPath, extraMd)
					if err != nil {
						return err
					}
					fmt.Println("Metadata edited successfully")
					return nil
				},
				Flags: []cli.Flag{
					metadataFlag,
				},
			},
			// compare
			{
				Name:  "compare",
				Usage: "Compare two benchmark runs",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					runId1 := cmd.Args().Get(0)
					runId2 := cmd.Args().Get(1)
					if runId1 == "" {
						return fmt.Errorf("first run-id is required")
					}
					if runId2 == "" {
						return fmt.Errorf("second run-id is required")
					}
					cfgFile := cmd.String(configFlag.Name)
					cfg, err := parseConfig(cfgFile)
					if err != nil {
						return err
					}
					runPath1 := filepath.Join(cfg.Benchmark.OutputDir, runId1)
					runPath2 := filepath.Join(cfg.Benchmark.OutputDir, runId2)
					runmd1, err := internal.LoadRunMetadata(filepath.Join(runPath1, "metadata.json"))
					if err != nil {
						return err
					}
					runmd2, err := internal.LoadRunMetadata(filepath.Join(runPath2, "metadata.json"))
					if err != nil {
						return err
					}
					results, err := internal.CompareRunMetadata(runmd1, runmd2)
					if err != nil {
						return err
					}
					fmt.Println(internal.PrintComparisonResults(results))
					return nil
				},
			},
			// ui
			{
				Name:  "ui",
				Usage: "Serve the local web UI",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfgFile := cmd.String(configFlag.Name)
					if env := os.Getenv("BENCHCTL_CONFIG_PATH"); strings.TrimSpace(env) != "" {
						cfgFile = env
					}
					cfg, err := parseConfig(cfgFile)
					if err != nil {
						return err
					}
					addr := cmd.String(uiAddrFlag.Name)
					fmt.Printf("Serving UI at http://%s\n", addr)
					fmt.Printf("Runs directory: %s\n", cfg.Benchmark.OutputDir)
					return internal.ServeUI(addr, cfg.Benchmark.OutputDir)
				},
				Flags: []cli.Flag{
					configFlag,
					uiAddrFlag,
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func parseConfig(cfgFile string) (*config.Config, error) {
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err

	}

	cfg, err := config.ParseYAML(data)
	if err != nil {
		return nil, errors.New("Error parsing configuration file: " + err.Error())
	}
	return cfg, nil
}

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func parseMetadata(md []string) (map[string]string, error) {
	customMetadata := make(map[string]string)
	for _, metadataFlag := range md {
		parts := strings.SplitN(metadataFlag, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("Invalid metadata format: " + metadataFlag + ". Expected format: key=value")
		}
		customMetadata[parts[0]] = parts[1]
	}
	return customMetadata, nil
}

// used to parse the --environment / -e flag
func parseEnvironment(entries []string) (map[string]string, error) {
	envVars := make(map[string]string)
	for _, entry := range entries {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid environment format: %s. Expected format: KEY=VALUE", entry)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("Invalid environment format: %s. Expected format: KEY=VALUE", entry)
		}
		if !envKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("Invalid environment key: %s", key)
		}
		envVars[key] = parts[1]
	}
	return envVars, nil
}

func applySkipFlags(cfg *config.Config, skipStages []string) error {
	if len(skipStages) == 0 {
		return nil
	}

	stageNames := make(map[string]int, len(cfg.Stages))
	for i, stage := range cfg.Stages {
		name := strings.TrimSpace(stage.Name)
		if name == "" {
			continue
		}
		stageNames[name] = i
	}

	skipSet := make(map[string]struct{}, len(skipStages))
	for _, stageName := range skipStages {
		name := strings.TrimSpace(stageName)
		if name == "" {
			return fmt.Errorf("skip stage name must be non-empty")
		}
		skipSet[name] = struct{}{}
	}

	for name := range skipSet {
		index, ok := stageNames[name]
		if !ok {
			return fmt.Errorf("unknown stage for --skip: %s", name)
		}
		cfg.Stages[index].Skip = true
	}

	return nil
}
