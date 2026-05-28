// Package run executes benchmarks and operates on completed run results.
package run

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/luccadibe/benchctl/internal"
	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/pkg/bench"
)

type (
	Result      = internal.RunResult
	RunResult   = internal.RunResult
	RunMetadata = internal.RunMetadata
)

type runParams struct {
	metadata map[string]string
	env      map[string]string
	skip     []string
	cases    []string
	timeout  time.Duration
}

// Option configures one invocation of Run.
type Option func(*runParams) error

// Run validates and executes a benchmark definition.
func Run(ctx context.Context, b *bench.Bench, opts ...Option) (*Result, error) {
	if b == nil {
		return nil, fmt.Errorf("benchmark is nil")
	}
	return runConfig(ctx, b.Config(), opts...)
}

// RunConfig executes a config directly. Prefer Run with bench.FromConfig for new code.
func RunConfig(ctx context.Context, cfg *bench.Config, opts ...Option) (*Result, error) {
	return runConfig(ctx, cfg, opts...)
}

func runConfig(ctx context.Context, cfg *config.Config, opts ...Option) (*Result, error) {
	if cfg == nil {
		return nil, fmt.Errorf("benchmark is nil")
	}

	params := runParams{}
	for _, opt := range opts {
		if err := opt(&params); err != nil {
			return nil, err
		}
	}

	runCtx := ctx
	if params.timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, params.timeout)
		defer cancel()
	}

	// we make a copy to avoid modifying the original config.
	// this allows re-using the same config for multiple runs while
	// applying different runtime options for each run.
	cloned := cfg.Clone()
	if err := applyRuntimeSkip(cloned, params.skip); err != nil {
		return nil, err
	}
	if err := applyRuntimeCases(cloned, params.cases); err != nil {
		return nil, err
	}
	if err := cloned.Validate(); err != nil {
		return nil, err
	}

	return internal.RunWorkflow(runCtx, cloned, params.metadata, params.env)
}

// WithMetadata adds a custom metadata key-value pair for this run.
func WithMetadata(key, value string) Option {
	return func(params *runParams) error {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("metadata key must be non-empty")
		}
		if params.metadata == nil {
			params.metadata = map[string]string{}
		}
		params.metadata[key] = value
		return nil
	}
}

// WithMetadataMap adds custom metadata for this run.
func WithMetadataMap(metadata map[string]string) Option {
	return func(params *runParams) error {
		if params.metadata == nil && len(metadata) > 0 {
			params.metadata = map[string]string{}
		}
		for key, value := range metadata {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("metadata key must be non-empty")
			}
			params.metadata[key] = value
		}
		return nil
	}
}

// WithEnv adds an environment variable for stages in this run.
func WithEnv(key, value string) Option {
	return func(params *runParams) error {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("environment key must be non-empty")
		}
		if params.env == nil {
			params.env = map[string]string{}
		}
		params.env[key] = value
		return nil
	}
}

// WithEnvMap adds environment variables for stages in this run.
func WithEnvMap(env map[string]string) Option {
	return func(params *runParams) error {
		if params.env == nil && len(env) > 0 {
			params.env = map[string]string{}
		}
		for key, value := range env {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("environment key must be non-empty")
			}
			params.env[key] = value
		}
		return nil
	}
}

// OnlyCase limits execution to a named comparison case for this run only.
func OnlyCase(name string) Option {
	return func(params *runParams) error {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("case name must be non-empty")
		}
		params.cases = append(params.cases, name)
		return nil
	}
}

// Skip skips a stage by name for this run only.
func Skip(stageName string) Option {
	return func(params *runParams) error {
		if strings.TrimSpace(stageName) == "" {
			return fmt.Errorf("skip stage name must be non-empty")
		}
		params.skip = append(params.skip, stageName)
		return nil
	}
}

// WithTimeout applies a timeout to this run.
func WithTimeout(timeout time.Duration) Option {
	return func(params *runParams) error {
		if timeout < 0 {
			return fmt.Errorf("timeout must be >= 0")
		}
		params.timeout = timeout
		return nil
	}
}

func applyRuntimeCases(cfg *config.Config, caseNames []string) error {
	if len(caseNames) == 0 {
		return nil
	}
	if len(cfg.Cases) == 0 {
		return fmt.Errorf("case filter requires cases in config")
	}

	requested := make(map[string]struct{}, len(caseNames))
	for _, caseName := range caseNames {
		name := strings.TrimSpace(caseName)
		if name == "" {
			return fmt.Errorf("case name must be non-empty")
		}
		requested[name] = struct{}{}
	}

	configured := make(map[string]struct{}, len(cfg.Cases))
	for _, benchmarkCase := range cfg.Cases {
		configured[benchmarkCase.Name] = struct{}{}
	}
	for name := range requested {
		if _, ok := configured[name]; !ok {
			return fmt.Errorf("unknown case for filter: %s", name)
		}
	}

	filtered := make([]config.Case, 0, len(requested))
	for _, benchmarkCase := range cfg.Cases {
		if _, ok := requested[benchmarkCase.Name]; ok {
			filtered = append(filtered, benchmarkCase)
		}
	}
	cfg.Cases = filtered
	return nil
}

func applyRuntimeSkip(cfg *config.Config, skipStages []string) error {
	if len(skipStages) == 0 {
		return nil
	}
	stageNames := make(map[string]int, len(cfg.Stages))
	for i, stage := range cfg.Stages {
		name := strings.TrimSpace(stage.Name)
		if name != "" {
			stageNames[name] = i
		}
	}
	for _, stageName := range skipStages {
		name := strings.TrimSpace(stageName)
		index, ok := stageNames[name]
		if !ok {
			return fmt.Errorf("unknown stage for skip: %s", name)
		}
		cfg.Stages[index].Skip = true
	}
	return nil
}
