package actor

import (
	"context"
	"errors"
)

var ErrInvalidRef = errors.New("actor: invalid ref")
var ErrStopped = errors.New("actor: stopped")

type Ref[M any] struct {
	inbox chan M
	done  <-chan struct{}
}

func (r Ref[M]) Valid() bool {
	return r.inbox != nil && r.done != nil
}

func (r Ref[M]) Send(ctx context.Context, msg M) error {
	if !r.Valid() {
		return ErrInvalidRef
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.done:
		return ErrStopped
	case r.inbox <- msg:
		return nil
	}
}

func Start[M any](ctx context.Context, run func(context.Context, Ref[M], <-chan M)) Ref[M] {
	if ctx == nil {
		panic("actor: Start: nil context")
	}
	if run == nil {
		panic("actor: Start: nil run")
	}
	inbox := make(chan M)
	done := make(chan struct{})
	ref := Ref[M]{inbox: inbox, done: done}

	go func() {
		defer close(done)
		run(ctx, ref, inbox)
	}()

	return ref
}
