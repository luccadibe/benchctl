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

func TestRuntimeCaseFilterDoesNotMutateBench(t *testing.T) {
	b := bench.New("cases",
		bench.WithResultsPath("./results"),
		bench.WithCases(
			bench.NewCase("a"),
			bench.NewCase("b"),
		),
		bench.WithStages(bench.Stage("run", bench.Command("echo run"))),
	)

	cfg := b.Config().Clone()
	if err := applyRuntimeCases(cfg, []string{"a"}); err != nil {
		t.Fatalf("apply case filter: %v", err)
	}
	if len(cfg.Cases) != 1 || cfg.Cases[0].Name != "a" {
		t.Fatalf("expected cloned config to filter to case a, got %#v", cfg.Cases)
	}
	if len(b.Config().Cases) != 2 {
		t.Fatalf("expected original bench to keep both cases, got %#v", b.Config().Cases)
	}
}

func TestRuntimeCaseFilterUnknownCase(t *testing.T) {
	cfg := bench.New("cases",
		bench.WithResultsPath("./results"),
		bench.WithCases(bench.NewCase("a")),
		bench.WithStages(bench.Stage("run", bench.Command("echo run"))),
	).Config()

	if err := applyRuntimeCases(cfg.Clone(), []string{"missing"}); err == nil {
		t.Fatal("expected error for unknown case")
	}
}

func TestRuntimeCaseFilterRequiresCases(t *testing.T) {
	cfg := bench.New("no-cases",
		bench.WithResultsPath("./results"),
		bench.WithStages(bench.Stage("run", bench.Command("echo run"))),
	).Config()

	if err := applyRuntimeCases(cfg.Clone(), []string{"a"}); err == nil {
		t.Fatal("expected error when config has no cases")
	}
}

func TestRuntimeCaseFilterPreservesConfigOrder(t *testing.T) {
	cfg := bench.New("cases",
		bench.WithResultsPath("./results"),
		bench.WithCases(
			bench.NewCase("a"),
			bench.NewCase("b"),
			bench.NewCase("c"),
		),
		bench.WithStages(bench.Stage("run", bench.Command("echo run"))),
	).Config().Clone()

	if err := applyRuntimeCases(cfg, []string{"b", "a"}); err != nil {
		t.Fatalf("apply case filter: %v", err)
	}
	if len(cfg.Cases) != 2 {
		t.Fatalf("expected 2 filtered cases, got %d", len(cfg.Cases))
	}
	if cfg.Cases[0].Name != "a" || cfg.Cases[1].Name != "b" {
		t.Fatalf("expected config order [a b], got %#v", cfg.Cases)
	}
}
