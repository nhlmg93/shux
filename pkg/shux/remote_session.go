package shux

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// RemoteSessionRef proxies session operations over IPC to a live owner.
// It mimics SessionRef's interface for UI compatibility.
type RemoteSessionRef struct {
	client       *IPCClient
	updateClient *IPCClient
	sessionName  string
	stopped      atomic.Bool
	mu           sync.RWMutex
	subscribers  map[chan any]struct{}
	updates      chan any
	stop         chan struct{}
	wg           sync.WaitGroup
	logger       ShuxLogger
}

// NewRemoteSessionRef creates a new remote session reference connected to a live owner.
func NewRemoteSessionRef(socketPath string, sessionName string, logger ShuxLogger) (*RemoteSessionRef, error) {
	client, err := DialIPC(socketPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to live session: %w", err)
	}
	updateClient, err := DialIPC(socketPath, logger)
	if err != nil {
		client.Stop()
		return nil, fmt.Errorf("failed to connect update stream: %w", err)
	}

	ref := &RemoteSessionRef{
		client:       client,
		updateClient: updateClient,
		sessionName:  sessionName,
		subscribers:  make(map[chan any]struct{}),
		updates:      make(chan any, 32),
		stop:         make(chan struct{}),
		logger:       logger,
	}

	ref.wg.Add(1)
	go ref.updateLoop()

	updateClient.Start(func(msg any) {
		ref.handleUpdate(msg)
	})
	if err := updateClient.Send(IPCSubscribeUpdates{}); err != nil {
		updateClient.Stop()
		client.Stop()
		return nil, fmt.Errorf("failed to subscribe to updates: %w", err)
	}

	return ref, nil
}

// Send dispatches a message to the remote session owner.
// Returns false if the connection is closed.
func (r *RemoteSessionRef) Send(msg any) bool {
	if r.stopped.Load() {
		return false
	}

	switch m := msg.(type) {
	case SubscribeUpdates:
		r.mu.Lock()
		if m.Subscriber != nil {
			r.subscribers[m.Subscriber] = struct{}{}
		}
		r.mu.Unlock()
		return true
	case UnsubscribeUpdates:
		r.mu.Lock()
		if m.Subscriber != nil {
			delete(r.subscribers, m.Subscriber)
		}
		r.mu.Unlock()
		return true
	}

	var ipcMsg any
	switch m := msg.(type) {
	case ActionMsg:
		ipcMsg = IPCActionMsg(m)
	case KeyInput:
		ipcMsg = IPCKeyInput(m)
	case MouseInput:
		ipcMsg = IPCMouseInput(m)
	case WriteToPane:
		ipcMsg = IPCWriteToPane(m)
	case ResizeMsg:
		ipcMsg = IPCResizeMsg(m)
	case CreateWindow:
		ipcMsg = IPCCreateWindow(m)
	case DetachSession:
		ipcMsg = IPCDetachSession{}
	case ExecuteCommandMsg:
		ipcMsg = IPCExecuteCommandMsg(m)
	default:
		if r.logger != nil {
			r.logger.Warnf("remote: unsupported message type %T", msg)
		}
		return false
	}

	if err := r.client.Send(ipcMsg); err != nil {
		if r.logger != nil {
			r.logger.Warnf("remote: send failed: %v", err)
		}
		return false
	}
	return true
}

