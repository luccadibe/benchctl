package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	_ "embed"

	"github.com/goccy/go-yaml"
	"github.com/luccadibe/benchctl/internal/plot"
)

// DataType represents allowed CSV column data types in configuration.
type DataType string

const (
	// allowed data types for the csv output (match design.md)
	DataTypeInteger   DataType = "integer"
	DataTypeFloat     DataType = "float"
	DataTypeString    DataType = "string"
	DataTypeTimestamp DataType = "timestamp"
)

// Config mirrors the YAML configuration shape.
type Config struct {
	Benchmark Benchmark       `yaml:"benchmark" json:"benchmark"`
	Hosts     map[string]Host `yaml:"hosts" json:"hosts"`
	Stages    []Stage         `yaml:"stages" json:"stages"`
	Plots     []plot.Plot     `yaml:"plots,omitempty" json:"plots,omitempty"`
}

// Benchmark holds top-level benchmark metadata.
type Benchmark struct {
	// the benchmark name to be used in metadata
	Name string `yaml:"name" json:"name"`
	// the directory to save the results
	OutputDir string `yaml:"output_dir" json:"output_dir"`
	// the logging configuration
	Logging *LoggingConfig `yaml:"logging,omitempty" json:"logging,omitempty"`
}

// LoggingConfig holds the logging configuration. If no path is provided, logs are written to stdout.
type LoggingConfig struct {
	Level string `yaml:"level" json:"level"`
	Path  string `yaml:"path,omitempty" json:"path,omitempty"`
}

// DataSchema defines the schema for structured data outputs (e.g., CSV files).
type DataSchema struct {
	Format  string       `yaml:"format" json:"format" jsonschema:"enum=csv"`
	Columns []DataColumn `yaml:"columns" json:"columns"`
}

