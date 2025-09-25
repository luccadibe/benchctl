package internal

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
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
	Benchmark  Benchmark       `yaml:"benchmark" json:"benchmark"`
	Hosts      map[string]Host `yaml:"hosts" json:"hosts"`
	DataSchema DataSchema      `yaml:"data_schema" json:"data_schema"`
	Stages     []Stage         `yaml:"stages" json:"stages"`
	Plots      []Plot          `yaml:"plots,omitempty" json:"plots,omitempty"`
}

// Benchmark holds top-level benchmark metadata.
type Benchmark struct {
	// the benchmark name to be used in metadata
	Name string `yaml:"name" json:"name"`
	// the directory to save the results
	OutputDir string `yaml:"output_dir" json:"output_dir"`
}

// DataSchema supports a fixed field (format) and inline column definitions.
type DataSchema struct {
	Format  string                `yaml:"format" json:"format" jsonschema:"enum=csv"`
	Columns map[string]DataColumn `json:"columns"`
}

// DataColumn represents a single column definition.
type DataColumn struct {
	Type DataType `yaml:"type" json:"type" jsonschema:"enum=timestamp,enum=integer,enum=float,enum=string"`
	Unit string   `yaml:"unit,omitempty" json:"unit,omitempty"`
}

// UnmarshalYAML supports a mixed mapping of a fixed field (format) and arbitrary column keys.
func (ds *DataSchema) UnmarshalYAML(unmarshal func(any) error) error {
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	ds.Columns = map[string]DataColumn{}
	for k, v := range raw {
		if k == "format" {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("data_schema.format must be a string")
			}
			ds.Format = s
			continue
		}
		b, err := yaml.Marshal(v)
		if err != nil {
			return fmt.Errorf("data_schema.%s: %w", k, err)
		}
		var col DataColumn
		if err := yaml.Unmarshal(b, &col); err != nil {
			return fmt.Errorf("data_schema.%s: %w", k, err)
		}
		if ds.Columns == nil {
			ds.Columns = map[string]DataColumn{}
		}
		ds.Columns[k] = col
	}
	return nil
}

// Host is a host in the benchmark. It can be a remote host or the local host.
type Host struct {
	IP          string `yaml:"ip,omitempty" json:"ip,omitempty"`
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
	Script      string      `yaml:"script,omitempty" json:"script,omitempty"`
	HealthCheck HealthCheck `yaml:"health_check,omitempty" json:"health_check,omitempty"`
	Outputs     []Output    `yaml:"outputs,omitempty" json:"outputs,omitempty"`
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
	RemotePath string `yaml:"remote_path" json:"remote_path"`
	// If not provided, saved under the run's output directory
	LocalPath string `yaml:"local_path,omitempty" json:"local_path,omitempty"`
}

// Plot is a future feature for visualization configuration.
type Plot struct {
	Name        string `yaml:"name" json:"name"`
	Title       string `yaml:"title" json:"title"`
	Type        string `yaml:"type" json:"type" jsonschema:"enum=time_series,enum=histogram,enum=boxplot"`
	X           string `yaml:"x,omitempty" json:"x,omitempty"`
	Y           string `yaml:"y,omitempty" json:"y,omitempty"`
	Aggregation string `yaml:"aggregation,omitempty" json:"aggregation,omitempty" jsonschema:"enum=avg,enum=median,enum=p95,enum=p99"`
	Format      string `yaml:"format,omitempty" json:"format,omitempty" jsonschema:"enum=png,enum=svg,enum=pdf"`
	ExportPath  string `yaml:"export_path,omitempty" json:"export_path,omitempty"`
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

	// hosts: allow empty for local only; no immediate checks here

	// data schema
	if strings.TrimSpace(cfg.DataSchema.Format) == "" {
		errs = append(errs, "data_schema.format must be set")
	}
	// Validate column types
	for colName, col := range cfg.DataSchema.Columns {
		switch col.Type {
		case DataTypeInteger, DataTypeFloat, DataTypeString, DataTypeTimestamp:
			// ok
		default:
			errs = append(errs, fmt.Sprintf("data_schema.%s.type must be one of [timestamp, integer, float, string]", colName))
		}
		_ = col.Unit // unit is optional
	}

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

		// health check validation
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

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
