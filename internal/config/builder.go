package config

// Option configures a Config created with New.
type Option func(*Config)

// New creates a benchmark configuration that can be validated with Validate.
func New(name, outputDir string, options ...Option) *Config {
	cfg := &Config{
		Benchmark: Benchmark{
			Name:      name,
			OutputDir: outputDir,
		},
		Hosts: map[string]Host{"local": {}},
	}
	for _, option := range options {
		option(cfg)
	}
	return cfg
}

// WithShell sets the default shell used by stages.
func WithShell(shell string) Option {
	return func(cfg *Config) {
		cfg.Benchmark.Shell = shell
	}
}

// WithLogging sets the benchmark logging configuration.
func WithLogging(logging LoggingConfig) Option {
	return func(cfg *Config) {
		cfg.Benchmark.Logging = &logging
	}
}

// WithGit sets the benchmark git metadata capture configuration.
func WithGit(git GitConfig) Option {
	return func(cfg *Config) {
		cfg.Benchmark.Git = &git
	}
}

// WithSync sets the benchmark sync configuration.
func WithSync(sync SyncConfig) Option {
	return func(cfg *Config) {
		cfg.Benchmark.Sync = &sync
	}
}

// Bool returns a pointer to value for optional config booleans.
func Bool(value bool) *bool {
	return &value
}

// WithHost adds or replaces a host alias.
func WithHost(alias string, host Host) Option {
	return func(cfg *Config) {
		if cfg.Hosts == nil {
			cfg.Hosts = map[string]Host{}
		}
		cfg.Hosts[alias] = host
	}
}

// WithCase appends a comparison benchmark case.
func WithCase(name string, env map[string]string) Option {
	return func(cfg *Config) {
		benchmarkCase := Case{Name: name}
		if len(env) > 0 {
			benchmarkCase.Env = make(map[string]string, len(env))
			for key, value := range env {
				benchmarkCase.Env[key] = value
			}
		}
		cfg.Cases = append(cfg.Cases, benchmarkCase)
	}
}

// WithStage appends a workflow stage.
func WithStage(stage Stage) Option {
	return func(cfg *Config) {
		cfg.Stages = append(cfg.Stages, stage)
	}
}

// WithCleanup appends a workflow cleanup step.
func WithCleanup(step Cleanup) Option {
	return func(cfg *Config) {
		cfg.Cleanup = append(cfg.Cleanup, step)
	}
}

// SSHHost creates a remote SSH host configuration.
func SSHHost(ip, username, keyFile string) Host {
	return Host{IP: ip, Username: username, KeyFile: keyFile}
}

// StageOption configures a Stage created with NewStage.
type StageOption func(*Stage)

// NewStage creates a workflow stage.
func NewStage(name string, options ...StageOption) Stage {
	stage := Stage{Name: name}
	for _, option := range options {
		option(&stage)
	}
	return stage
}

// OnHost sets the stage host.
func OnHost(alias string) StageOption {
	return func(stage *Stage) {
		stage.Host = alias
	}
}

// OnHosts sets multiple stage hosts.
func OnHosts(aliases ...string) StageOption {
	return func(stage *Stage) {
		stage.Hosts = append([]string(nil), aliases...)
	}
}

// RunCommand sets the shell command for a stage.
func RunCommand(command string) StageOption {
	return func(stage *Stage) {
		stage.Command = command
	}
}

// RunScript sets the script path for a stage.
func RunScript(script string) StageOption {
	return func(stage *Stage) {
		stage.Script = script
	}
}

// StageShell overrides the default benchmark shell for a stage.
func StageShell(shell string) StageOption {
	return func(stage *Stage) {
		stage.Shell = shell
	}
}

// Skip marks a stage as skipped.
func Skip() StageOption {
	return func(stage *Stage) {
		stage.Skip = true
	}
}

// ExecuteOnlyFor limits a stage to one case name.
func ExecuteOnlyFor(caseName string) StageOption {
	return func(stage *Stage) {
		stage.ExecuteOnlyFor = caseName
	}
}

// Background marks a stage as a background stage.
func Background() StageOption {
	return func(stage *Stage) {
		stage.Background = true
	}
}

// WithHealthCheck sets the stage health check.
func WithHealthCheck(healthCheck HealthCheck) StageOption {
	return func(stage *Stage) {
		stage.HealthCheck = &healthCheck
	}
}

// WithOutput appends an output collection rule.
func WithOutput(output Output) StageOption {
	return func(stage *Stage) {
		stage.Outputs = append(stage.Outputs, output)
	}
}

// NewOutput creates an output collection rule.
func NewOutput(name, remotePath string) Output {
	return Output{Name: name, RemotePath: remotePath}
}

// CleanupOption configures a Cleanup created with NewCleanup.
type CleanupOption func(*Cleanup)

// NewCleanup creates a workflow cleanup step.
func NewCleanup(name string, options ...CleanupOption) Cleanup {
	step := Cleanup{Name: name}
	for _, option := range options {
		option(&step)
	}
	return step
}

// CleanupOnHost sets the cleanup host.
func CleanupOnHost(alias string) CleanupOption {
	return func(step *Cleanup) {
		step.Host = alias
	}
}

// CleanupOnHosts sets multiple cleanup hosts.
func CleanupOnHosts(aliases ...string) CleanupOption {
	return func(step *Cleanup) {
		step.Hosts = append([]string(nil), aliases...)
	}
}

// CleanupCommand sets the shell command for a cleanup step.
func CleanupCommand(command string) CleanupOption {
	return func(step *Cleanup) {
		step.Command = command
	}
}

// CleanupScript sets the script path for a cleanup step.
func CleanupScript(script string) CleanupOption {
	return func(step *Cleanup) {
		step.Script = script
	}
}

// CleanupShell overrides the default benchmark shell for a cleanup step.
func CleanupShell(shell string) CleanupOption {
	return func(step *Cleanup) {
		step.Shell = shell
	}
}

// Validate validates a config built with Go helpers.
func (cfg *Config) Validate() error {
	return validateConfig(cfg)
}
