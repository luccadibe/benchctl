package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/luccadibe/benchctl/internal"
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

					md := cmd.StringSlice(metadataFlag.Name)
					customMetadata, err := parseMetadata(md)
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

					internal.RunWorkflow(subCtx, cfg, customMetadata)
					return nil
				},
				Flags: []cli.Flag{
					metadataFlag,
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
					cfg := internal.GetDefaultConfigFile()
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
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func parseConfig(cfgFile string) (*internal.Config, error) {
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err

	}

	cfg, err := internal.ParseYAML(data)
	if err != nil {
		return nil, errors.New("Error parsing configuration file: " + err.Error())
	}
	return cfg, nil
}

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
