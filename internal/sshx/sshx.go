package sshx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
}

func Run(ctx context.Context, cfg Config, command string, stdout, stderr io.Writer) (int, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	clientConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{ssh.Password(cfg.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	dialer := &net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), clientConfig)
	if err != nil {
		return -1, err
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return -1, err
	}
	defer session.Close()
	session.Stdout = stdout
	session.Stderr = stderr

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		return -1, ctx.Err()
	case err := <-errCh:
		if err == nil {
			return 0, nil
		}
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitStatus(), nil
		}
		return -1, err
	}
}
