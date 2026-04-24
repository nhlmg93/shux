package pane

import (
	"context"

	"github.com/mitchellh/go-libghostty"
	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

// Actor runs a single pane. VT is a libghostty handle; it is nil until
// a follow-up creates the terminal with known dimensions (WithSize).
type Actor struct {
	VT *libghostty.Terminal
}

// NewActor returns a pane actor. VT is nil until dimensions are wired.
func NewActor() *Actor {
	return &Actor{}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	defer func() {
		if a.VT != nil {
			a.VT.Close()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-inbox:
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor().Run)
}
