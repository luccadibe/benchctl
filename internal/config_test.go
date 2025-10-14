//go:build unit

package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadConfig_Success validates a happy-path config.
func TestLoadConfig_Success(t *testing.T) {
	cfgPath := filepath.Join("testdata", "1.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", cfgPath, err)
	}

	cfg, err := ParseYAML(data)
	if err != nil {
		t.Fatalf("unexpected error parsing %s: %v", cfgPath, err)
	}

	if cfg.Benchmark.Name == "" {
		t.Fatalf("expected benchmark.name to be set")
	}
	if cfg.Benchmark.OutputDir == "" {
		t.Fatalf("expected benchmark.output_dir to be set")
	}
	if len(cfg.Stages) == 0 {
		t.Fatalf("expected stages to be non-empty")
	}
	// Check that data_schema is now in outputs
	if len(cfg.Stages) > 1 && len(cfg.Stages[1].Outputs) > 0 {
		output := cfg.Stages[1].Outputs[0]
		if output.Name == "" {
			t.Fatalf("expected output name to be set")
		}
		if output.DataSchema == nil {
			t.Fatalf("expected data_schema to be set in output")
		}
		if output.DataSchema.Format != "csv" {
			t.Fatalf("expected data_schema.format to be csv, got %s", output.DataSchema.Format)
		}
		if len(output.DataSchema.Columns) == 0 {
			t.Fatalf("expected data_schema.columns to be non-empty")
		}
	}
	// Check that plots reference outputs correctly
	if len(cfg.Plots) > 0 {
		plot := cfg.Plots[0]
		if plot.Source == "" {
			t.Fatalf("expected plot source to be set")
		}
	}
}

// TestLoadConfig_InvalidStage ensures we error when neither command nor script is given.
func TestLoadConfig_InvalidStage(t *testing.T) {
	cfgPath := filepath.Join("testdata", "2.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", cfgPath, err)
	}

	_, err = ParseYAML(data)
	if err == nil {
		t.Fatalf("expected error for invalid stage without command or script")
	}
	if !strings.Contains(err.Error(), "exactly one of command or script must be set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestLoadConfig_BadHealthCheck validates error messages for invalid healthcheck configuration.
func TestLoadConfig_BadHealthCheck(t *testing.T) {
	cfgPath := filepath.Join("testdata", "3.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", cfgPath, err)
	}

	_, err = ParseYAML(data)
	if err == nil {
		t.Fatalf("expected error for bad health check config")
	}
	// Expect mentions of type/timeout/retries
	msg := err.Error()
	if !strings.Contains(msg, "health_check.type") {
		t.Fatalf("expected error to reference health_check.type, got: %v", msg)
	}
	if !strings.Contains(msg, "timeout must be a positive duration") {
		t.Fatalf("expected timeout validation error, got: %v", msg)
	}
	if !strings.Contains(msg, "retries must be >= 0") {
		t.Fatalf("expected retries validation error, got: %v", msg)
	}
}

// TestTimestampFormatValid ensures timestamp format is accepted when valid.
func TestTimestampFormatValid(t *testing.T) {
	yaml := `
benchmark:
  name: ts-format-valid
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: gen
    host: local
    script: gen.sh
    outputs:
      - name: data
        remote_path: /tmp/data.csv
        data_schema:
          format: csv
          columns:
            - name: timestamp
              type: timestamp
              unit: s
              format: unix
            - name: value
              type: float
`
	cfg, err := ParseYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error parsing valid timestamp format: %v", err)
	}
	if len(cfg.Stages) == 0 || len(cfg.Stages[0].Outputs) == 0 || cfg.Stages[0].Outputs[0].DataSchema == nil {
		t.Fatalf("expected stage/output/schema to be present")
	}
	cols := cfg.Stages[0].Outputs[0].DataSchema.Columns
	if len(cols) == 0 {
		t.Fatalf("expected columns in data_schema")
	}
	var timestampCol *DataColumn
	for i := range cols {
		if cols[i].Name == "timestamp" {
			timestampCol = &cols[i]
			break
		}
	}
	if timestampCol == nil {
		t.Fatalf("expected timestamp column in data_schema")
	}
	if strings.ToLower(timestampCol.Format) != "unix" {
		t.Fatalf("expected timestamp format 'unix', got %q", timestampCol.Format)
	}
}

// TestTimestampFormatInvalid ensures invalid timestamp format triggers validation error.
func TestTimestampFormatInvalid(t *testing.T) {
	yaml := `
benchmark:
  name: ts-format-invalid
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: gen
    host: local
    script: gen.sh
    outputs:
      - name: data
        remote_path: /tmp/data.csv
        data_schema:
          format: csv
          columns:
            - name: timestamp
              type: timestamp
              unit: s
              format: not_a_real_format
            - name: value
              type: float
`
	_, err := ParseYAML([]byte(yaml))
	if err == nil {
		t.Fatalf("expected error for invalid timestamp format")
	}
	if !strings.Contains(err.Error(), "data_schema.columns[0].format must be one of") {
		t.Fatalf("unexpected error: %v", err)
	}
}
