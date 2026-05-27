package bench

import "github.com/luccadibe/benchctl/internal/config"

// CaseOption configures a case created with NewCase.
type CaseOption func(*config.Case)

// NewCase creates a benchmark case.
func NewCase(name string, opts ...CaseOption) Case {
	benchmarkCase := config.Case{Name: name}
	for _, opt := range opts {
		opt(&benchmarkCase)
	}
	return benchmarkCase
}

// Env sets one environment variable on a case.
func Env(key, value string) CaseOption {
	return func(benchmarkCase *config.Case) {
		if benchmarkCase.Env == nil {
			benchmarkCase.Env = map[string]string{}
		}
		benchmarkCase.Env[key] = value
	}
}

// EnvMap sets environment variables on a case.
func EnvMap(env map[string]string) CaseOption {
	return func(benchmarkCase *config.Case) {
		if benchmarkCase.Env == nil && len(env) > 0 {
			benchmarkCase.Env = map[string]string{}
		}
		for key, value := range env {
			benchmarkCase.Env[key] = value
		}
	}
}
