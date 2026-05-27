//go:build unit

package config

import "testing"

func TestBuilderCreatesValidConfig(t *testing.T) {
	cfg := New("builder", "./results",
		WithHost("remote", SSHHost("10.0.0.1", "bench", "~/.ssh/id_rsa")),
		WithCase("rocksdb", map[string]string{"ENGINE": "rocksdb"}),
		WithStage(NewStage("collect",
			OnHost("remote"),
			ExecuteOnlyFor("rocksdb"),
			RunCommand("./collect.sh"),
			WithOutput(NewOutput("metrics", "/tmp/metrics.csv")),
		)),
	)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
	if cfg.Stages[0].Outputs[0].RemotePath != "/tmp/metrics.csv" {
		t.Fatalf("expected remote path to be set")
	}
}
