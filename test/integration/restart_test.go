package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"shux/internal/client"
	"shux/internal/daemon"
	"shux/internal/persist"
	"shux/internal/protocol"
	"shux/internal/shux"
	"shux/test/testutil"
)

func TestDaemon_gracefulRestartReplacesBackend(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

	layout := persist.LayoutSnapshot{
		WindowID: "w-1",
		Cols:     80,
		Rows:     24,
		Panes:    []persist.LayoutPaneSnapshot{{PaneID: "p-1", Col: 0, Row: 0, Cols: 80, Rows: 24}},
	}
	m := persist.BuildManifest(
		protocol.SessionID("s-1"),
		cfg.ShellPath,
		dir,
		[]protocol.WindowID{"w-1"},
		map[string]persist.LayoutSnapshot{"w-1": layout},
	)
	if err := persist.SaveManifest(dir, m); err != nil {
		t.Fatal(err)
	}

	addr, stop := startTestDaemonWithConfig(t, cfg)
	defer stop()

	if err := client.Restart(t.Context(), addr); err != nil {
		t.Fatal(err)
	}
	if err := client.WaitReady(t.Context(), addr, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	attachAndDetach(t, addr)
}

func TestResurrectionCheckpoint_persistsWindowAndPaneNames(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	app, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}

	ref := app.TestSupervisor()
	sid, wid, pid := app.DefaultSessionID, app.DefaultWindowID, app.DefaultPaneID
	testutil.MustSend(t, ctx, ref, protocol.CommandWindowRename{
		SessionID: sid,
		WindowID:  wid,
		Name:      "workspace",
	})
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneRename{
		SessionID: sid,
		WindowID:  wid,
		PaneID:    pid,
		Name:      "shell",
	})

	deadline := time.Now().Add(testutil.TestWaitTimeout)
	for time.Now().Before(deadline) {
		m, ok, err := persist.LoadManifest(dir)
		if err == nil && ok && len(m.Sessions) > 0 &&
			m.Sessions[0].WindowNames[string(wid)] == "workspace" &&
			m.Sessions[0].PaneNames[persist.PaneNameMapKey(wid, pid)] == "shell" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("checkpoint manifest missing renamed window/pane names")
}

func TestResurrection_windowAndPaneNamesSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/true")

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	app1, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := app1.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}
	ref := app1.TestSupervisor()
	sid, wid, pid := app1.DefaultSessionID, app1.DefaultWindowID, app1.DefaultPaneID
	testutil.MustSend(t, ctx, ref, protocol.CommandWindowRename{SessionID: sid, WindowID: wid, Name: "editor"})
	testutil.MustSend(t, ctx, ref, protocol.CommandPaneRename{SessionID: sid, WindowID: wid, PaneID: pid, Name: "logs"})
	time.Sleep(200 * time.Millisecond)
	if err := app1.Close(); err != nil {
		t.Fatal(err)
	}

	app2, err := shux.NewShuxWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app2.Close()
	if err := app2.BootstrapDefaultSession(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := app2.WindowName(app2.DefaultSessionID, app2.DefaultWindowID); got != "editor" {
		t.Fatalf("window name = %q, want %q", got, "editor")
	}
	if got, _ := app2.PaneName(app2.DefaultSessionID, app2.DefaultWindowID, app2.DefaultPaneID); got != "logs" {
		t.Fatalf("pane name = %q, want %q", got, "logs")
	}
}

func TestDaemon_gracefulRestartPreservesLongRunningProcessWithL3(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	dir := t.TempDir()
	cfg := testutil.ResurrectionConfig(dir, "/bin/sh")
	addr, stop := startTestDaemonWithConfig(t, cfg)
	defer stop()

	sendShellCommandThenDetach(t, addr, "(sleep 1; printf SHUX_L3_AFTER\\n) &\n")

	if err := client.Restart(ctx, addr); err != nil {
		t.Fatal(err)
	}
	if err := client.WaitReady(ctx, addr, 2*time.Second); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1200 * time.Millisecond)
	out := attachAndDetachAfter(t, addr, 150*time.Millisecond)
	if !bytes.Contains(out, []byte("SHUX_L3_AFTER")) {
		t.Fatalf("expected post-restart output marker, got %q", out)
	}
}

func sendShellCommandThenDetach(t *testing.T, addr, command string) {
	t.Helper()

	sshClient, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "test",
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sshClient.Close()

	sess, err := sshClient.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	sess.Stdout = io.Discard
	sess.Stderr = io.Discard
	if err := sess.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	if err := sess.Shell(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond)
	for i := 0; i < len(command); i++ {
		if _, err := stdin.Write([]byte{command[i]}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	if _, err := stdin.Write([]byte{sshCtrlB, 'd'}); err != nil {
		t.Fatal(err)
	}
	_ = stdin.Close()
	if err := sess.Wait(); err != nil {
		var exitErr *ssh.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatal(err)
		}
	}
}

func startTestDaemonWithConfig(t *testing.T, cfg shux.Config) (string, func()) {
	t.Helper()
	addr := freeLoopbackAddr(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- daemon.RunWithConfig(ctx, addr, cfg) }()
	if err := client.WaitReady(t.Context(), addr, 2*time.Second); err != nil {
		cancel()
		t.Fatal(err)
	}
	return addr, func() {
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("daemon stopped with error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("daemon did not stop")
		}
	}
}
