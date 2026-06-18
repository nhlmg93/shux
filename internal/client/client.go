package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"shux/internal/protocol"
	"shux/internal/sshkey"
)

type AttachOptions struct {
	Bash          bool
	Control       bool
	TargetSession string
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
		return attachWithMode(ctx, addr, opts)
	}
	if err := spawnDetached(opts); err != nil {
		return err
	}
	if err := WaitReady(ctx, addr, 2*time.Second); err != nil {
		return err
	}
	return attachWithMode(ctx, addr, opts)
}

func attachWithMode(ctx context.Context, addr string, opts AttachOptions) error {
	if opts.Control {
		return AttachControl(ctx, addr)
	}
	return AttachWithOptions(ctx, addr, opts)
}

func Attach(ctx context.Context, addr string) error {
	return AttachWithOptions(ctx, addr, AttachOptions{})
}

func AttachWithOptions(ctx context.Context, addr string, opts AttachOptions) error {
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
	if opts.TargetSession != "" {
		if !protocol.ValidSessionName(opts.TargetSession) {
			return fmt.Errorf("client: invalid session target %q", opts.TargetSession)
		}
		if err := sess.Start("attach -t " + opts.TargetSession); err != nil {
			return fmt.Errorf("client: start attach target: %w", err)
		}
	} else {
		if err := sess.Shell(); err != nil {
			return fmt.Errorf("client: start shell: %w", err)
		}
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

func AttachControl(ctx context.Context, addr string) error {
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

	sess.Stdin = os.Stdin
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr
	if err := sess.Start("control-mode"); err != nil {
		return fmt.Errorf("client: start control mode: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()
	select {
	case <-ctx.Done():
		_ = sess.Close()
		return ctx.Err()
	case err := <-done:
		if err != nil && err != io.EOF {
			return fmt.Errorf("client: control session: %w", err)
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

func NewSession(ctx context.Context, addr string, opts AttachOptions, name string) error {
	if !protocol.ValidSessionName(name) {
		return fmt.Errorf("client: invalid session name %q", name)
	}
	if err := ensureServer(ctx, addr, opts); err != nil {
		return err
	}
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

	out, err := sess.CombinedOutput("new-session -s " + name)
	if err != nil {
		return fmt.Errorf("client: new-session: %w: %s", err, out)
	}
	if len(out) > 0 {
		_, _ = os.Stdout.Write(out)
	}
	return nil
}

func KillSession(ctx context.Context, addr string, opts AttachOptions, name string) error {
	if !protocol.ValidSessionName(name) {
		return fmt.Errorf("client: invalid session name %q", name)
	}
	if err := ensureServer(ctx, addr, opts); err != nil {
		return err
	}
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

	out, err := sess.CombinedOutput("kill-session -t " + name)
	if err != nil {
		return fmt.Errorf("client: kill-session: %w: %s", err, out)
	}
	if len(out) > 0 {
		_, _ = os.Stdout.Write(out)
	}
	return nil
}

func ListSessions(ctx context.Context, addr string, opts AttachOptions) ([]string, error) {
	if err := ensureServer(ctx, addr, opts); err != nil {
		return nil, err
	}
	sshClient, err := dialTrusted(ctx, addr)
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()

	sess, err := sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("client: new ssh session: %w", err)
	}
	defer sess.Close()

	out, err := sess.CombinedOutput("list-sessions")
	if err != nil {
		return nil, fmt.Errorf("client: list-sessions: %w: %s", err, out)
	}
	lines := strings.Split(string(out), "\n")
	sessions := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "  ") {
			line = strings.TrimSpace(line[1:])
		}
		sessions = append(sessions, line)
	}
	return sessions, nil
}

func ensureServer(ctx context.Context, addr string, opts AttachOptions) error {
	available, err := ServerAvailable(ctx, addr)
	if err != nil {
		return err
	}
	if available {
		return nil
	}
	if err := spawnDetached(opts); err != nil {
		return err
	}
	return WaitReady(ctx, addr, 2*time.Second)
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
	cmd.Env = spawnEnv()
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Start()
}

// spawnEnv returns a clean environment for daemon children. Strip XDG overrides
// and SHUX_DAEMON so spawned daemons use the user's real config and host key.
func spawnEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "XDG_CONFIG_HOME=") ||
			strings.HasPrefix(e, "XDG_STATE_HOME=") ||
			strings.HasPrefix(e, "SHUX_DAEMON=") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// SpawnAndWaitReady starts a detached daemon child and waits until it accepts SSH.
func SpawnAndWaitReady(ctx context.Context, addr string, opts AttachOptions, timeout time.Duration) error {
	if err := spawnDetached(opts); err != nil {
		return err
	}
	return WaitReady(ctx, addr, timeout)
}

func Restart(ctx context.Context, addr string) error {
	out, err := runExec(ctx, addr, "restart-daemon")
	if err != nil {
		if isConnectionRefused(err) {
			return fmt.Errorf("client: no shux daemon listening on %s (start one with ./shux)", addr)
		}
		return fmt.Errorf("client: restart: %w", err)
	}
	if len(out) > 0 {
		_, _ = os.Stdout.Write(out)
	}
	return nil
}

func ListWindows(ctx context.Context, addr string, jsonOutput bool) error {
	resp, err := runQuery(ctx, addr, protocol.QueryRequest{Method: protocol.QueryListWindows})
	if err != nil {
		return fmt.Errorf("client: list-windows: %w", err)
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(resp.Windows)
	}
	writeWindowsTable(os.Stdout, resp.Windows)
	return nil
}

func ListPanes(ctx context.Context, addr string, jsonOutput bool) error {
	resp, err := runQuery(ctx, addr, protocol.QueryRequest{Method: protocol.QueryListPanes})
	if err != nil {
		return fmt.Errorf("client: list-panes: %w", err)
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(resp.Panes)
	}
	writePanesTable(os.Stdout, resp.Panes)
	return nil
}

func DisplayMessage(ctx context.Context, addr string, format string, jsonOutput bool) error {
	resp, err := runQuery(ctx, addr, protocol.QueryRequest{
		Method: protocol.QueryDisplayMessage,
		Format: format,
	})
	if err != nil {
		return fmt.Errorf("client: display-message: %w", err)
	}
	if resp.Display == nil {
		return fmt.Errorf("client: display-message: daemon returned empty response")
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(resp.Display)
	}
	_, _ = fmt.Fprintln(os.Stdout, resp.Display.Message)
	return nil
}

func runExec(ctx context.Context, addr string, command string) ([]byte, error) {
	sshClient, err := dialTrusted(ctx, addr)
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()

	sess, err := sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("client: new ssh session: %w", err)
	}
	defer sess.Close()

	out, err := sess.CombinedOutput(command)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, out)
	}
	return out, nil
}

