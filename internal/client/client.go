package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"shux/internal/sshkey"
)

type AttachOptions struct {
	Bash bool
}

func clientTerm() string {
	term := os.Getenv("TERM")
	if term == "" {
		return "xterm-256color"
	}
	return term
}

func AttachOrSpawn(ctx context.Context, addr string) error {
	return AttachOrSpawnWithOptions(ctx, addr, AttachOptions{})
}

func AttachOrSpawnWithOptions(ctx context.Context, addr string, opts AttachOptions) error {
	available, err := ServerAvailable(ctx, addr)
	if err != nil {
		return err
	}
	if available {
		// Attach never mutates daemon startup policy. In particular, --bash only
		// affects a newly spawned daemon child; existing daemons keep their shell.
		return Attach(ctx, addr)
	}
	if err := spawnDetached(opts); err != nil {
		return err
	}
	if err := WaitReady(ctx, addr, 2*time.Second); err != nil {
		return err
	}
	return Attach(ctx, addr)
}

func Attach(ctx context.Context, addr string) error {
	sshClient, err := dialTrusted(ctx, addr)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	sess, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("client: new ssh session: %w", err)
	}
	defer sess.Close()

	stdinFD := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(stdinFD)
	if err != nil {
		return fmt.Errorf("client: raw terminal: %w", err)
	}
	defer term.Restore(stdinFD, oldState)

	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return fmt.Errorf("client: terminal size: %w", err)
	}
	if err := sess.RequestPty(clientTerm(), height, width, ssh.TerminalModes{}); err != nil {
		return fmt.Errorf("client: request pty: %w", err)
	}

	sess.Stdin = os.Stdin
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr
	if err := sess.Shell(); err != nil {
		return fmt.Errorf("client: start shell: %w", err)
	}

	stopResize := forwardWindowChanges(sess, int(os.Stdout.Fd()))
	defer stopResize()

	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()
	select {
	case <-ctx.Done():
		_ = sess.Close()
		return ctx.Err()
	case err := <-done:
		if err != nil && err != io.EOF {
			return fmt.Errorf("client: ssh session: %w", err)
		}
		return nil
	}
}

func forwardWindowChanges(sess *ssh.Session, fd int) func() {
	changes := make(chan os.Signal, 1)
	signal.Notify(changes, syscall.SIGWINCH)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-changes:
				width, height, err := term.GetSize(fd)
				if err != nil || width <= 0 || height <= 0 {
					continue
				}
				_ = sess.WindowChange(height, width)
			}
		}
	}()
	return func() {
		signal.Stop(changes)
		close(done)
	}
}

func Detach(ctx context.Context, addr string) error {
	sshClient, err := dialTrusted(ctx, addr)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	sess, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("client: new ssh session: %w", err)
	}
	defer sess.Close()

	out, err := sess.CombinedOutput("detach-client")
	if err != nil {
		return fmt.Errorf("client: detach: %w: %s", err, out)
	}
	if len(out) > 0 {
		_, _ = os.Stdout.Write(out)
	}
	return nil
}

func ServerAvailable(ctx context.Context, addr string) (bool, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		if isConnectionRefused(err) {
			return false, nil
		}
		return false, err
	}
	_ = conn.Close()

	sshConn, err := dialTrusted(ctx, addr)
	if err != nil {
		return false, fmt.Errorf("client: port %s is not a trusted shux ssh server: %w", addr, err)
	}
	_ = sshConn.Close()
	return true, nil
}

func spawnDetached(opts AttachOptions) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("client: executable path: %w", err)
	}
	logFile, err := openSpawnLog()
	if err != nil {
		return err
	}
	defer logFile.Close()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return fmt.Errorf("client: open devnull: %w", err)
	}
	defer devNull.Close()

	args := make([]string, 0, 1)
	if opts.Bash {
		args = append(args, "--bash")
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Start()
}

func WaitReady(ctx context.Context, addr string, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()

	for {
		available, err := ServerAvailable(ctx, addr)
		if err != nil {
			return err
		}
		if available {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("client: shux server %s did not become ready within %s", addr, timeout)
		case <-tick.C:
		}
	}
}

func dialTrusted(ctx context.Context, addr string) (*ssh.Client, error) {
	hostKeyPath, err := sshkey.HostKeyPath()
	if err != nil {
		return nil, err
	}
	trusted, err := loadHostPublicKey(hostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("client: shux host key unavailable: %w", err)
	}
	config := &ssh.ClientConfig{
		User:            os.Getenv("USER"),
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.FixedHostKey(trusted),
		Timeout:         time.Second,
	}
	tcp, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	conn, chans, reqs, err := ssh.NewClientConn(tcp, addr, config)
	if err != nil {
		_ = tcp.Close()
		return nil, fmt.Errorf("client: port %s is not a trusted shux ssh server: %w", addr, err)
	}
	return ssh.NewClient(conn, chans, reqs), nil
}

func openSpawnLog() (*os.File, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("client: user cache dir: %w", err)
	}
	dir = filepath.Join(dir, "shux")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("client: create cache dir: %w", err)
	}
	path := filepath.Join(dir, "daemon.log")
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

func loadHostPublicKey(path string) (ssh.PublicKey, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		return nil, err
	}
	return signer.PublicKey(), nil
}

func isConnectionRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}
