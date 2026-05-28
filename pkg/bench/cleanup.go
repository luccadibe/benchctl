package bench

import "github.com/luccadibe/benchctl/internal/config"

// CleanupOption configures a cleanup step created with Cleanup.
type CleanupOption func(*config.Cleanup)

// Cleanup creates a workflow cleanup step.
func Cleanup(name string, opts ...CleanupOption) CleanupConfig {
	step := config.Cleanup{Name: name}
	for _, opt := range opts {
		opt(&step)
	}
	return step
}

// CleanupHost sets the cleanup host alias.
func CleanupHost(alias string) CleanupOption {
	return func(step *config.Cleanup) {
		step.Host = alias
	}
}

// CleanupHosts sets multiple cleanup host aliases.
func CleanupHosts(aliases ...string) CleanupOption {
	return func(step *config.Cleanup) {
		step.Hosts = append([]string(nil), aliases...)
	}
}

// CleanupCommand sets the shell command for a cleanup step.
func CleanupCommand(command string) CleanupOption {
	return func(step *config.Cleanup) {
		step.Command = command
	}
}

// CleanupScript sets the script path for a cleanup step.
func CleanupScript(script string) CleanupOption {
	return func(step *config.Cleanup) {
		step.Script = script
	}
}

// CleanupShell overrides the default benchmark shell for a cleanup step.
func CleanupShell(shell string) CleanupOption {
	return func(step *config.Cleanup) {
		step.Shell = shell
	}
}
