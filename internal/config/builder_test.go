//go:build unit

package config

import "testing"

func TestBuilderCreatesValidConfig(t *testing.T) {
	cfg := New("builder", "./results",
		WithHost("remote", SSHHost("10.0.0.1", "bench", "~/.ssh/id_rsa")),
		WithStage(NewStage("collect",
			OnHost("remote"),
			RunCommand("./collect.sh"),
			WithOutput(NewOutput("metrics", "/tmp/metrics.csv", NewCSVSchema(
				Column("timestamp", DataTypeTimestamp, Unit("s"), TimeFormat("unix")),
				Column("latency_ms", DataTypeFloat, Unit("ms")),
			))),
		)),
	)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
	if cfg.Stages[0].Outputs[0].DataSchema.Columns[1].Unit != "ms" {
		t.Fatalf("expected unit helper to set ms")
	}
}
