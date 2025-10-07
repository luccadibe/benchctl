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
