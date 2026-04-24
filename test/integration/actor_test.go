package integration_test

import (
	"context"
	"testing"
	"time"

	"shux-dev/internal/protocol"
	"shux-dev/internal/supervisor"
)

func TestStart_acceptsCommandNoop(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ref := supervisor.Start(ctx)
	if err := ref.Send(ctx, protocol.CommandNoop{}); err != nil {
		t.Fatal(err)
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
}
