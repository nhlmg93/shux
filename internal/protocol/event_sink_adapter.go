package protocol

import (
	"context"
	"fmt"
)

// EventChanAdapter adapts a buffered event channel to EventSink.
// DeliverEvent is non-blocking: a full channel is reported to the hub as a
// failed sink so slow subscribers cannot stall actor progress.
type EventChanAdapter chan Event

func (s EventChanAdapter) DeliverEvent(ctx context.Context, e Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s <- e:
		return nil
	default:
		return fmt.Errorf("event sink full")
	}
}
