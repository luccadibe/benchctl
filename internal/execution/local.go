package execution

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

// Local execution client for running commands locally
type localClient struct{}

// NewLocalClient creates a new local execution client
func NewLocalClient() ExecutionClient {
	return &localClient{}
}

// RunCommand executes a command locally using the shell
func (c *localClient) RunCommand(ctx context.Context, req CommandRequest) (CommandResult, error) {
	if strings.TrimSpace(req.Command) == "" {
		return CommandResult{ExitCode: -1}, errors.New("empty command")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", req.Command)
	if req.Stdin != nil {
		cmd.Stdin = req.Stdin
	}

	if req.UsePTY {
		return runLocalWithPTY(cmd, req)
	}

	return runLocalPiped(cmd, req)
}

func runLocalPiped(cmd *exec.Cmd, req CommandRequest) (CommandResult, error) {
	var capture *captureBuffer
	if !req.DisableCapture {
		capture = newCaptureBuffer()
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return CommandResult{ExitCode: -1}, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return CommandResult{ExitCode: -1}, err
	}

	if err := cmd.Start(); err != nil {
		return CommandResult{ExitCode: -1}, err
	}

	var wg sync.WaitGroup
	copyStream := func(r io.Reader, additional io.Writer) {
		defer wg.Done()
		_, _ = io.Copy(additional, r)
	}

	stdoutDest := multiWriterFiltered(req.Stdout, capture)
	stderrDest := multiWriterFiltered(req.Stderr, capture)

	wg.Add(2)
	go copyStream(stdoutPipe, stdoutDest)
	go copyStream(stderrPipe, stderrDest)

	err = cmd.Wait()
	wg.Wait()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := CommandResult{ExitCode: exitCode}
	if capture != nil {
		result.Output = capture.String()
	}
	return result, err
}

func runLocalWithPTY(cmd *exec.Cmd, req CommandRequest) (CommandResult, error) {
	var capture *captureBuffer
	if !req.DisableCapture {
		capture = newCaptureBuffer()
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return CommandResult{ExitCode: -1}, err
	}
	defer ptmx.Close()
	if os.Stdout != nil {
		_ = pty.InheritSize(os.Stdout, ptmx)
	}

	// If stdin provided, stream it into the PTY.
	if req.Stdin != nil {
		go func() {
			_, _ = io.Copy(ptmx, req.Stdin)
		}()
	}

	combinedDest := multiWriterFiltered(req.Stdout, req.Stderr, capture)
	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(combinedDest, ptmx)
		close(copyDone)
	}()

	err = cmd.Wait()
	<-copyDone

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := CommandResult{ExitCode: exitCode}
	if capture != nil {
		result.Output = capture.String()
	}
	return result, err
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
