//go:build unit

package config

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
	// Check outputs in test fixture
	if len(cfg.Stages) > 1 && len(cfg.Stages[1].Outputs) > 0 {
		output := cfg.Stages[1].Outputs[0]
		if output.Name == "" {
			t.Fatalf("expected output name to be set")
		}
		if output.RemotePath == "" {
			t.Fatalf("expected remote_path to be set")
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

func TestStageHostAndHostsConflict(t *testing.T) {
	yaml := `
benchmark:
  name: conflict
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: run
    host: local
    hosts: [local]
    command: echo hello
`
	_, err := ParseYAML([]byte(yaml))
	if err == nil {
		t.Fatalf("expected error for stage with both host and hosts")
	}
	if !strings.Contains(err.Error(), "cannot set both host and hosts") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStageDefaultHostLocal(t *testing.T) {
	yaml := `
benchmark:
  name: default-host
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: run
    command: echo hello
`
	cfg, err := ParseYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Stages[0].Host != "" || len(cfg.Stages[0].Hosts) != 0 {
		t.Fatalf("expected stage host fields to be empty")
	}
}

func TestStageNameUnique(t *testing.T) {
	yaml := `
benchmark:
  name: duplicate
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: run
    command: echo hello
  - name: run
    command: echo again
`
	_, err := ParseYAML([]byte(yaml))
	if err == nil {
		t.Fatalf("expected error for duplicate stage names")
	}
	if !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCasesAndExecuteOnlyForValidation(t *testing.T) {
	yaml := `
benchmark:
  name: cases
  output_dir: ./results
hosts:
  local: {}
cases:
  - name: a
    env:
      ENGINE: a
stages:
  - name: run-a
    command: echo hello
    execute_only_for: a
`
	cfg, err := ParseYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Stages[0].ExecuteOnlyFor != "a" {
		t.Fatalf("expected execute_only_for to be preserved, got %q", cfg.Stages[0].ExecuteOnlyFor)
	}
	if cfg.Cases[0].Env["ENGINE"] != "a" {
		t.Fatalf("expected case env to be preserved")
	}
}

func TestExecuteOnlyForUnknownCase(t *testing.T) {
	yaml := `
benchmark:
  name: cases
  output_dir: ./results
hosts:
  local: {}
cases:
  - name: a
stages:
  - name: run-b
    command: echo hello
    execute_only_for: b
`
	_, err := ParseYAML([]byte(yaml))
	if err == nil {
		t.Fatalf("expected error for unknown case")
	}
	if !strings.Contains(err.Error(), "execute_only_for references unknown case") {
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

func TestParseYAMLErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		contain string
	}{
		{
			name: "unknown stage host",
			yaml: `
benchmark:
  name: bad-host
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: run
    host: missing
    command: echo hello
`,
			contain: "references unknown host",
		},
		{
			name: "duplicate case names",
			yaml: `
benchmark:
  name: dup-cases
  output_dir: ./results
hosts:
  local: {}
cases:
  - name: a
  - name: a
stages:
  - name: run
    command: echo hello
`,
			contain: "cases[1].name duplicates",
		},
		{
			name: "duplicate output names",
			yaml: `
benchmark:
  name: dup-outputs
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: collect-a
    command: echo a
    outputs:
      - name: metrics
        remote_path: /tmp/a.csv
  - name: collect-b
    command: echo b
    outputs:
      - name: metrics
        remote_path: /tmp/b.csv
`,
			contain: "output name 'metrics' is not unique",
		},
		{
			name: "empty stage name",
			yaml: `
benchmark:
  name: empty-stage
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: "   "
    command: echo hello
`,
			contain: "stages[0].name must be set",
		},
		{
			name: "empty output name",
			yaml: `
benchmark:
  name: empty-output
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: collect
    command: echo hello
    outputs:
      - name: " "
        remote_path: /tmp/out.csv
`,
			contain: "outputs[0].name must be set",
		},
		{
			name: "execute_only_for without cases",
			yaml: `
benchmark:
  name: no-cases
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: run
    command: echo hello
    execute_only_for: a
`,
			contain: "execute_only_for requires cases",
		},
		{
			name: "duplicate stage hosts",
			yaml: `
benchmark:
  name: dup-hosts
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: run
    hosts: [local, local]
    command: echo hello
`,
			contain: "hosts contains duplicate host",
		},
		{
			name: "disallowed output local_path",
			yaml: `
benchmark:
  name: local-path
  output_dir: ./results
hosts:
  local: {}
stages:
  - name: collect
    command: echo hello
    outputs:
      - name: metrics
        remote_path: /tmp/metrics.csv
        local_path: ./metrics.csv
`,
			contain: "local_path is not allowed",
		},
		{
			name: "sync without remote",
			yaml: `
benchmark:
  name: sync
  output_dir: ./results
  sync: {}
hosts:
  local: {}
stages:
  - name: run
    command: echo hello
`,
			contain: "benchmark.sync.remote must be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseYAML([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected parse error")
			}
			if !strings.Contains(err.Error(), tt.contain) {
				t.Fatalf("expected error containing %q, got: %v", tt.contain, err)
			}
		})
	}
}
