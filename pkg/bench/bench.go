// Package bench defines static benchmark configuration.
package bench

import (
	"fmt"
	"os"

	"github.com/luccadibe/benchctl/internal/config"
)

type (
	Config        = config.Config
	Benchmark     = config.Benchmark
	LoggingConfig = config.LoggingConfig
	GitConfig     = config.GitConfig
	SyncConfig    = config.SyncConfig
	HostConfig    = config.Host
	Case          = config.Case
	StageConfig   = config.Stage
	HealthConfig  = config.HealthCheck
	OutputConfig  = config.Output
)

// Bench is a benchmark definition that can be run one or more times.
type Bench struct {
	cfg *config.Config
}

// Option configures a Bench created with New.
type Option func(*config.Config)

// New creates a benchmark definition. Use WithResultsPath to set output_dir.
func New(name string, opts ...Option) *Bench {
	cfg := config.New(name, "")
	for _, opt := range opts {
		opt(cfg)
	}
	return &Bench{cfg: cfg}
}

// FromConfig wraps an existing config as a Bench.
func FromConfig(cfg *Config) *Bench {
	return &Bench{cfg: cfg}
}

// FromYAML loads a benchmark definition from YAML bytes.
func FromYAML(data []byte) (*Bench, error) {
	cfg, err := config.ParseYAML(data)
	if err != nil {
		return nil, err
	}
	return &Bench{cfg: cfg}, nil
}

// FromFile loads a benchmark definition from a YAML file.
func FromFile(path string) (*Bench, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return FromYAML(data)
}

// Config returns the underlying validated config shape used by YAML workflows.
func (b *Bench) Config() *Config {
	if b == nil {
		return nil
	}
	return b.cfg
}

// Validate validates the benchmark definition without running it.
func (b *Bench) Validate() error {
	if b == nil || b.cfg == nil {
		return fmt.Errorf("benchmark is nil")
	}
	return b.cfg.Validate()
}

// WithResultsPath sets benchmark.output_dir.
func WithResultsPath(path string) Option {
	return func(cfg *config.Config) {
		cfg.Benchmark.OutputDir = path
	}
}

// WithShell sets the default shell for stages.
func WithShell(shell string) Option {
	return func(cfg *config.Config) {
		cfg.Benchmark.Shell = shell
	}
}

// WithLogging sets benchmark logging configuration.
func WithLogging(logging LoggingConfig) Option {
	return func(cfg *config.Config) {
		cfg.Benchmark.Logging = &logging
	}
}

// WithGit sets git metadata capture configuration.
func WithGit(git GitConfig) Option {
	return func(cfg *config.Config) {
		cfg.Benchmark.Git = &git
	}
}

// WithSync sets result sync configuration.
func WithSync(sync SyncConfig) Option {
	return func(cfg *config.Config) {
		cfg.Benchmark.Sync = &sync
	}
}

// WithHost adds or replaces a host alias.
func WithHost(alias string, host HostConfig) Option {
	return func(cfg *config.Config) {
		if cfg.Hosts == nil {
			cfg.Hosts = map[string]config.Host{}
		}
		cfg.Hosts[alias] = host
	}
}

// WithCases replaces the benchmark cases.
func WithCases(cases ...Case) Option {
	return func(cfg *config.Config) {
		cfg.Cases = append([]config.Case(nil), cases...)
	}
}

// WithStages replaces the benchmark stage list.
func WithStages(stages ...StageConfig) Option {
	return func(cfg *config.Config) {
		cfg.Stages = append([]config.Stage(nil), stages...)
	}
}

// Local returns the built-in local host configuration.
func Local() HostConfig {
	return config.Host{}
}

// SSH creates a remote SSH host configuration.
func SSH(ip, username, keyFile string) HostConfig {
	return config.SSHHost(ip, username, keyFile)
}

// Bool returns a pointer to value for optional config booleans.
func Bool(value bool) *bool {
	return config.Bool(value)
}

// LogInfo creates an info-level logging config.
func LogInfo() LoggingConfig { return LoggingConfig{Level: "info"} }

// LogDebug creates a debug-level logging config.
func LogDebug() LoggingConfig { return LoggingConfig{Level: "debug"} }

// LogWarn creates a warn-level logging config.
func LogWarn() LoggingConfig { return LoggingConfig{Level: "warn"} }

// LogError creates an error-level logging config.
func LogError() LoggingConfig { return LoggingConfig{Level: "error"} }

// RequireClean creates git config that controls dirty-worktree behavior.
func RequireClean(require bool) GitConfig {
	return GitConfig{Capture: Bool(true), RequireClean: require}
}

// DisableGit disables git metadata capture.
func DisableGit() GitConfig {
	return GitConfig{Capture: Bool(false)}
}
