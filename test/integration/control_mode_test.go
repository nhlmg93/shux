package integration

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"shux/internal/shux"
)

func TestControlMode_scriptedStdinStdout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	shell := writeControlTestShell(t)
	cfg := shux.DefaultConfig()
	cfg.ShellPath = shell

	addr, stop := startTestDaemonWithConfig(t, cfg)
	defer stop()

	cli, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "test",
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	sess, err := cli.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}

	lines := streamLines(ctx, io.MultiReader(stdout, stderr))
	if err := sess.Start("control-mode"); err != nil {
		t.Fatal(err)
	}

	writeLine(t, stdin, "subscribe pane-output layout-change")
	expectLine(t, ctx, lines, func(line string) bool {
		return strings.HasPrefix(line, "ok subscribe")
	}, "ok subscribe")

	writeLine(t, stdin, "new-window")
	newWindow := expectLine(t, ctx, lines, func(line string) bool {
		return strings.HasPrefix(line, "ok new-window")
	}, "ok new-window")
	newPane := parseField(newWindow, "pane")
	if newPane == "" {
		t.Fatalf("new-window response missing pane: %q", newWindow)
	}

	expectLine(t, ctx, lines, func(line string) bool {
		return strings.HasPrefix(line, "%layout")
	}, "%layout notification")
	expectLine(t, ctx, lines, func(line string) bool {
		return strings.HasPrefix(line, "%output") && strings.Contains(line, "CONTROL_MODE_READY")
	}, "%output notification")

	writeLine(t, stdin, "split horizontal")
	splitLine := expectLine(t, ctx, lines, func(line string) bool {
		return strings.HasPrefix(line, "ok split")
	}, "ok split")
	splitPane := parseField(splitLine, "pane")
	if splitPane == "" {
		t.Fatalf("split response missing pane: %q", splitLine)
	}

	writeLine(t, stdin, "select-pane "+newPane)
	expectLine(t, ctx, lines, func(line string) bool {
		return strings.HasPrefix(line, "ok select-pane") && strings.Contains(line, "pane="+newPane)
	}, "ok select-pane")

	writeLine(t, stdin, "capture-pane "+newPane)
	expectLine(t, ctx, lines, func(line string) bool {
		return strings.HasPrefix(line, "ok capture-pane") && strings.Contains(line, "pane="+newPane)
	}, "ok capture-pane")

	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sess.Wait(); err != nil && !errors.Is(err, io.EOF) {
		var exitErr *ssh.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatal(err)
		}
	}
}

func writeControlTestShell(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control-shell.sh")
	script := "#!/bin/sh\n" +
		"echo CONTROL_MODE_READY\n" +
		"exec /bin/sh\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeLine(t *testing.T, w io.Writer, line string) {
	t.Helper()
	if _, err := fmt.Fprintln(w, line); err != nil {
		t.Fatal(err)
	}
}

func streamLines(ctx context.Context, r io.Reader) <-chan string {
	out := make(chan string, 128)
	go func() {
		defer close(out)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 256), 64*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- line:
			}
		}
	}()
	return out
}

func expectLine(t *testing.T, ctx context.Context, lines <-chan string, match func(string) bool, want string) string {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for %s", want)
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("stream closed waiting for %s", want)
			}
			if match(line) {
				return line
			}
		}
	}
}

func parseField(line, key string) string {
	parts := strings.Fields(line)
	for _, part := range parts {
		if strings.HasPrefix(part, key+"=") {
			return strings.TrimPrefix(part, key+"=")
		}
	}
	return ""
}
