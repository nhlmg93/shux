package actor

import "context"

type Ref[M any] struct {
	inbox chan M
}

func (r Ref[M]) Send(ctx context.Context, msg M) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r.inbox <- msg:
		return nil
	}
}

func Start[M any](ctx context.Context, run func(context.Context, Ref[M], <-chan M)) Ref[M] {
	inbox := make(chan M)
	ref := Ref[M]{inbox: inbox}

	go run(ctx, ref, inbox)

	return ref
}
