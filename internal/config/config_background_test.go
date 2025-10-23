package config

import (
	"strings"
	"testing"
)

func TestBackgroundStageCannotAppendMetadata(t *testing.T) {
	yaml := `
benchmark:
  name: test
  output_dir: ./tmp
hosts:
  local: {}
stages:
  - name: monitor
    host: local
    command: echo ok
    background: true
    append_metadata: true
`

	_, err := ParseYAML([]byte(yaml))
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "background=true") {
		t.Fatalf("expected error to mention background=true constraint, got %v", err)
	}
}
