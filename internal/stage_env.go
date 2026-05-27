package internal

import (
	"os"
	"sort"
	"strings"

	"github.com/luccadibe/benchctl/internal/config"
)

const (
	EnvRunID      = "BENCHCTL_RUN_ID"
	EnvOutputDir  = "BENCHCTL_OUTPUT_DIR"
	EnvRunDir     = "BENCHCTL_RUN_DIR"
	EnvConfigPath = "BENCHCTL_CONFIG_PATH"
	EnvBenchctl   = "BENCHCTL_BIN"
	EnvCaseName   = "BENCHCTL_CASE_NAME"
	EnvHost       = "BENCHCTL_HOST"
	DefaultShell  = "bash -lic"
)

// buildStageEnv returns variables exported to stage commands and available for
// expanding stages[].outputs name and remote_path templates.
func buildStageEnv(
	runID, runDir string,
	cfg *config.Config,
	envVars map[string]string,
	benchmarkCase config.Case,
	hostAlias string,
) map[string]string {
	env := make(map[string]string, 8+len(envVars)+len(benchmarkCase.Env))
	env[EnvRunID] = runID
	env[EnvOutputDir] = cfg.Benchmark.OutputDir
	env[EnvRunDir] = runDir

	if configPath := strings.TrimSpace(os.Getenv(EnvConfigPath)); configPath != "" {
		env[EnvConfigPath] = configPath
	}
	if exePath, err := os.Executable(); err == nil && strings.TrimSpace(exePath) != "" {
		env[EnvBenchctl] = exePath
	}
	for key, value := range envVars {
		env[key] = value
	}
	if name := strings.TrimSpace(benchmarkCase.Name); name != "" {
		env[EnvCaseName] = name
	}
	for key, value := range benchmarkCase.Env {
		env[key] = value
	}
	if alias := strings.TrimSpace(hostAlias); alias != "" {
		env[EnvHost] = alias
	}
	return env
}

func envPrefixFromMap(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	exports := make([]string, 0, len(keys))
	for _, key := range keys {
		exports = append(exports, key+"="+shellQuote(env[key]))
	}
	return "export " + strings.Join(exports, " ") + "; "
}
