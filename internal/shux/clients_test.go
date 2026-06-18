package shux

import (
	"context"
	"testing"
	"time"

	"shux/internal/protocol"
)

func TestResolveClientID_singleAndMulti(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	app, err := NewShuxWithConfig(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	if _, err := app.resolveClientID(""); err == nil {
		t.Fatal("expected error with no clients")
	}

	app.registerClient("c-1", protocol.SessionID("s-1"), nil)
	if id, err := app.resolveClientID(""); err != nil || id != "c-1" {
		t.Fatalf("single client: id=%q err=%v", id, err)
	}

	app.registerClient("c-2", protocol.SessionID("s-2"), nil)
	if _, err := app.resolveClientID(""); err == nil {
		t.Fatal("expected error with multiple clients")
	}
	if id, err := app.resolveClientID("c-2"); err != nil || id != "c-2" {
		t.Fatalf("explicit client: id=%q err=%v", id, err)
	}
}
