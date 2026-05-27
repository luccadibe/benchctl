//go:build unit

package bench

import (
	"testing"
	"time"
)

func TestBuilderCreatesValidBench(t *testing.T) {
	b := New("public",
		WithResultsPath("./results"),
		WithGit(RequireClean(false)),
		WithHost("local", Local()),
		WithStages(
			Stage("setup",
				Host("local"),
				Command("echo hello"),
			),
			BackgroundStage("monitor",
				Script("./monitor.sh"),
				Output("metrics", RemotePath("/tmp/metrics.csv")),
			),
			Stage("load",
				Script("./load.sh"),
				PortCheck("8080", Timeout(30*time.Second), Retries(3)),
			),
		),
	)

	if err := b.Validate(); err != nil {
		t.Fatalf("expected valid bench: %v", err)
	}

	cfg := b.Config()
	if cfg.Benchmark.OutputDir != "./results" {
		t.Fatalf("expected output dir to be set")
	}
	if !cfg.Stages[1].Background {
		t.Fatalf("expected background stage")
	}
	if cfg.Stages[1].Outputs[0].RemotePath != "/tmp/metrics.csv" {
		t.Fatalf("expected remote path to be set")
	}
}

func TestFromYAML(t *testing.T) {
	b, err := FromYAML([]byte(`
benchmark:
  name: yaml
  output_dir: ./results
stages:
  - name: hello
    command: echo hello
`))
	if err != nil {
		t.Fatalf("from yaml: %v", err)
	}
	if b.Config().Benchmark.Name != "yaml" {
		t.Fatalf("expected yaml benchmark name")
	}
}
