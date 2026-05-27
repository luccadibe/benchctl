//go:build integration

package test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/luccadibe/benchctl/internal/config"
	"github.com/luccadibe/benchctl/internal/execution"
)

const (
	testKeyPath    = "./testdata/ssh/test_key"
	testUsername   = "testuser"
	testHost1      = "localhost"
	testPort1      = 2222
	testHost2      = "localhost"
	testPort2      = 2223
	commandTimeout = 10 * time.Second
)

var hosts = []config.Host{
	{
		IP:       testHost1,
		Port:     testPort1,
		Username: testUsername,
		KeyFile:  testKeyPath,
	},
	{
		IP:       testHost2,
		Port:     testPort2,
		Username: testUsername,
		KeyFile:  testKeyPath,
	},
}

func TestSSHClientConnectionWithInvalidKey(t *testing.T) {
	host := hosts[0]
	host.KeyFile = "./testdata/ssh/nonexistent_key"

	_, err := execution.NewSSHClient(host)
	if err == nil {
		t.Fatal("expected error for invalid key file, got nil")
	}

	if !strings.Contains(err.Error(), "error creating ssh client") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSSHClientRunCommand(t *testing.T) {
	host := hosts[0]

	client, err := execution.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	res, err := client.RunCommand(ctx, execution.CommandRequest{Command: "echo 'hello world'"})
	if err != nil {
		t.Fatalf("failed to run command: %v", err)
	}

	if !strings.Contains(res.Output, "hello world") {
		t.Errorf("expected output to contain %q, got %q", "hello world", res.Output)
	}
}

func TestSSHClientContextCancellation(t *testing.T) {
	host := hosts[0]

	client, err := execution.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.RunCommand(ctx, execution.CommandRequest{Command: "sleep 10"})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}

	if !strings.Contains(err.Error(), "context done") && !strings.Contains(err.Error(), "SIGINT") {
		t.Errorf("unexpected error message for cancelled context: %v", err)
	}
}

func TestSSHClientMultipleHosts(t *testing.T) {
	for _, host := range hosts {
		t.Run(strconv.Itoa(host.Port), func(t *testing.T) {
			client, err := execution.NewSSHClient(host)
			if err != nil {
				t.Fatalf("failed to create SSH client: %v", err)
			}
			defer client.Close()

			ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
			defer cancel()

			res, err := client.RunCommand(ctx, execution.CommandRequest{Command: "whoami"})
			if err != nil {
				t.Fatalf("failed to run command: %v", err)
			}

			if !strings.Contains(res.Output, testUsername) {
				t.Errorf("expected output to contain %q, got %q", testUsername, res.Output)
			}
		})
	}
}

func TestSSHClientCommandWithStderr(t *testing.T) {
	host := hosts[0]

	client, err := execution.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	res, err := client.RunCommand(ctx, execution.CommandRequest{Command: "echo 'error message' >&2"})
	if err != nil {
		t.Fatalf("failed to run command: %v", err)
	}

	if !strings.Contains(res.Output, "error message") {
		t.Errorf("expected output to contain stderr message, got %q", res.Output)
	}
}

func TestSSHClientCommandFailure(t *testing.T) {
	host := hosts[0]

	client, err := execution.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	_, err = client.RunCommand(ctx, execution.CommandRequest{Command: "exit 1"})
	if err == nil {
		t.Fatal("expected error for failed command, got nil")
	}
}

func TestSSHClientCloseAndReconnect(t *testing.T) {
	host := hosts[0]

	client1, err := execution.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create first SSH client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	res1, err := client1.RunCommand(ctx, execution.CommandRequest{Command: "echo 'first'"})
	if err != nil {
		t.Fatalf("failed to run command on first client: %v", err)
	}

	if !strings.Contains(res1.Output, "first") {
		t.Errorf("unexpected output from first client: %q", res1.Output)
	}

	if err := client1.Close(); err != nil {
		t.Fatalf("failed to close first client: %v", err)
	}

	client2, err := execution.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create second SSH client: %v", err)
	}
	defer client2.Close()

	ctx2, cancel2 := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel2()

	res2, err := client2.RunCommand(ctx2, execution.CommandRequest{Command: "echo 'second'"})
	if err != nil {
		t.Fatalf("failed to run command on second client: %v", err)
	}

	if !strings.Contains(res2.Output, "second") {
		t.Errorf("unexpected output from second client: %q", res2.Output)
	}
}
