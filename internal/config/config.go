package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	_ "embed"

	"github.com/goccy/go-yaml"
)

// Config mirrors the YAML configuration shape.
type Config struct {
	Benchmark Benchmark       `yaml:"benchmark" json:"benchmark"`
	Hosts     map[string]Host `yaml:"hosts" json:"hosts"`
	Cases     []Case          `yaml:"cases,omitempty" json:"cases,omitempty"`
	Stages    []Stage         `yaml:"stages" json:"stages"`
}

// Benchmark holds top-level benchmark metadata.
type Benchmark struct {
	// the benchmark name to be used in metadata
	Name string `yaml:"name" json:"name"`
	// the directory to save the results
	OutputDir string `yaml:"output_dir" json:"output_dir"`
	// Shell command used to execute stages (default: "bash -lic").
	Shell string `yaml:"shell,omitempty" json:"shell,omitempty" jsonschema:"default=bash -lic"`
	// the logging configuration
	Logging *LoggingConfig `yaml:"logging,omitempty" json:"logging,omitempty"`
	// Git controls automatic repository metadata capture.
	Git *GitConfig `yaml:"git,omitempty" json:"git,omitempty"`
	// Sync controls optional result sync via rclone.
	Sync *SyncConfig `yaml:"sync,omitempty" json:"sync,omitempty"`
}

// LoggingConfig controls slog level and the JSON log file path.
// Human-readable logs always go to stdout; JSON logs default to benchctl.ndjson in the run directory.
type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	Path       string `yaml:"path,omitempty" json:"path,omitempty"`
	TimeFormat string `yaml:"time_format,omitempty" json:"time_format,omitempty"`
}

// GitConfig holds automatic git metadata capture settings.
type GitConfig struct {
	Capture      *bool `yaml:"capture,omitempty" json:"capture,omitempty"`
	RequireClean bool  `yaml:"require_clean,omitempty" json:"require_clean,omitempty"`
	SavePatch    bool  `yaml:"save_patch,omitempty" json:"save_patch,omitempty"`
}

// SyncConfig holds result synchronization settings.
type SyncConfig struct {
	Remote string   `yaml:"remote" json:"remote"`
	Args   []string `yaml:"args,omitempty" json:"args,omitempty"`
}

// Host is a host in the benchmark. It can be a remote host or the local host.
type Host struct {
	IP          string `yaml:"ip,omitempty" json:"ip,omitempty"`
	Port        int    `yaml:"port,omitempty" json:"port,omitempty"`
	Username    string `yaml:"username,omitempty" json:"username,omitempty"`
	Password    string `yaml:"password,omitempty" json:"password,omitempty"`
	KeyFile     string `yaml:"key_file,omitempty" json:"key_file,omitempty"`
	KeyPassword string `yaml:"key_password,omitempty" json:"key_password,omitempty"`
}

