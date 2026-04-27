package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"shux/internal/client"
	"shux/internal/daemon"
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
	go func() { firstDone <- attachAndDetachAfter(t, addr, 500*time.Millisecond) }()

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

	if err := daemon.Run(t.Context(), addr); err != nil {
		t.Fatalf("second daemon candidate should exit cleanly: %v", err)
	}
}

func TestClientQuitBindingDoesNotStopDaemonWhenPeerRemains(t *testing.T) {
	addr, stop := startTestDaemon(t)
	defer stop()

	peerDone := make(chan []byte, 1)
	go func() { peerDone <- attachAndDetachAfter(t, addr, 1200*time.Millisecond) }()
	time.Sleep(200 * time.Millisecond)

	attachAndSendKeys(t, addr, []byte{sshCtrlB, 'q'}, 0)

	select {
	case <-peerDone:
		t.Fatal("peer client exited when another client quit")
	case <-time.After(500 * time.Millisecond):
	}

	select {
	case <-peerDone:
	case <-time.After(2 * time.Second):
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

func TestDaemonStopsAfterClientQuitBinding(t *testing.T) {
	addr := freeLoopbackAddr(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- daemon.Run(ctx, addr) }()
	defer cancel()

	if err := client.WaitReady(t.Context(), addr, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	attachAndSendKeys(t, addr, []byte{sshCtrlB, 'q'}, 0)

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("daemon stopped with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not stop after quit binding")
	}
}

func startTestDaemon(t *testing.T) (string, func()) {
	t.Helper()
	addr := freeLoopbackAddr(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- daemon.Run(ctx, addr) }()
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

func attachAndDetach(t *testing.T, addr string) []byte {
	t.Helper()
	return attachAndDetachAfter(t, addr, 0)
}

func attachAndDetachAfter(t *testing.T, addr string, delay time.Duration) []byte {
	t.Helper()
	return attachAndSendKeys(t, addr, []byte{sshCtrlB, 'd'}, delay)
}

func attachAndSendKeys(t *testing.T, addr string, keys []byte, delay time.Duration) []byte {
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
	if err := sess.Shell(); err != nil {
		t.Fatal(err)
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
