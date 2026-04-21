package shux

import (
	"fmt"
	"reflect"
	"sync"
)

type askEnvelope struct {
	msg   any
	reply chan any
}

type loopRef struct {
	inbox chan any
	stop  chan struct{}
	done  chan struct{}
	once  sync.Once
}

func newLoopRef(buffer int) *loopRef {
	return &loopRef{
		inbox: make(chan any, buffer),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
}

func (r *loopRef) send(msg any) bool {
	select {
	case <-r.stop:
		return false
	case <-r.done:
		return false
	default:
	}

	select {
	case <-r.stop:
		return false
	case <-r.done:
		return false
	case r.inbox <- msg:
		return true
	}
}

func (r *loopRef) ask(msg any) chan any {
	reply := make(chan any, 1)
	if !r.send(askEnvelope{msg: msg, reply: reply}) {
		reply <- nil
	}
	return reply
}

func (r *loopRef) stopLoop() {
	r.once.Do(func() {
		close(r.stop)
	})
}

func (r *loopRef) shutdown() {
	r.stopLoop()
	<-r.done
}

// Public methods for embedding types

// Send sends a message to the loop's inbox. Returns false if stopped.
func (r *loopRef) Send(msg any) bool {
	if r == nil {
		return false
	}
	return r.send(msg)
}

// Ask sends a message and returns a channel for the reply. Returns nil if stopped.
func (r *loopRef) Ask(msg any) chan any {
	if r == nil {
		return nil
	}
	return r.ask(msg)
}

// Stop signals the loop to stop (idempotent).
func (r *loopRef) Stop() {
	if r != nil {
		r.stopLoop()
	}
}

// Shutdown stops the loop and waits for it to complete.
func (r *loopRef) Shutdown() {
	if r != nil {
		r.shutdown()
	}
}

type asker interface {
	Ask(msg any) chan any
}

func askValue(ref asker, msg any) (any, bool) {
	if ref == nil {
		return nil, false
	}
	v := reflect.ValueOf(ref)
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return nil, false
	}
	reply := ref.Ask(msg)
	if reply == nil {
		return nil, false
	}
	result, ok := <-reply
	if isNilValue(result) {
		return nil, ok
	}
	return result, ok
}

func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// recoverWithContext generates rich crash context for panic recovery.
// Used by all actor run() methods to log state on crash.
func recoverWithContext(actorType string, id uint32, collectionCount, activeID int) string {
	return fmt.Sprintf("actor=%s id=%d collection_count=%d active_id=%d",
		actorType, id, collectionCount, activeID)
}
