package bench

import "github.com/luccadibe/benchctl/internal/config"

// OutputOption configures an output created with NewOutput.
type OutputOption func(*config.Output)

// NewOutput creates an output collection rule.
func NewOutput(name string, opts ...OutputOption) OutputConfig {
	output := config.Output{Name: name}
	for _, opt := range opts {
		opt(&output)
	}
	return output
}

// RemotePath sets the remote path to collect.
func RemotePath(path string) OutputOption {
	return func(output *config.Output) {
		output.RemotePath = path
	}
}
