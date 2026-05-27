//go:build unit

package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAddMetadataInitializesCustom(t *testing.T) {
	runDir := t.TempDir()
	metadata := RunMetadata{
		RunID:         "1",
		BenchmarkName: "annotate",
		StartTime:     time.Now(),
		EndTime:       time.Now(),
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "metadata.json"), b, 0644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	if err := AddMetadata(runDir, map[string]string{"latency_p95_ms": "12.3"}); err != nil {
		t.Fatalf("add metadata: %v", err)
	}

	updated, err := LoadRunMetadata(filepath.Join(runDir, "metadata.json"))
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if updated.Custom["latency_p95_ms"] != "12.3" {
		t.Fatalf("expected annotation to be persisted")
	}
}
