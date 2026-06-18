package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"shux/internal/client"
	"shux/internal/daemon"
	"shux/internal/shux"
)

const sshCtrlB = 0x02

func TestDaemonStartsTrustedWishServer(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	available, err := client.ServerAvailable(t.Context(), addr)
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("expected daemon to be available")
	}
}

func TestSSHClientCanAttachWithPTYAndDetach(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	attachAndDetach(t, addr)
}

func TestTwoSSHClientsCanAttachConcurrentlyAndDetachIndependently(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	firstDone := make(chan []byte, 1)
	go func() { firstDone <- attachAndDetachAfter(t, addr, 200*time.Millisecond) }()

	attachAndDetach(t, addr)

	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("first client did not detach")
	}

	available, err := client.ServerAvailable(t.Context(), addr)
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("daemon should remain available after clients detach")
	}
}

func TestSecondDaemonCandidateExitsCleanlyWhenShuxOwnsPort(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	if err := daemon.RunWithConfig(t.Context(), addr, shux.DefaultConfig()); err != nil {
		t.Fatalf("second daemon candidate should exit cleanly: %v", err)
	}
}

func TestClientQuitBindingDoesNotStopDaemonWhenPeerRemains(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	peerDone := make(chan []byte, 1)
	go func() { peerDone <- attachAndDetachAfter(t, addr, 400*time.Millisecond) }()
	time.Sleep(100 * time.Millisecond)

	attachAndSendKeys(t, addr, []byte{sshCtrlB, '!'}, 0)

	select {
	case <-peerDone:
		t.Fatal("peer client exited when another client quit")
	case <-time.After(200 * time.Millisecond):
	}

	select {
	case <-peerDone:
	case <-time.After(time.Second):
		t.Fatal("peer client did not detach after its own detach key")
	}

	available, err := client.ServerAvailable(t.Context(), addr)
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("daemon should remain available after non-last client quit")
	}
}

func TestClientDisplayPanesBindingDoesNotStopDaemon(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	// prefix q opens pane quick select; digit selects a pane; prefix d detaches.
	attachAndSendKeys(t, addr, []byte{sshCtrlB, 'q', '1', sshCtrlB, 'd'}, 0)

	available, err := client.ServerAvailable(t.Context(), addr)
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("daemon should remain available after display panes binding")
	}
}

func TestLastClientDetachDoesNotStopDaemon(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	attachAndDetachAfter(t, addr, 100*time.Millisecond)

	available, err := client.ServerAvailable(t.Context(), addr)
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("daemon should remain available after last client detach")
	}
}

