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

	ref := &RemoteSessionRef{
		client:      client,
		sessionName: sessionName,
		subscribers: make(map[chan any]struct{}),
		updates:     make(chan any, 32),
		stop:        make(chan struct{}),
		logger:      logger,
	}

	// Start the update handler
	ref.wg.Add(1)
	go ref.updateLoop()

	// Start IPC client with handler that forwards to subscribers
	client.Start(func(msg any) {
		ref.handleUpdate(msg)
	})

	// Subscribe to updates from owner
	_ = client.Send(IPCSubscribeUpdates{})

	return ref, nil
}

// Send dispatches a message to the remote session owner.
// Returns false if the connection is closed.
func (r *RemoteSessionRef) Send(msg any) bool {
	if r.stopped.Load() {
		return false
	}

	var ipcMsg any
	switch m := msg.(type) {
	case ActionMsg:
		ipcMsg = IPCActionMsg{
			Action: m.Action,
			Args:   m.Args,
			Amount: m.Amount,
		}
	case KeyInput:
		ipcMsg = IPCKeyInput{
			Code:        m.Code,
			Text:        m.Text,
			ShiftedCode: m.ShiftedCode,
			BaseCode:    m.BaseCode,
			Mods:        m.Mods,
			IsRepeat:    m.IsRepeat,
		}
	case MouseInput:
		ipcMsg = IPCMouseInput{
			Row:    m.Row,
			Col:    m.Col,
			Button: m.Button,
			Mods:   m.Mods,
			Action: m.Action,
		}
	case WriteToPane:
		ipcMsg = IPCWriteToPane{Data: m.Data}
	case ResizeMsg:
		ipcMsg = IPCResizeMsg{Rows: m.Rows, Cols: m.Cols}
	case SubscribeUpdates:
		// Register local subscriber
		r.mu.Lock()
		if m.Subscriber != nil {
			r.subscribers[m.Subscriber] = struct{}{}
		}
		r.mu.Unlock()
		// Forward to owner so it knows to send updates to this client
		if err := r.client.Send(IPCSubscribeUpdates{}); err != nil {
			if r.logger != nil {
				r.logger.Warnf("remote: failed to forward subscribe: %v", err)
			}
			return false
		}
		return true
	case UnsubscribeUpdates:
		// Unregister local subscriber
		r.mu.Lock()
		if m.Subscriber != nil {
			delete(r.subscribers, m.Subscriber)
		}
		r.mu.Unlock()
		// Forward to owner so it stops sending updates to this client
		if err := r.client.Send(IPCUnsubscribeUpdates{}); err != nil {
			if r.logger != nil {
				r.logger.Warnf("remote: failed to forward unsubscribe: %v", err)
			}
			return false
		}
		return true
	case DetachSession:
		ipcMsg = IPCDetachSession{}
	case ExecuteCommandMsg:
		ipcMsg = IPCExecuteCommandMsg{Command: m.Command}
	default:
		// Unknown message type - log and drop
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
		ipcMsg = IPCActionMsg{
			Action: m.Action,
			Args:   m.Args,
			Amount: m.Amount,
		}
	case GetWindowView:
		ipcMsg = IPCGetWindowView{}
	case DetachSession:
		ipcMsg = IPCDetachSession{}
	case ExecuteCommandMsg:
		ipcMsg = IPCExecuteCommandMsg{Command: m.Command}
	case GetFullSessionSnapshot:
		// This is handled directly by the owner, not forwarded over IPC
		if r.logger != nil {
			r.logger.Warnf("remote: GetFullSessionSnapshot not supported for remote clients")
		}
		reply <- nil
		return reply
	default:
		// Unknown message type
		if r.logger != nil {
			r.logger.Warnf("remote: unsupported ask type %T", msg)
		}
		reply <- nil
		return reply
	}

	go func() {
		// Use timeout to prevent goroutine leak if client hangs
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		type response struct {
			result any
			err    error
		}
		respCh := make(chan response, 1)

		go func() {
			result, err := r.client.Ask(ipcMsg)
			respCh <- response{result, err}
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

		// Convert IPC response back to local types
		switch r := result.(type) {
		case IPCWindowView:
			reply <- WindowView{
				Content:   r.Content,
				CursorRow: r.CursorRow,
				CursorCol: r.CursorCol,
				CursorOn:  r.CursorOn,
				Title:     r.Title,
			}
		case IPCActionResult:
			var err error
			if r.Error != "" {
				err = fmt.Errorf("%s", r.Error)
			}
			reply <- ActionResult{Quit: r.Quit, Err: err}
		case IPCCommandResult:
			var err error
			if r.Error != "" {
				err = fmt.Errorf("%s", r.Error)
			}
			reply <- CommandResult{Success: r.Success, Error: err.Error(), Quit: r.Quit}
		case IPCSessionDetached:
			reply <- nil // Detach acknowledged
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
		return // Already stopped
	}

	// Send unsubscribe before closing
	_ = r.client.Send(IPCUnsubscribeUpdates{})

	close(r.stop)
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
	// Convert IPC messages to local equivalents
	var localMsg any
	switch m := msg.(type) {
	case IPCWindowView:
		localMsg = WindowView{
			Content:   m.Content,
			CursorRow: m.CursorRow,
			CursorCol: m.CursorCol,
			CursorOn:  m.CursorOn,
			Title:     m.Title,
		}
	case IPCPaneContentUpdated:
		localMsg = PaneContentUpdated{ID: m.ID}
	case IPCSessionEmpty:
		localMsg = SessionEmpty{ID: m.ID}
	case IPCSessionDetached:
		// Owner acknowledged detach - client should quit
		localMsg = SessionEmpty{ID: 0}
	case IPCSessionKilled:
		// Session was killed - client should quit
		localMsg = SessionEmpty{ID: 0}
	default:
		localMsg = msg
	}

	select {
	case r.updates <- localMsg:
	default:
		// Drop update if channel full
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
			// Drop if subscriber not keeping up
		}
	}
}

// SessionName returns the name of the remote session.
func (r *RemoteSessionRef) SessionName() string {
	return r.sessionName
}
