package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/goforj/godump"
	"golang.org/x/crypto/ssh"
)

const (
	DEFAULT_SSH_PORT = 22
)

// all things SSH here

type sshClient struct {
	client *ssh.Client
	host   Host
}

type commandResult struct {
	output   []byte
	err      error
	exitCode int
}

func NewSSHClient(host Host) (ExecutionClient, error) {
	client, err := connect(host)
	if err != nil {
		return nil, errors.New("error creating ssh client: " + err.Error())
	}
	return &sshClient{client: client, host: host}, nil
}

func (c *sshClient) Close() error {
	return c.client.Close()
}

// RunCommand runs a command on the remote host and returns the output and exit code. If the context is done, it will send a SIGINT signal to the remote process.
func (c *sshClient) RunCommand(ctx context.Context, command string) (string, int, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", -1, errors.New("error creating new session: " + err.Error())
	}
	defer session.Close()

	resultChan := make(chan commandResult)

	go func() {
		output, err := session.CombinedOutput(command)
		exitCode := 0
		if err != nil {
			// Try to extract exit code from the error
			if exitError, ok := err.(*ssh.ExitError); ok {
				exitCode = exitError.ExitStatus()
			} else {
				exitCode = -1
			}
		}
		resultChan <- commandResult{output: output, err: err, exitCode: exitCode}
	}()

	select {
	case <-ctx.Done():
		err = session.Signal(ssh.SIGINT)
		if err != nil {
			return "", -1, errors.New("error sending SIGINT signal: " + err.Error())
		}
		return "", -1, errors.New("context done")
	case result := <-resultChan:
		return string(result.output), result.exitCode, result.err
	}
}

// CommandExists checks if a command exists on the remote host.
func (c *sshClient) CommandExists(ctx context.Context, cmd string) (bool, error) {
	checkCmd := fmt.Sprintf("which %s", cmd)
	_, exitCode, err := c.RunCommand(ctx, checkCmd)
	if err != nil {
		return false, err
	}
	return exitCode == 0, nil
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
	output, exitCode, err := c.RunCommand(subCtx, command)
	godump.Dump(output, err)
	if err != nil {
		return false, err
	}
	return exitCode == 0, nil
}

// TODO: implement Password auth, for now only key auth is supported
func connect(host Host) (*ssh.Client, error) {
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
