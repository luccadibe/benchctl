package run

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/luccadibe/benchctl/internal"
	"github.com/luccadibe/benchctl/pkg/bench"
)

type ComparisonResult = internal.ComparisonResult

// Inspect returns the human-readable inspection for a run directory.
func Inspect(runDir string, verbose bool) string {
	return internal.InspectRun(runDir, verbose)
}

// Annotate adds metadata to a completed run directory.
func Annotate(runDir string, metadata map[string]string) error {
	return internal.AddMetadata(runDir, metadata)
}

// LoadMetadata loads metadata.json from a run directory.
func LoadMetadata(runDir string) (*RunMetadata, error) {
	return internal.LoadRunMetadata(filepath.Join(runDir, "metadata.json"))
}

// Compare compares custom metadata for two loaded runs.
func Compare(first, second *RunMetadata) ([]ComparisonResult, error) {
	return internal.CompareRunMetadata(first, second)
}

// FormatComparison renders comparison results for CLI-style output.
func FormatComparison(results []ComparisonResult) string {
	return internal.PrintComparisonResults(results)
}

// SyncPush syncs benchmark results according to benchmark.sync.
func SyncPush(ctx context.Context, b *bench.Bench) error {
	if b == nil {
		return fmt.Errorf("benchmark is nil")
	}
	return internal.SyncResults(ctx, b.Config())
}
