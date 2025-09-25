package internal

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

const (
	DEFAULT_SSH_PORT = 22
)

// all things SSH here

type sshClient struct {
	client *ssh.Client
	host   *Host
}

type commandOutput struct {
	output []byte
	err    error
}

func NewSSHClient(host *Host) (*sshClient, error) {
	client, err := connect(host)
	if err != nil {
		return nil, errors.New("error creating ssh client: " + err.Error())
	}
	return &sshClient{client: client, host: host}, nil
}

func (c *sshClient) Close() error {
	return c.client.Close()
}

// RunCommand runs a command on the remote host and returns the output. If the context is done, it will send a SIGINT signal to the remote process.
func (c *sshClient) RunCommand(ctx context.Context, command string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", errors.New("error creating new session: " + err.Error())
	}
	defer session.Close()

	outChan := make(chan commandOutput)

	go func() {
		output, err := session.CombinedOutput(command)
		if err != nil {
			outChan <- commandOutput{output: nil, err: errors.New("error running command: " + err.Error())}
		}
		outChan <- commandOutput{output: output, err: nil}
	}()

	select {
	case <-ctx.Done():
		err = session.Signal(ssh.SIGINT)
		if err != nil {
			return "", errors.New("error sending SIGINT signal: " + err.Error())
		}
		return "", errors.New("context done")
	case out := <-outChan:
		return string(out.output), out.err
	}

}

// TODO: implement Password auth, for now only key auth is supported
func connect(host *Host) (*ssh.Client, error) {
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
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host.IP, DEFAULT_SSH_PORT), sshConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}