func runQuery(ctx context.Context, addr string, req protocol.QueryRequest) (protocol.QueryResponse, error) {
	sshClient, err := dialTrusted(ctx, addr)
	if err != nil {
		return protocol.QueryResponse{}, err
	}
	defer sshClient.Close()

	sess, err := sshClient.NewSession()
	if err != nil {
		return protocol.QueryResponse{}, fmt.Errorf("client: new ssh session: %w", err)
	}
	defer sess.Close()

	payload, err := json.Marshal(req)
	if err != nil {
		return protocol.QueryResponse{}, fmt.Errorf("client: marshal query request: %w", err)
	}
	sess.Stdin = bytes.NewReader(payload)
	out, err := sess.CombinedOutput("query")
	if err != nil {
		return protocol.QueryResponse{}, fmt.Errorf("%w: %s", err, out)
	}
	var resp protocol.QueryResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return protocol.QueryResponse{}, fmt.Errorf("client: decode query response: %w", err)
	}
	return resp, nil
}

func writeWindowsTable(w io.Writer, windows []protocol.WindowInfo) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "INDEX\tSESSION\tWINDOW\tPANES")
	for _, window := range windows {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%d\n", window.Index, window.SessionID, window.WindowID, window.PaneCount)
	}
	_ = tw.Flush()
}

func writePanesTable(w io.Writer, panes []protocol.PaneInfo) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "INDEX\tSESSION\tWINDOW\tWIN_INDEX\tPANE\tCOL\tROW\tCOLS\tROWS")
	for _, pane := range panes {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\t%d\t%d\t%d\t%d\n",
			pane.Index,
			pane.SessionID,
			pane.WindowID,
			pane.WindowIndex,
			pane.PaneID,
			pane.Col,
			pane.Row,
			pane.Cols,
			pane.Rows,
		)
	}
	_ = tw.Flush()
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

func RunControlCommand(ctx context.Context, addr string, argv ...string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("client: empty control command")
	}
	sshClient, err := dialTrusted(ctx, addr)
	if err != nil {
		return "", err
	}
	defer sshClient.Close()

	sess, err := sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("client: new ssh session: %w", err)
	}
	defer sess.Close()

	out, err := sess.CombinedOutput(strings.Join(argv, " "))
	if err != nil {
		return "", fmt.Errorf("client: %s: %w: %s", argv[0], err, out)
	}
	return string(out), nil
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
