package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/internal/execution"
)

type resolvedOutput struct {
	name          string
	remotePath    string
	localFilename string
}

const escapedDollar = "\x00BENCHCTL_DOLLAR\x00"

func expandTemplate(template string, env map[string]string) (string, error) {
	if !strings.Contains(template, "$") {
		return template, nil
	}

	// Preserve literal dollars before os.Expand ($$ is not reliable with custom mappers).
	placeholder := strings.ReplaceAll(template, "$$", escapedDollar)

	var unknown []string
	expanded := os.Expand(placeholder, func(key string) string {
		value, ok := env[key]
		if !ok {
			unknown = append(unknown, key)
			return ""
		}
		return value
	})
	expanded = strings.ReplaceAll(expanded, escapedDollar, "$")
	if len(unknown) > 0 {
		sort.Strings(unknown)
		unknown = dedupeStrings(unknown)
		return "", fmt.Errorf("undefined variable %s", strings.Join(unknown, ", "))
	}
	return expanded, nil
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}

func resolveOutput(output config.Output, env map[string]string) (resolvedOutput, error) {
	name, err := expandTemplate(output.Name, env)
	if err != nil {
		return resolvedOutput{}, fmt.Errorf("name: %w", err)
	}
	if strings.TrimSpace(name) == "" {
		return resolvedOutput{}, fmt.Errorf("name is empty after expansion")
	}

	remotePath, err := expandTemplate(output.RemotePath, env)
	if err != nil {
		return resolvedOutput{}, fmt.Errorf("remote_path: %w", err)
	}
	if strings.TrimSpace(remotePath) == "" {
		return resolvedOutput{}, fmt.Errorf("remote_path is empty after expansion")
	}

	ext := filepath.Ext(remotePath)
	return resolvedOutput{
		name:          name,
		remotePath:    remotePath,
		localFilename: name + ext,
	}, nil
}

// prepareMetadataForSave resolves $VAR templates in the config snapshot written to
// metadata.json. Stage commands and outputs use the same env as execution; templates
// that would expand differently per case or host (e.g. ${BENCHCTL_HOST}) are left as-is.
func prepareMetadataForSave(metadata *RunMetadata, runID, runDir string, envVars map[string]string) {
	if metadata == nil || metadata.Config == nil {
		return
	}
	cfg := metadata.Config
	for i := range cfg.Stages {
		envs := stageEnvs(cfg, runID, runDir, envVars, cfg.Stages[i])
		stage := &cfg.Stages[i]
		if v, ok := uniqueExpansion(stage.Command, envs); ok {
			stage.Command = v
		}
		for j := range stage.Outputs {
			if v, ok := uniqueExpansion(stage.Outputs[j].Name, envs); ok {
				stage.Outputs[j].Name = v
			}
			if v, ok := uniqueExpansion(stage.Outputs[j].RemotePath, envs); ok {
				stage.Outputs[j].RemotePath = v
			}
		}
	}
	if len(metadata.Custom) == 0 {
		return
	}
	benchmarkCase := config.Case{}
	if cases := workflowCases(cfg); len(cases) == 1 {
		benchmarkCase = cases[0]
	}
	env := buildStageEnv(runID, runDir, cfg, envVars, benchmarkCase, "")
	for k, v := range metadata.Custom {
		if expanded, err := expandTemplate(v, env); err == nil {
			metadata.Custom[k] = expanded
		}
	}
}

// stageEnvs returns one env map per case/host combination that would run the stage.
func stageEnvs(cfg *config.Config, runID, runDir string, envVars map[string]string, stage config.Stage) []map[string]string {
	var envs []map[string]string
	for _, benchmarkCase := range workflowCases(cfg) {
		if !stageAppliesToCase(stage, benchmarkCase) {
			continue
		}
		for _, host := range resolveStageHosts(stage) {
			envs = append(envs, buildStageEnv(runID, runDir, cfg, envVars, benchmarkCase, host))
		}
	}
	return envs
}

// uniqueExpansion expands template when every env in envs yields the same result.
func uniqueExpansion(template string, envs []map[string]string) (string, bool) {
	if !strings.Contains(template, "$") {
		return template, true
	}
	if len(envs) == 0 {
		return "", false
	}
	first, err := expandTemplate(template, envs[0])
	if err != nil {
		return "", false
	}
	for _, env := range envs[1:] {
		expanded, err := expandTemplate(template, env)
		if err != nil || expanded != first {
			return "", false
		}
	}
	return first, true
}

func collectStageOutputs(
	ctx context.Context,
	client execution.ExecutionClient,
	runDir string,
	stage config.Stage,
	logger *slog.Logger,
	env map[string]string,
) error {
	for _, output := range stage.Outputs {
		resolved, err := resolveOutput(output, env)
		if err != nil {
			return fmt.Errorf("output %q in stage %s: %w", output.Name, stage.Name, err)
		}

		localPath := filepath.Join(runDir, resolved.localFilename)
		if err := client.Scp(ctx, resolved.remotePath, localPath); err != nil {
			return fmt.Errorf("failed to collect output %s for stage %s: %w", resolved.name, stage.Name, err)
		}
		logger.Info(
			"output collected",
			"output", resolved.name,
			"remote_path", resolved.remotePath,
			"local_path", localPath,
		)
	}
	return nil
}