func TestNamedSessions_create_list_attachTarget(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	addr, stop := startTestDaemon(t)
	defer stop()

	if err := client.NewSession(ctx, addr, client.AttachOptions{}, "work"); err != nil {
		t.Fatal(err)
	}
	sessions, err := client.ListSessions(ctx, addr, client.AttachOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !containsSession(sessions, "main") || !containsSession(sessions, "work") {
		t.Fatalf("sessions = %v, want main and work", sessions)
	}

	attachAndSendKeysCommand(t, addr, "attach -t work", []byte{sshCtrlB, 'd'}, 0)
}

func TestNamedSessions_respectsMaxSessionsBound(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	cfg := shux.DefaultConfig()
	cfg.MaxSessions = 2
	addr, stop := startTestDaemonWithConfig(t, cfg)
	defer stop()

	if err := client.NewSession(ctx, addr, client.AttachOptions{}, "work"); err != nil {
		t.Fatal(err)
	}
	if err := client.NewSession(ctx, addr, client.AttachOptions{}, "extra"); err == nil {
		t.Fatal("expected max-sessions error")
	}
}

func TestDaemonStopsAfterClientQuitBinding(t *testing.T) {
	addr := freeLoopbackAddr(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- daemon.RunWithConfig(ctx, addr, shux.DefaultConfig()) }()
	defer cancel()

	if err := client.WaitReady(t.Context(), addr, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	attachAndSendKeys(t, addr, []byte{sshCtrlB, '!'}, 0)

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("daemon stopped with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not stop after quit binding")
	}
}

func TestCLIIntrospectionCommands(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	windowsText := captureStdout(t, func() {
		if err := client.ListWindows(ctx, addr, false); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(windowsText, "WINDOW") || !strings.Contains(windowsText, "w-1") {
		t.Fatalf("list-windows output missing expected values: %q", windowsText)
	}

	windowsJSON := captureStdout(t, func() {
		if err := client.ListWindows(ctx, addr, true); err != nil {
			t.Fatal(err)
		}
	})
	var windows []struct {
		Index     int    `json:"index"`
		SessionID string `json:"session_id"`
		WindowID  string `json:"window_id"`
		PaneCount int    `json:"pane_count"`
	}
	if err := json.Unmarshal([]byte(windowsJSON), &windows); err != nil {
		t.Fatalf("unmarshal list-windows json: %v; raw=%q", err, windowsJSON)
	}
	if len(windows) != 1 || windows[0].WindowID != "w-1" || windows[0].PaneCount != 1 {
		t.Fatalf("unexpected windows: %#v", windows)
	}

	panesJSON := captureStdout(t, func() {
		if err := client.ListPanes(ctx, addr, true); err != nil {
			t.Fatal(err)
		}
	})
	var panes []struct {
		PaneID string `json:"pane_id"`
		Cols   int    `json:"cols"`
		Rows   int    `json:"rows"`
	}
	if err := json.Unmarshal([]byte(panesJSON), &panes); err != nil {
		t.Fatalf("unmarshal list-panes json: %v; raw=%q", err, panesJSON)
	}
	if len(panes) != 1 || panes[0].PaneID != "p-1" {
		t.Fatalf("unexpected panes: %#v", panes)
	}

	displayText := captureStdout(t, func() {
		if err := client.DisplayMessage(ctx, addr, "#{pane_id}", false); err != nil {
			t.Fatal(err)
		}
	})
	if strings.TrimSpace(displayText) != "p-1" {
		t.Fatalf("display-message text=%q want %q", displayText, "p-1")
	}

	displayJSON := captureStdout(t, func() {
		if err := client.DisplayMessage(ctx, addr, "#{session_id}:#{window_id}:#{pane_id}", true); err != nil {
			t.Fatal(err)
		}
	})
	var msg struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(displayJSON), &msg); err != nil {
		t.Fatalf("unmarshal display-message json: %v; raw=%q", err, displayJSON)
	}
	if msg.Message != "s-1:w-1:p-1" {
		t.Fatalf("display-message json=%q want %q", msg.Message, "s-1:w-1:p-1")
	}
}

func TestCLIWindowPaneCommands(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := client.HasSession(ctx, addr, client.AttachOptions{}, "main"); err != nil {
		t.Fatalf("has-session main: %v", err)
	}
	if err := client.HasSession(ctx, addr, client.AttachOptions{}, "missing"); err == nil {
		t.Fatal("has-session missing should fail")
	}

	if err := client.NewWindow(ctx, addr, ""); err != nil {
		t.Fatalf("new-window: %v", err)
	}

	windowsJSON := captureStdout(t, func() {
		if err := client.ListWindowsWithTarget(ctx, addr, true, "main"); err != nil {
			t.Fatal(err)
		}
	})
	var windows []struct {
		WindowID string `json:"window_id"`
	}
	if err := json.Unmarshal([]byte(windowsJSON), &windows); err != nil {
		t.Fatal(err)
	}
	if len(windows) < 2 {
		t.Fatalf("expected at least 2 windows after new-window, got %d", len(windows))
	}

	if err := client.SplitWindow(ctx, addr, "main:2", true); err != nil {
		t.Fatalf("split-window: %v", err)
	}

	panesJSON := captureStdout(t, func() {
		if err := client.ListPanesWithTarget(ctx, addr, true, "main"); err != nil {
			t.Fatal(err)
		}
	})
	var panes []struct {
		PaneID string `json:"pane_id"`
	}
	if err := json.Unmarshal([]byte(panesJSON), &panes); err != nil {
		t.Fatal(err)
	}
	if len(panes) < 2 {
		t.Fatalf("expected at least 2 panes after split, got %d", len(panes))
	}

	captureStdout(t, func() {
		if err := client.ListCommands(ctx, addr); err != nil {
			t.Fatalf("list-commands: %v", err)
		}
	})
}

func startTestDaemon(t *testing.T) (string, func()) {
	t.Helper()
	return startTestDaemonWithConfig(t, shux.DefaultConfig())
}

func attachAndDetach(t *testing.T, addr string) []byte {
	t.Helper()
	return attachAndDetachAfter(t, addr, 0)
}

func attachAndDetachAfter(t *testing.T, addr string, delay time.Duration) []byte {
	t.Helper()
	return attachAndSendKeys(t, addr, []byte{sshCtrlB, 'd'}, delay)
}

func attachAndSendKeys(t *testing.T, addr string, keys []byte, delay time.Duration) []byte {
	return attachAndSendKeysCommand(t, addr, "", keys, delay)
}

func attachAndSendKeysCommand(t *testing.T, addr, command string, keys []byte, delay time.Duration) []byte {
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
	stdout, err := sess.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	out := &readyBuffer{ready: make(chan struct{})}
	copyDone := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(out, stdout); copyDone <- struct{}{} }()
	go func() { _, _ = io.Copy(out, stderr); copyDone <- struct{}{} }()

	if err := sess.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	if command != "" {
		if err := sess.Start(command); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := sess.Shell(); err != nil {
			t.Fatal(err)
		}
	}

	go func() {
		select {
		case <-out.ready:
		case <-time.After(time.Second):
		}
		if delay > 0 {
			time.Sleep(delay)
		}
		_, _ = stdin.Write(keys)
		_ = stdin.Close()
	}()

	err = sess.Wait()
	<-copyDone
	<-copyDone
	if err != nil && err != io.EOF {
		var exitErr *ssh.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatal(err)
		}
	}
	return out.Bytes()
}

