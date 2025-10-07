//go:build integration

package test

import (
	"context"
	"strings"
	"testing"
	"time"

	"benchctl/internal"
)

const (
	testKeyPath    = "./testdata/ssh/test_key"
	testUsername   = "testuser"
	testHost1      = "localhost"
	testPort1      = 2222
	testHost2      = "localhost"
	testPort2      = 2223
	testHost3      = "localhost"
	testPort3      = 2224
	commandTimeout = 10 * time.Second
)

var hosts = []internal.Host{
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

func TestSSHClientConnection(t *testing.T) {
	host := hosts[0]

	client, err := internal.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestSSHClientConnectionWithInvalidKey(t *testing.T) {
	host := hosts[0]
	host.KeyFile = "./testdata/ssh/nonexistent_key"

	_, err := internal.NewSSHClient(host)
	if err == nil {
		t.Fatal("expected error for invalid key file, got nil")
	}

	if !strings.Contains(err.Error(), "error creating ssh client") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSSHClientRunCommand(t *testing.T) {
	host := hosts[0]

	client, err := internal.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	output, _, err := client.RunCommand(ctx, "echo 'hello world'")
	if err != nil {
		t.Fatalf("failed to run command: %v", err)
	}

	expected := "hello world"
	if !strings.Contains(output, expected) {
		t.Errorf("expected output to contain %q, got %q", expected, output)
	}
}

func TestSSHClientRunMultipleCommands(t *testing.T) {
	host := hosts[0]

	client, err := internal.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	tests := []struct {
		name     string
		command  string
		contains string
	}{
		{
			name:     "whoami",
			command:  "whoami",
			contains: testUsername,
		},
		{
			name:     "pwd",
			command:  "pwd",
			contains: "/",
		},
		{
			name:     "date",
			command:  "date +%Y",
			contains: "20",
		},
		{
			name:     "hostname",
			command:  "hostname",
			contains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
			defer cancel()

			output, _, err := client.RunCommand(ctx, tt.command)
			if err != nil {
				t.Fatalf("failed to run command %q: %v", tt.command, err)
			}

			if tt.contains != "" && !strings.Contains(output, tt.contains) {
				t.Errorf("expected output to contain %q, got %q", tt.contains, output)
			}
		})
	}
}

func TestSSHClientContextCancellation(t *testing.T) {
	host := hosts[0]

	client, err := internal.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run a long-running command that will be cancelled
	_, _, err = client.RunCommand(ctx, "sleep 10")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}

	if !strings.Contains(err.Error(), "context done") && !strings.Contains(err.Error(), "SIGINT") {
		t.Errorf("unexpected error message for cancelled context: %v", err)
	}
}

func TestSSHClientMultipleHosts(t *testing.T) {
	hosts := hosts

	for i, host := range hosts {
		t.Run(host.IP, func(t *testing.T) {
			client, err := internal.NewSSHClient(host)
			if err != nil {
				t.Fatalf("failed to create SSH client for host %d: %v", i+1, err)
			}
			defer client.Close()

			ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
			defer cancel()

			output, _, err := client.RunCommand(ctx, "whoami")
			if err != nil {
				t.Fatalf("failed to run command on host %d: %v", i+1, err)
			}

			if !strings.Contains(output, testUsername) {
				t.Errorf("expected output to contain %q, got %q", testUsername, output)
			}
		})
	}
}

func TestSSHClientCommandWithStderr(t *testing.T) {
	host := hosts[0]

	client, err := internal.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	// Command that writes to stderr
	output, _, err := client.RunCommand(ctx, "echo 'error message' >&2")
	if err != nil {
		t.Fatalf("failed to run command: %v", err)
	}

	if !strings.Contains(output, "error message") {
		t.Errorf("expected output to contain stderr message, got %q", output)
	}
}

func TestSSHClientCommandFailure(t *testing.T) {
	host := hosts[0]

	client, err := internal.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	// Command that exits with non-zero status
	_, _, err = client.RunCommand(ctx, "exit 1")
	if err == nil {
		t.Fatal("expected error for failed command, got nil")
	}
}

func TestSSHClientCloseAndReconnect(t *testing.T) {
	host := hosts[0]

	// First connection
	client1, err := internal.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create first SSH client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	output1, _, err := client1.RunCommand(ctx, "echo 'first'")
	if err != nil {
		t.Fatalf("failed to run command on first client: %v", err)
	}

	if !strings.Contains(output1, "first") {
		t.Errorf("unexpected output from first client: %q", output1)
	}

	// Close first connection
	if err := client1.Close(); err != nil {
		t.Fatalf("failed to close first client: %v", err)
	}

	// Second connection
	client2, err := internal.NewSSHClient(host)
	if err != nil {
		t.Fatalf("failed to create second SSH client: %v", err)
	}
	defer client2.Close()

	ctx2, cancel2 := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel2()

	output2, _, err := client2.RunCommand(ctx2, "echo 'second'")
	if err != nil {
		t.Fatalf("failed to run command on second client: %v", err)
	}

	if !strings.Contains(output2, "second") {
		t.Errorf("unexpected output from second client: %q", output2)
	}
}
