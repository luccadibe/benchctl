//go:build unit

package run

import (
	"testing"

	"github.com/luccadibe/benchctl/pkg/bench"
)

func TestRuntimeSkipDoesNotMutateBench(t *testing.T) {
	b := bench.New("skip",
		bench.WithResultsPath("./results"),
		bench.WithStages(bench.Stage("optional", bench.Command("echo optional"))),
	)

	cfg := b.Config().Clone()
	if err := applyRuntimeSkip(cfg, []string{"optional"}); err != nil {
		t.Fatalf("apply skip: %v", err)
	}
	if !cfg.Stages[0].Skip {
		t.Fatalf("expected cloned config stage to be skipped")
	}
	if b.Config().Stages[0].Skip {
		t.Fatalf("expected original bench stage to remain unskipped")
	}
}

func TestRuntimeSkipUnknownStage(t *testing.T) {
	cfg := bench.New("skip",
		bench.WithResultsPath("./results"),
		bench.WithStages(bench.Stage("setup", bench.Command("echo setup"))),
	).Config()

	if err := applyRuntimeSkip(cfg.Clone(), []string{"missing"}); err == nil {
		t.Fatal("expected error for unknown stage")
	}
}
