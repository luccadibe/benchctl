package execution

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/goforj/godump"
	"github.com/luccadibe/benchctl/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

const (
	DEFAULT_SSH_PORT = 22
)

// all things SSH here

type sshClient struct {
	client *ssh.Client
	host   config.Host
}

func NewSSHClient(host config.Host) (ExecutionClient, error) {
	client, err := connect(host)
	if err != nil {
		return nil, errors.New("error creating ssh client: " + err.Error())
	}
	return &sshClient{client: client, host: host}, nil
}

func (c *sshClient) Close() error {
	return c.client.Close()
}

// RunCommand runs a command on the remote host and returns the output and exit code.
// If the context is done, it will send a SIGINT signal to the remote process.
func (c *sshClient) RunCommand(ctx context.Context, req CommandRequest) (CommandResult, error) {
	if strings.TrimSpace(req.Command) == "" {
		return CommandResult{ExitCode: -1}, errors.New("empty command")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return CommandResult{ExitCode: -1}, errors.New("error creating new session: " + err.Error())
	}
	defer session.Close()

	streaming := req.UsePTY || req.Stdout != nil || req.Stderr != nil || req.Stdin != nil
	if streaming {
		return c.runStreamingCommand(ctx, session, req)
	}
	return c.runCombinedCommand(ctx, session, req)
}

func (c *sshClient) runCombinedCommand(ctx context.Context, session *ssh.Session, req CommandRequest) (CommandResult, error) {
	resultChan := make(chan struct {
		output   []byte
		exitCode int
		err      error
	})

	go func() {
		output, err := session.CombinedOutput(req.Command)
		exitCode := 0
		if err != nil {
			if exitError, ok := err.(*ssh.ExitError); ok {
				exitCode = exitError.ExitStatus()
			} else {
				exitCode = -1
			}
		}
		resultChan <- struct {
			output   []byte
			exitCode int
			err      error
		}{output: output, exitCode: exitCode, err: err}
	}()

	select {
	case <-ctx.Done():
		if err := session.Signal(ssh.SIGINT); err != nil {
			return CommandResult{ExitCode: -1}, errors.New("error sending SIGINT signal: " + err.Error())
		}
		return CommandResult{ExitCode: -1}, errors.New("context done")
	case res := <-resultChan:
		result := CommandResult{ExitCode: res.exitCode}
		if !req.DisableCapture {
			result.Output = string(res.output)
		}
		return result, res.err
	}
}

// runStreamingCommand runs a command on the remote host and streams the output to the stdout and stderr.
func (c *sshClient) runStreamingCommand(ctx context.Context, session *ssh.Session, req CommandRequest) (CommandResult, error) {
	if req.Stdin != nil {
		session.Stdin = req.Stdin
	}

	if req.UsePTY {
		modes := ssh.TerminalModes{
			ssh.ECHO:          1,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}
		width, height := termSize()
		if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
			return CommandResult{ExitCode: -1}, errors.New("error requesting PTY: " + err.Error())
		}
	}

	var capture *captureBuffer
	if !req.DisableCapture {
		capture = newCaptureBuffer()
	}

	var stdoutPipe io.Reader
	var stderrPipe io.Reader
	var err error

	stdoutPipe, err = session.StdoutPipe()
	if err != nil {
		return CommandResult{ExitCode: -1}, err
	}

	if !req.UsePTY {
		stderrPipe, err = session.StderrPipe()
		if err != nil {
			return CommandResult{ExitCode: -1}, err
		}
	}

	if err := session.Start(req.Command); err != nil {
		return CommandResult{ExitCode: -1}, err
	}

	stdoutDest := multiWriterFiltered(req.Stdout, capture)
	stderrDest := multiWriterFiltered(req.Stderr, capture)
	combinedDest := stdoutDest
	if req.UsePTY {
		combinedDest = multiWriterFiltered(req.Stdout, req.Stderr, capture)
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = io.Copy(combinedDest, stdoutPipe)
	})

	if !req.UsePTY {
		wg.Go(func() {
			_, _ = io.Copy(stderrDest, stderrPipe)
		})
	}

	resultChan := make(chan struct {
		exitCode int
		err      error
	}, 1)

	go func() {
		err := session.Wait()
		exitCode := 0
		if err != nil {
			if exitError, ok := err.(*ssh.ExitError); ok {
				exitCode = exitError.ExitStatus()
			} else {
				exitCode = -1
			}
		}
		resultChan <- struct {
			exitCode int
			err      error
		}{exitCode: exitCode, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGINT)
		wg.Wait()
		return CommandResult{ExitCode: -1}, errors.New("context done")
	case res := <-resultChan:
		wg.Wait()
		result := CommandResult{ExitCode: res.exitCode}
		if capture != nil {
			result.Output = capture.String()
		}
		return result, res.err
	}
}