func TestKillSession_removesNamedSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	addr, stop := startTestDaemon(t)
	defer stop()

	if err := client.NewSession(ctx, addr, client.AttachOptions{}, "work"); err != nil {
		t.Fatal(err)
	}
	if err := client.KillSession(ctx, addr, client.AttachOptions{}, "work"); err != nil {
		t.Fatal(err)
	}
	sessions, err := client.ListSessions(ctx, addr, client.AttachOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !containsSession(sessions, "main") {
		t.Fatalf("sessions = %v, want main", sessions)
	}
	if containsSession(sessions, "work") {
		t.Fatalf("sessions = %v, work should be removed", sessions)
	}
}

func TestKillLastSession_stopsDaemon(t *testing.T) {
	addr := freeLoopbackAddr(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- daemon.RunWithConfig(ctx, addr, shux.DefaultConfig()) }()

	if err := client.WaitReady(t.Context(), addr, 2*time.Second); err != nil {
		cancel()
		t.Fatal(err)
	}

	killCtx, killCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer killCancel()
	if err := client.KillSession(killCtx, addr, client.AttachOptions{}, "main"); err != nil {
		cancel()
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("daemon stopped with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("daemon did not stop after killing last session")
	}

	available, err := client.ServerAvailable(t.Context(), addr)
	if err != nil {
		t.Fatal(err)
	}
	if available {
		t.Fatal("daemon should not remain available after last session killed")
	}
}

func containsSession(sessions []string, name string) bool {
	for _, session := range sessions {
		if session == name {
			return true
		}
	}
	return false
}

type readyBuffer struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	once  sync.Once
	ready chan struct{}
}

func (b *readyBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	n, err := b.buf.Write(p)
	b.mu.Unlock()
	if n > 0 {
		b.once.Do(func() { close(b.ready) })
	}
	return n, err
}

func (b *readyBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()

	fn()

	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}
