// Package benchctl exposes the benchmark configuration and execution API for Go users.
package benchctl

import (
	"context"

	"github.com/luccadibe/benchctl/internal"
	"github.com/luccadibe/benchctl/internal/config"
)

type (
	Config        = config.Config
	Benchmark     = config.Benchmark
	LoggingConfig = config.LoggingConfig
	GitConfig     = config.GitConfig
	SyncConfig    = config.SyncConfig
	Host          = config.Host
	Case          = config.Case
	Stage         = config.Stage
	HealthCheck   = config.HealthCheck
	Output        = config.Output
	DataSchema    = config.DataSchema
	DataColumn    = config.DataColumn
	DataType      = config.DataType
	Option        = config.Option
	StageOption   = config.StageOption
	RunMetadata   = internal.RunMetadata
	RunResult     = internal.RunResult
)

const (
	DataTypeInteger   = config.DataTypeInteger
	DataTypeFloat     = config.DataTypeFloat
	DataTypeString    = config.DataTypeString
	DataTypeTimestamp = config.DataTypeTimestamp
)

var (
	NewConfig       = config.New
	WithShell       = config.WithShell
	WithLogging     = config.WithLogging
	WithGit         = config.WithGit
	WithSync        = config.WithSync
	WithHost        = config.WithHost
	WithCase        = config.WithCase
	WithStage       = config.WithStage
	SSHHost         = config.SSHHost
	NewStage        = config.NewStage
	OnHost          = config.OnHost
	OnHosts         = config.OnHosts
	RunCommand      = config.RunCommand
	RunScript       = config.RunScript
	StageShell      = config.StageShell
	Skip            = config.Skip
	ExecuteOnlyFor  = config.ExecuteOnlyFor
	Background      = config.Background
	WithHealthCheck = config.WithHealthCheck
	WithOutput      = config.WithOutput
	NewOutput       = config.NewOutput
	NewCSVSchema    = config.NewCSVSchema
	Column          = config.Column
	Unit            = config.Unit
	TimeFormat      = config.TimeFormat
	Bool            = config.Bool
)

// ParseYAML parses and validates a YAML benchmark configuration.
func ParseYAML(data []byte) (*Config, error) {
	return config.ParseYAML(data)
}

// Run executes a benchmark workflow.
func Run(ctx context.Context, cfg *Config, customMetadata map[string]string, envVars map[string]string) (*RunResult, error) {
	return internal.RunWorkflow(ctx, cfg, customMetadata, envVars)
}
