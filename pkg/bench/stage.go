package bench

import (
	"time"

	"github.com/luccadibe/benchctl/internal/config"
)

// StageOption configures a stage created with Stage or BackgroundStage.
type StageOption func(*config.Stage)

// Stage creates a foreground workflow stage.
func Stage(name string, opts ...StageOption) StageConfig {
	stage := config.Stage{Name: name}
	for _, opt := range opts {
		opt(&stage)
	}
	return stage
}

// BackgroundStage creates a background workflow stage.
func BackgroundStage(name string, opts ...StageOption) StageConfig {
	stage := Stage(name, opts...)
	stage.Background = true
	return stage
}

// Host sets the stage host alias.
func Host(alias string) StageOption {
	return func(stage *config.Stage) {
		stage.Host = alias
	}
}

// Hosts sets the stage host aliases.
func Hosts(aliases ...string) StageOption {
	return func(stage *config.Stage) {
		stage.Hosts = append([]string(nil), aliases...)
	}
}

// Command sets the shell command for a stage.
func Command(command string) StageOption {
	return func(stage *config.Stage) {
		stage.Command = command
	}
}

// Script sets the script path for a stage.
func Script(script string) StageOption {
	return func(stage *config.Stage) {
		stage.Script = script
	}
}

// Shell overrides the default benchmark shell for a stage.
func Shell(shell string) StageOption {
	return func(stage *config.Stage) {
		stage.Shell = shell
	}
}

// OnlyFor limits a stage to one benchmark case name.
func OnlyFor(caseName string) StageOption {
	return func(stage *config.Stage) {
		stage.ExecuteOnlyFor = caseName
	}
}

// Background marks a stage as a background stage.
func Background() StageOption {
	return func(stage *config.Stage) {
		stage.Background = true
	}
}

// Outputs appends output collection rules.
func Outputs(outputs ...OutputConfig) StageOption {
	return func(stage *config.Stage) {
		stage.Outputs = append(stage.Outputs, outputs...)
	}
}

// Output creates and appends one output collection rule.
func Output(name string, opts ...OutputOption) StageOption {
	return Outputs(NewOutput(name, opts...))
}

// HealthCheck sets a stage health check.
func HealthCheck(healthCheck HealthConfig) StageOption {
	return func(stage *config.Stage) {
		stage.HealthCheck = &healthCheck
	}
}

// PortCheck sets a port health check.
func PortCheck(port string, opts ...HealthOption) StageOption {
	return HealthCheck(newHealthCheck("port", port, opts...))
}

// HTTPCheck sets an HTTP health check.
func HTTPCheck(url string, opts ...HealthOption) StageOption {
	return HealthCheck(newHealthCheck("http", url, opts...))
}

// CommandCheck sets a command health check.
func CommandCheck(command string, opts ...HealthOption) StageOption {
	return HealthCheck(newHealthCheck("command", command, opts...))
}

// HealthOption configures a health check.
type HealthOption func(*config.HealthCheck)

// Timeout sets a health check timeout.
func Timeout(timeout time.Duration) HealthOption {
	return func(healthCheck *config.HealthCheck) {
		healthCheck.Timeout = timeout.String()
	}
}

// Retries sets a health check retry count.
func Retries(retries int) HealthOption {
	return func(healthCheck *config.HealthCheck) {
		healthCheck.Retries = retries
	}
}

func newHealthCheck(kind, target string, opts ...HealthOption) HealthConfig {
	healthCheck := config.HealthCheck{Type: kind, Target: target}
	for _, opt := range opts {
		opt(&healthCheck)
	}
	return healthCheck
}
