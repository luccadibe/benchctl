//go:build unit

package internal

import (
	"reflect"
	"testing"

	"github.com/luccadibe/benchctl/internal/config"
)

func TestRcloneSyncArgs(t *testing.T) {
	cfg := config.New("sync", "./results", config.WithSync(config.SyncConfig{
		Remote: "s3:benchctl/results",
		Args:   []string{"--checksum"},
	}))
	args, err := rcloneSyncArgs(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"sync", "./results", "s3:benchctl/results", "--checksum"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}