// DataColumn represents a single column definition with a name.
type DataColumn struct {
	Name string   `yaml:"name" json:"name"`
	Type DataType `yaml:"type" json:"type" jsonschema:"enum=timestamp,enum=integer,enum=float,enum=string"`
	Unit string   `yaml:"unit,omitempty" json:"unit,omitempty"`
	// Format is only applicable for timestamp type. Supported: unix, unix_ms, unix_us, unix_ns, rfc3339, rfc3339_nano, iso8601
	Format string `yaml:"format,omitempty" json:"format,omitempty"`
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

// Stage is a step in the workflow.
type Stage struct {
	Name    string `yaml:"name" json:"name"`
	Host    string `yaml:"host,omitempty" json:"host,omitempty"`
	Command string `yaml:"command,omitempty" json:"command,omitempty"`
	// Script is a path to the script to execute. It will be copied to the host and executed.
	Script string `yaml:"script,omitempty" json:"script,omitempty"`
	// If true, the command/script stdout is expected to be a JSON object whose
	// keys/values will be appended to run metadata under .Custom
	AppendMetadata bool `yaml:"append_metadata,omitempty" json:"append_metadata,omitempty"`
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
	LocalPath  string      `yaml:"local_path,omitempty" json:"local_path,omitempty"`
	DataSchema *DataSchema `yaml:"data_schema,omitempty" json:"data_schema,omitempty"`
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

	// hosts: allow empty for local only

	// stages
	hostAliases := map[string]struct{}{}
	for k := range cfg.Hosts {
		hostAliases[k] = struct{}{}
	}
	// Always allow "local" as a valid host alias
	hostAliases["local"] = struct{}{}

	for i := range cfg.Stages {
		st := &cfg.Stages[i]
		if strings.TrimSpace(st.Name) == "" {
			errs = append(errs, fmt.Sprintf("stages[%d].name must be set", i))
		}
		if st.Host != "" {
			if _, ok := hostAliases[st.Host]; !ok {
				errs = append(errs, fmt.Sprintf("stages[%d].host references unknown host '%s'", i, st.Host))
			}
		}
		hasCmd := strings.TrimSpace(st.Command) != ""
		hasScript := strings.TrimSpace(st.Script) != ""
		if hasCmd == hasScript {
			errs = append(errs, "exactly one of command or script must be set")
		}

		if st.Background && st.AppendMetadata {
			errs = append(errs, fmt.Sprintf("stages[%d] with background=true cannot set append_metadata", i))
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
			if output.DataSchema != nil {
				if strings.TrimSpace(output.DataSchema.Format) == "" {
					errs = append(errs, fmt.Sprintf("stages[%d].outputs[%d].data_schema.format must be set", i, j))
				}
				// Validate columns
				for k, col := range output.DataSchema.Columns {
					if strings.TrimSpace(col.Name) == "" {
						errs = append(errs, fmt.Sprintf("stages[%d].outputs[%d].data_schema.columns[%d].name must be set", i, j, k))
					}
					switch col.Type {
					case DataTypeInteger, DataTypeFloat, DataTypeString, DataTypeTimestamp:
						// ok
					default:
						errs = append(errs, fmt.Sprintf("stages[%d].outputs[%d].data_schema.columns[%d].type must be one of [timestamp, integer, float, string]", i, j, k))
					}
					_ = col.Unit // unit is optional
					// If timestamp with format, validate allowed formats
					if col.Type == DataTypeTimestamp && strings.TrimSpace(col.Format) != "" {
						switch strings.ToLower(col.Format) {
						case "unix", "unix_ms", "unix_us", "unix_ns", "rfc3339", "rfc3339_nano", "iso8601":
							// ok
						default:
							errs = append(errs, fmt.Sprintf("stages[%d].outputs[%d].data_schema.columns[%d].format must be one of [unix, unix_ms, unix_us, unix_ns, rfc3339, rfc3339_nano, iso8601]", i, j, k))
						}
					}
				}
			}
		}
	}

	// Collect all output names for plot validation
	outputNames := map[string]struct{}{}
	for _, stage := range cfg.Stages {
		for _, output := range stage.Outputs {
			if strings.TrimSpace(output.Name) != "" {
				outputNames[output.Name] = struct{}{}
			}
		}
	}

	// Validate plots
	for i := range cfg.Plots {
		plot := &cfg.Plots[i]
		if strings.TrimSpace(plot.Name) == "" {
			errs = append(errs, fmt.Sprintf("plots[%d].name must be set", i))
		}
		if strings.TrimSpace(plot.Source) == "" {
			errs = append(errs, fmt.Sprintf("plots[%d].source must be set", i))
		} else if _, exists := outputNames[plot.Source]; !exists {
			errs = append(errs, fmt.Sprintf("plots[%d].source references unknown output '%s'", i, plot.Source))
		}
		if strings.TrimSpace(plot.Type) != "" {
			switch plot.Type {
			case "time_series", "histogram", "boxplot":
				// ok
			default:
				errs = append(errs, fmt.Sprintf("plots[%d].type must be one of [time_series, histogram, boxplot]", i))
			}
		}
		if strings.TrimSpace(plot.Aggregation) != "" {
			switch plot.Aggregation {
			case "avg", "median", "p95", "p99":
				// ok
			default:
				errs = append(errs, fmt.Sprintf("plots[%d].aggregation must be one of [avg, median, p95, p99]", i))
			}
		}
		if strings.TrimSpace(plot.Format) != "" {
			switch plot.Format {
			case "png", "svg", "pdf":
				// ok
			default:
				errs = append(errs, fmt.Sprintf("plots[%d].format must be one of [png, svg, pdf]", i))
			}
		}
		engine := strings.TrimSpace(plot.Engine)
		if engine != "" {
			switch engine {
			case "gonum", "seaborn":
				// ok
			default:
				errs = append(errs, fmt.Sprintf("plots[%d].engine must be one of [gonum, seaborn]", i))
			}
		} else {
			plot.Engine = "seaborn" // default to seaborn
			engine = "seaborn"
		}
		if strings.TrimSpace(plot.GroupBy) != "" && engine == "gonum" {
			errs = append(errs, fmt.Sprintf("plots[%d].groupby is only supported with the seaborn engine", i))
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
