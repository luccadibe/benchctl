package internal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Local execution client for running commands locally
type localClient struct{}

// NewLocalClient creates a new local execution client
func NewLocalClient() ExecutionClient {
	return &localClient{}
}

// RunCommand executes a command locally using the shell
func (c *localClient) RunCommand(ctx context.Context, command string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return string(output), exitCode, err
}

// CheckPort checks if a port is listening locally
func (c *localClient) CheckPort(ctx context.Context, port string, timeout time.Duration) (bool, error) {
	// For local execution, use netstat or ss to check if port is listening
	cmd := exec.CommandContext(ctx, "sh", "-c", "netstat -tlnp | grep ':"+port+" ' || ss -tlnp | grep ':"+port+" '")
	err := cmd.Run()
	return err == nil, nil
}

// Scp copies a file locally (just uses cp)
func (c *localClient) Scp(ctx context.Context, remotePath, localPath string) error {
	// Create the destination directory if it doesn't exist
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// For local execution, just copy the file
	cmd := exec.CommandContext(ctx, "cp", remotePath, localPath)
	return cmd.Run()
}

// Upload copies a file locally (local to local)
func (c *localClient) Upload(ctx context.Context, localPath, remotePath string) error {
	dir := filepath.Dir(remotePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "cp", localPath, remotePath)
	return cmd.Run()
}

// Close closes the local client (no-op for local execution)
func (c *localClient) Close() error {
	// Nothing to close for local execution
	return nil
}
