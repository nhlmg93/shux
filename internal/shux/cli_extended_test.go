package shux

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestExtendedCLI_buffersAndEnv(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	app, err := NewShuxWithConfig(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	var out bytes.Buffer

	handled, err := app.HandleRemoteCommand(ctx, []string{"set-buffer", "hello"}, &out)
	if err != nil || !handled {
		t.Fatalf("set-buffer: handled=%v err=%v", handled, err)
	}
	out.Reset()
	handled, err = app.HandleRemoteCommand(ctx, []string{"list-buffers"}, &out)
	if err != nil || !handled {
		t.Fatalf("list-buffers: handled=%v err=%v", handled, err)
	}
	if out.String() == "" {
		t.Fatal("expected buffer listed")
	}

	handled, err = app.HandleRemoteCommand(ctx, []string{"set-environment", "FOO", "bar"}, io.Discard)
	if err != nil || !handled {
		t.Fatalf("set-environment: handled=%v err=%v", handled, err)
	}
	out.Reset()
	handled, err = app.HandleRemoteCommand(ctx, []string{"show-environment"}, &out)
	if err != nil || !handled {
		t.Fatalf("show-environment: handled=%v err=%v", handled, err)
	}
	if !bytes.Contains(out.Bytes(), []byte("FOO=bar")) {
		t.Fatalf("show-environment output: %q", out.String())
	}
}

func TestExtendedCLI_showOptionsAndKeys(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	app, err := NewShuxWithConfig(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	var out bytes.Buffer

	handled, err := app.HandleRemoteCommand(ctx, []string{"show-options", "map-leader"}, &out)
	if err != nil || !handled {
		t.Fatalf("show-options: handled=%v err=%v", handled, err)
	}
	if out.Len() == 0 {
		t.Fatal("expected option output")
	}

	out.Reset()
	handled, err = app.HandleRemoteCommand(ctx, []string{"list-keys"}, &out)
	if err != nil || !handled {
		t.Fatalf("list-keys: handled=%v err=%v", handled, err)
	}
	if out.Len() == 0 {
		t.Fatal("expected key listing")
	}
}
