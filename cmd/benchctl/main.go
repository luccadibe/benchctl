package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/luccadibe/benchctl/internal"
	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/pkg/bench"
	"github.com/luccadibe/benchctl/pkg/run"
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

func main() {
	internal.ConfigureDefaultLogger()

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
					bench, err := parseBench(cfgFile)
					if err != nil {
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
					var runOptions []run.Option
					if len(customMetadata) > 0 {
						runOptions = append(runOptions, run.WithMetadataMap(customMetadata))
					}
					if len(envVars) > 0 {
						runOptions = append(runOptions, run.WithEnvMap(envVars))
					}
					for _, stageName := range cmd.StringSlice(skipFlag.Name) {
						runOptions = append(runOptions, run.Skip(stageName))
					}
					if cmd.IsSet(timeoutFlag.Name) {
						runOptions = append(runOptions, run.WithTimeout(cmd.Duration(timeoutFlag.Name)))
					}

					_, err = run.Run(ctx, bench, runOptions...)
					return err
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
					bench, err := parseBench(cfgFile)
					if err != nil {
						return err
					}
					runId := cmd.Args().Get(0)
					if runId == "" {
						return fmt.Errorf("run-id is required")
					}
					runPath := filepath.Join(bench.Config().Benchmark.OutputDir, runId)
					fmt.Println(run.Inspect(runPath, cmd.Bool(verboseFlag.Name)))
					return nil
				},
				Flags: []cli.Flag{
					configFlag,
				},
			},
			// annotate
			{
				Name:  "annotate",
				Usage: "Annotate a completed benchmark run's metadata",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfgFile := cmd.String(configFlag.Name)
					if env := os.Getenv("BENCHCTL_CONFIG_PATH"); strings.TrimSpace(env) != "" {
						cfgFile = env
					}
					bench, err := parseBench(cfgFile)
					if err != nil {
						return err
					}
					runId := cmd.Args().Get(0)
					if runId == "" {
						return fmt.Errorf("run-id is required")
					}
					runPath := filepath.Join(bench.Config().Benchmark.OutputDir, runId)
					md := cmd.StringSlice(metadataFlag.Name)
					extraMd, err := parseMetadata(md)
					if err != nil {
						return err
					}
					err = run.Annotate(runPath, extraMd)
					if err != nil {
						return err
					}
					fmt.Println("Run annotated successfully")
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
					bench, err := parseBench(cfgFile)
					if err != nil {
						return err
					}
					runPath1 := filepath.Join(bench.Config().Benchmark.OutputDir, runId1)
					runPath2 := filepath.Join(bench.Config().Benchmark.OutputDir, runId2)
					runmd1, err := run.LoadMetadata(runPath1)
					if err != nil {
						return err
					}
					runmd2, err := run.LoadMetadata(runPath2)
					if err != nil {
						return err
					}
					results, err := run.Compare(runmd1, runmd2)
					if err != nil {
						return err
					}
					fmt.Println(run.FormatComparison(results))
					return nil
				},
			},
			// sync
			{
				Name:  "sync",
				Usage: "Sync benchmark results using rclone",
				Commands: []*cli.Command{
					{
						Name:  "push",
						Usage: "Push benchmark results to benchmark.sync.remote",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							cfgFile := cmd.String(configFlag.Name)
							if env := os.Getenv("BENCHCTL_CONFIG_PATH"); strings.TrimSpace(env) != "" {
								cfgFile = env
							}
							bench, err := parseBench(cfgFile)
							if err != nil {
								return err
							}
							return run.SyncPush(ctx, bench)
						},
						Flags: []cli.Flag{
							configFlag,
						},
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

func parseBench(cfgFile string) (*bench.Bench, error) {
	b, err := bench.FromFile(cfgFile)
	if err != nil {
		return nil, errors.New("Error parsing configuration file: " + err.Error())
	}
	return b, nil
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