// Ask sends a message and returns a channel for the reply.
// The reply channel is buffered with capacity 1.
func (r *RemoteSessionRef) Ask(msg any) chan any {
	reply := make(chan any, 1)
	if r.stopped.Load() {
		reply <- nil
		return reply
	}

	var ipcMsg any
	switch m := msg.(type) {
	case ActionMsg:
		ipcMsg = IPCActionMsg(m)
	case GetWindowView:
		ipcMsg = IPCGetWindowView{}
	case GetActiveWindow:
		ipcMsg = IPCGetActiveWindow{}
	case DetachSession:
		ipcMsg = IPCDetachSession{}
	case ExecuteCommandMsg:
		ipcMsg = IPCExecuteCommandMsg(m)
	case GetFullSessionSnapshot:
		if r.logger != nil {
			r.logger.Warnf("remote: GetFullSessionSnapshot not supported for remote clients")
		}
		reply <- nil
		return reply
	default:
		if r.logger != nil {
			r.logger.Warnf("remote: unsupported ask type %T", msg)
		}
		reply <- nil
		return reply
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		type response struct {
			result any
			err    error
		}
		respCh := make(chan response, 1)
		go func() {
			result, err := r.client.Ask(ipcMsg)
			respCh <- response{result: result, err: err}
		}()

		var result any
		var err error
		select {
		case <-ctx.Done():
			err = fmt.Errorf("ask timeout")
		case resp := <-respCh:
			result = resp.result
			err = resp.err
		}
		if err != nil {
			if r.logger != nil {
				r.logger.Warnf("remote: ask failed: %v", err)
			}
			reply <- nil
			return
		}

		switch v := result.(type) {
		case IPCWindowView:
			reply <- WindowView(v)
		case IPCActionResult:
			var askErr error
			if v.Error != "" {
				askErr = fmt.Errorf("%s", v.Error)
			}
			reply <- ActionResult{Quit: v.Quit, Err: askErr}
		case IPCCommandResult:
			reply <- CommandResult{Success: v.Success, Error: v.Error, Quit: v.Quit}
		case IPCSessionDetached:
			reply <- nil
		case bool:
			if v {
				reply <- true
			} else {
				reply <- nil
			}
		default:
			reply <- result
		}
	}()

	return reply
}

// Stop signals the remote session to stop (same as Shutdown for remote).
func (r *RemoteSessionRef) Stop() {
	r.Shutdown()
}

// Shutdown disconnects from the remote session owner.
// Note: This does NOT kill the owner - it just detaches this client.
func (r *RemoteSessionRef) Shutdown() {
	if !r.stopped.CompareAndSwap(false, true) {
		return
	}

	_ = r.updateClient.Send(IPCUnsubscribeUpdates{})
	close(r.stop)
	r.updateClient.Stop()
	r.client.Stop()
	r.wg.Wait()

	if r.logger != nil {
		r.logger.Infof("remote: session=%s disconnected", r.sessionName)
	}
}

// updateLoop handles local subscriber notifications.
func (r *RemoteSessionRef) updateLoop() {
	defer r.wg.Done()
	for {
		select {
		case <-r.stop:
			return
		case msg := <-r.updates:
			r.notifySubscribers(msg)
		}
	}
}

// handleUpdate processes messages from the owner and forwards to subscribers.
func (r *RemoteSessionRef) handleUpdate(msg any) {
	var localMsg any
	switch m := msg.(type) {
	case IPCWindowView:
		localMsg = WindowView(m)
	case IPCPaneContentUpdated:
		localMsg = PaneContentUpdated(m)
	case IPCSessionEmpty:
		localMsg = SessionEmpty(m)
	case IPCSessionDetached, IPCSessionKilled:
		localMsg = SessionEmpty{ID: 0}
	default:
		localMsg = msg
	}

	select {
	case r.updates <- localMsg:
	default:
	}
}

// notifySubscribers sends updates to all registered subscribers.
func (r *RemoteSessionRef) notifySubscribers(msg any) {
	r.mu.RLock()
	subscribers := make([]chan any, 0, len(r.subscribers))
	for ch := range r.subscribers {
		subscribers = append(subscribers, ch)
	}
	r.mu.RUnlock()

	for _, ch := range subscribers {
		select {
		case ch <- msg:
		default:
		}
	}
}

// SessionName returns the name of the remote session.
func (r *RemoteSessionRef) SessionName() string {
	return r.sessionName
}
