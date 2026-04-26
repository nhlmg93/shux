package pane

import (
	"context"
	"fmt"

	"github.com/mitchellh/go-libghostty"
	"shux/internal/actor"
	"shux/internal/protocol"
)

// Default cell pixel size for libghostty Resize, matching go-libghostty test usage.
// Init uses NewTerminal(WithSize); Resize must be given explicit cell dimensions.
const defaultCellW, defaultCellH = 8, 16

// Actor runs a single pane. VT is a libghostty handle; it is nil until
// a follow-up creates the terminal with known dimensions (WithSize).
type Actor struct {
	VT *libghostty.Terminal
}

// NewActor returns a pane actor. VT is nil until dimensions are wired.
func NewActor() *Actor {
	return &Actor{}
}

// initTerminal allocates a libghostty Terminal exactly once. Cols and rows are
// cell counts as uint16: they cannot be negative at the type level; zero is invalid.
// It panics on zero size, double init, or if NewTerminal returns an error.
func (a *Actor) initTerminal(cols, rows uint16) {
	if cols == 0 || rows == 0 {
		panic(fmt.Sprintf("pane: InitTerminal: invalid size %dx%d (cols and rows must be positive)", cols, rows))
	}
	if a.VT != nil {
		panic("pane: InitTerminal: terminal already created (double init)")
	}
	term, err := libghostty.NewTerminal(libghostty.WithSize(cols, rows))
	if err != nil {
		panic(fmt.Sprintf("pane: NewTerminal: %v", err))
	}
	a.VT = term
}

// resizeTerminal changes VT dimensions after init. It panics on zero size,
// resize before init, or if Resize returns an error.
func (a *Actor) resizeTerminal(cols, rows uint16) {
	if cols == 0 || rows == 0 {
		panic(fmt.Sprintf("pane: resizeTerminal: invalid size %dx%d (cols and rows must be positive)", cols, rows))
	}
	if a.VT == nil {
		panic("pane: resizeTerminal: terminal not created (resize before init)")
	}
	if err := a.VT.Resize(cols, rows, defaultCellW, defaultCellH); err != nil {
		panic(fmt.Sprintf("pane: Resize: %v", err))
	}
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
		case msg := <-inbox:
			if err := protocol.ValidateCommand(msg); err != nil {
				panic(err)
			}
			switch msg := msg.(type) {
			case protocol.CommandNoop:
			case protocol.CommandPaneInit:
				a.initTerminal(msg.Cols, msg.Rows)
			case protocol.CommandPaneResize:
				a.resizeTerminal(msg.Cols, msg.Rows)
			default:
				panic(fmt.Sprintf("pane: unhandled command type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor().Run)
}
