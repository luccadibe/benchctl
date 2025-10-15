package execution

import (
	"context"
	"io"
	"time"
)

// CommandRequest defines how a command should be executed by an ExecutionClient.
type CommandRequest struct {
	Command        string    // shell or binary invocation to run
	Stdout         io.Writer // optional live stdout
	Stderr         io.Writer // optional live stderr
	Stdin          io.Reader // optional stdin source
	UsePTY         bool      // request a PTY/TTY when supported
	DisableCapture bool      // when true, do not retain combined output
}

// CommandResult describes the outcome of a command invocation.
type CommandResult struct {
	Output   string // combined stdout+stderr unless capture disabled
	ExitCode int
}

// ExecutionClient defines the interface for executing commands on hosts.
type ExecutionClient interface {
	RunCommand(ctx context.Context, req CommandRequest) (CommandResult, error)
	CheckPort(ctx context.Context, port string, timeout time.Duration) (bool, error)
	Scp(ctx context.Context, remotePath, localPath string) error
	Upload(ctx context.Context, localPath, remotePath string) error
	Close() error
}