// Case describes a comparison benchmark case.
type Case struct {
	Name string            `yaml:"name" json:"name"`
	Env  map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// Stage is a step in the workflow.
type Stage struct {
	Name    string   `yaml:"name" json:"name"`
	Host    string   `yaml:"host,omitempty" json:"host,omitempty"`
	Hosts   []string `yaml:"hosts,omitempty" json:"hosts,omitempty"`
	Command string   `yaml:"command,omitempty" json:"command,omitempty"`
	// Script is a path to the script to execute. It will be copied to the host and executed.
	Script string `yaml:"script,omitempty" json:"script,omitempty"`
	// Shell command used to execute this stage (defaults to benchmark.shell).
	Shell string `yaml:"shell,omitempty" json:"shell,omitempty"`
	// Whether the stage should be skipped.
	Skip bool `yaml:"skip,omitempty" json:"skip,omitempty"`
	// ExecuteOnlyFor limits a stage to one case name when cases are configured.
	ExecuteOnlyFor string `yaml:"execute_only_for,omitempty" json:"execute_only_for,omitempty"`
	// Whether the stage should be ran in the background, allowing execution to continue with other stages.
	// Stages running in the background will be sent a SIGTERM when the last non-background
	// task is executed.
	Background  bool         `yaml:"background,omitempty" json:"background,omitempty"`
	HealthCheck *HealthCheck `yaml:"health_check,omitempty" json:"health_check,omitempty"`
	Outputs     []Output     `yaml:"outputs,omitempty" json:"outputs,omitempty"`
}

// HealthCheck determines readiness/success for a stage.
type HealthCheck struct {
	Type    string `yaml:"type,omitempty" json:"type,omitempty" jsonschema:"enum=port,enum=http,enum=file,enum=process,enum=command"`
	Target  string `yaml:"target,omitempty" json:"target,omitempty"`
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retries int    `yaml:"retries,omitempty" json:"retries,omitempty"`
}

// Output is a file to collect after the stage is executed. (Optional)
type Output struct {
	Name       string `yaml:"name" json:"name"`
	RemotePath string `yaml:"remote_path" json:"remote_path"`
	// If not provided, saved under the run's output directory
	LocalPath string `yaml:"local_path,omitempty" json:"local_path,omitempty"`
}

// ParseYAML loads and validates configuration using strict decoding.
func ParseYAML(data []byte) (*Config, error) {
	var config Config
	if err := yaml.UnmarshalWithOptions(data, &config, yaml.Strict()); err != nil {
		return nil, err
	}
	if err := validateConfig(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func validateConfig(cfg *Config) error {
	var errs []string

	// benchmark
	if strings.TrimSpace(cfg.Benchmark.Name) == "" {
		errs = append(errs, "benchmark.name must be set")
	}
	if strings.TrimSpace(cfg.Benchmark.OutputDir) == "" {
		errs = append(errs, "benchmark.output_dir must be set")
	}
	if cfg.Benchmark.Sync != nil && strings.TrimSpace(cfg.Benchmark.Sync.Remote) == "" {
		errs = append(errs, "benchmark.sync.remote must be set")
	}

	// hosts: allow empty for local only

	// stages
	hostAliases := map[string]struct{}{}
	for k := range cfg.Hosts {
		hostAliases[k] = struct{}{}
	}
	// Always allow "local" as a valid host alias
	hostAliases["local"] = struct{}{}

	stageNames := map[string]int{}
	caseNames := map[string]int{}
	for i, benchmarkCase := range cfg.Cases {
		name := strings.TrimSpace(benchmarkCase.Name)
		if name == "" {
			errs = append(errs, fmt.Sprintf("cases[%d].name must be set", i))
			continue
		}
		if previous, exists := caseNames[name]; exists {
			errs = append(errs, fmt.Sprintf("cases[%d].name duplicates cases[%d] (%s)", i, previous, name))
			continue
		}
		caseNames[name] = i
	}
	for i := range cfg.Stages {
		st := &cfg.Stages[i]
		if strings.TrimSpace(st.Name) == "" {
			errs = append(errs, fmt.Sprintf("stages[%d].name must be set", i))
		} else {
			name := strings.TrimSpace(st.Name)
			if prevIndex, exists := stageNames[name]; exists {
				errs = append(errs, fmt.Sprintf("stages[%d].name duplicates stages[%d] (%s)", i, prevIndex, name))
			} else {
				stageNames[name] = i
			}
		}
		if st.Host != "" && len(st.Hosts) > 0 {
			errs = append(errs, fmt.Sprintf("stages[%d] cannot set both host and hosts", i))
		}
		if st.Host != "" {
			if _, ok := hostAliases[st.Host]; !ok {
				errs = append(errs, fmt.Sprintf("stages[%d].host references unknown host '%s'", i, st.Host))
			}
		}
		if len(st.Hosts) > 0 {
			seen := make(map[string]struct{}, len(st.Hosts))
			for _, hostAlias := range st.Hosts {
				if strings.TrimSpace(hostAlias) == "" {
					errs = append(errs, fmt.Sprintf("stages[%d].hosts entries must be non-empty", i))
					continue
				}
				if _, ok := seen[hostAlias]; ok {
					errs = append(errs, fmt.Sprintf("stages[%d].hosts contains duplicate host '%s'", i, hostAlias))
					continue
				}
				seen[hostAlias] = struct{}{}
				if _, ok := hostAliases[hostAlias]; !ok {
					errs = append(errs, fmt.Sprintf("stages[%d].hosts references unknown host '%s'", i, hostAlias))
				}
			}
		}
		hasCmd := strings.TrimSpace(st.Command) != ""
		hasScript := strings.TrimSpace(st.Script) != ""
		if hasCmd == hasScript {
			errs = append(errs, "exactly one of command or script must be set")
		}
		if strings.TrimSpace(st.ExecuteOnlyFor) != "" {
			if len(cfg.Cases) == 0 {
				errs = append(errs, fmt.Sprintf("stages[%d].execute_only_for requires cases", i))
			} else if _, ok := caseNames[st.ExecuteOnlyFor]; !ok {
				errs = append(errs, fmt.Sprintf("stages[%d].execute_only_for references unknown case '%s'", i, st.ExecuteOnlyFor))
			}
		}

		// health check validation
		if st.HealthCheck != nil {
			hc := st.HealthCheck
			if strings.TrimSpace(hc.Type) != "" {
				switch hc.Type {
				case "port", "http", "file", "process", "command":
					// ok
				default:
					errs = append(errs, "health_check.type must be one of [port, http, file, process, command]")
				}
			}
			if strings.TrimSpace(hc.Timeout) != "" {
				d, err := time.ParseDuration(hc.Timeout)
				if err != nil || d <= 0 {
					errs = append(errs, "timeout must be a positive duration")
				}
			}
			if hc.Retries < 0 {
				errs = append(errs, "retries must be >= 0")
			}
		}

		// outputs validation
		for j, output := range st.Outputs {
			if strings.TrimSpace(output.Name) == "" {
				errs = append(errs, fmt.Sprintf("stages[%d].outputs[%d].name must be set", i, j))
			}
			if strings.TrimSpace(output.RemotePath) == "" {
				errs = append(errs, fmt.Sprintf("stages[%d].outputs[%d].remote_path must be set", i, j))
			}
			if strings.TrimSpace(output.LocalPath) != "" {
				errs = append(errs, fmt.Sprintf("stages[%d].outputs[%d].local_path is not allowed; files are stored directly in the run directory using output.name", i, j))
			}
		}
	}

	// Collect all output names for uniqueness checks.
	outputNames := map[string][]int{} // map from output name to stage indices
	for i, stage := range cfg.Stages {
		for _, output := range stage.Outputs {
			if strings.TrimSpace(output.Name) != "" {
				outputNames[output.Name] = append(outputNames[output.Name], i)
			}
		}
	}

	// Check for duplicate output names
	for name, stageIndices := range outputNames {
		if len(stageIndices) > 1 {
			errs = append(errs, fmt.Sprintf("output name '%s' is not unique; found in stages: %v", name, stageIndices))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func GetDefaultConfigFile() string {
	return string(defaultConfigFile)
}

//go:embed files/default_benchmark.yaml
var defaultConfigFile []byte