// CommandExists checks if a command exists on the remote host.
func (c *sshClient) CommandExists(ctx context.Context, cmd string) (bool, error) {
	checkCmd := fmt.Sprintf("which %s", cmd)
	res, err := c.RunCommand(ctx, CommandRequest{Command: checkCmd})
	if err != nil {
		return false, err
	}
	return res.ExitCode == 0, nil
}

// Utility function to check if a port is listening (using nc -z, requires nc to be installed on the remote host)
func (c *sshClient) CheckPort(ctx context.Context, port string, timeout time.Duration) (bool, error) {
	// Check if nc is installed
	exists, err := c.CommandExists(ctx, "nc")
	if err != nil {
		return false, err
	}
	if !exists {
		return false, errors.New("nc (netcat) is not installed on the remote host")
	}

	command := fmt.Sprintf("nc -z localhost %s", port)
	subCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := c.RunCommand(subCtx, CommandRequest{Command: command})
	godump.Dump(result.Output, err)
	if err != nil {
		return false, err
	}
	return result.ExitCode == 0, nil
}

// TODO: implement Password auth, for now only key auth is supported
func connect(host config.Host) (*ssh.Client, error) {
	var key ssh.Signer
	var err error
	keyFile, err := os.ReadFile(host.KeyFile)
	if err != nil {
		return nil, err
	}

	if host.KeyPassword != "" {
		key, err = ssh.ParsePrivateKeyWithPassphrase(keyFile, []byte(host.KeyPassword))

	} else {
		key, err = ssh.ParsePrivateKey(keyFile)

	}
	if err != nil {
		return nil, errors.New("error reading key file: " + err.Error())
	}

	sshConfig := &ssh.ClientConfig{
		User: host.Username,
		Auth: []ssh.AuthMethod{
			// ssh.Password(host.Password),
			ssh.PublicKeys(key),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	port := host.Port
	if port == 0 {
		port = DEFAULT_SSH_PORT
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host.IP, port), sshConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Scp copies a file from the remote host to the local host.
func (c *sshClient) Scp(ctx context.Context, remotePath string, localPath string) error {
	client, err := scp.NewClientBySSH(c.client)
	if err != nil {
		return errors.New("error creating scp client: " + err.Error())
	}

	file, err := os.Create(localPath)
	if err != nil {
		return errors.New("error creating local file: " + err.Error())
	}
	defer file.Close()

	err = client.CopyFromRemote(ctx, file, remotePath)
	if err != nil {
		return errors.New("error copying file: " + err.Error())
	}
	return nil
}

// Upload copies a local file to the remote host
func (c *sshClient) Upload(ctx context.Context, localPath, remotePath string) error {
	client, err := scp.NewClientBySSH(c.client)
	if err != nil {
		return errors.New("error creating scp client: " + err.Error())
	}
	file, err := os.Open(localPath)
	if err != nil {
		return errors.New("error opening local file: " + err.Error())
	}
	defer file.Close()
	// Mode 0755 for scripts
	err = client.CopyFile(ctx, file, remotePath, "0755")
	if err != nil {
		return errors.New("error uploading file: " + err.Error())
	}
	return nil
}

// termSize returns the terminal size
func termSize() (width, height int) {
	width, height = 80, 40
	if os.Stdout != nil {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
				return w, h
			}
		}
	}
	return width, height
}
