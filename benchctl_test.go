//go:build unit

package benchctl

import "testing"

func TestPublicBuilderAPI(t *testing.T) {
	cfg := NewConfig("public", "./results",
		WithStage(NewStage("hello", RunCommand("echo hello"))),
	)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
}
