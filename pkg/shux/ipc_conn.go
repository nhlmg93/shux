package shux

import (
	"encoding/gob"
	"fmt"
	"net"
	"sync"
)

func init() {
	gob.Register(IPCActionMsg{})
	gob.Register(IPCKeyInput{})
	gob.Register(IPCMouseInput{})
	gob.Register(IPCWriteToPane{})
	gob.Register(IPCResizeMsg{})
	gob.Register(IPCCreateWindow{})
	gob.Register(IPCSubscribeUpdates{})
	gob.Register(IPCUnsubscribeUpdates{})
	gob.Register(IPCGetWindowView{})
	gob.Register(IPCGetActiveWindow{})
	gob.Register(IPCExecuteCommandMsg{})
	gob.Register(IPCDetachSession{})
	gob.Register(IPCKillSession{})
	gob.Register(IPCWindowView{})
	gob.Register(IPCPaneContentUpdated{})
	gob.Register(IPCSessionEmpty{})
	gob.Register(IPCSessionDetached{})
	gob.Register(IPCSessionKilled{})
	gob.Register(IPCActionResult{})
	gob.Register(IPCCommandResult{})
	gob.Register(IPCEnvelope{})

	gob.Register(SessionSnapshot{})
	gob.Register(WindowSnapshot{})
	gob.Register(PaneSnapshot{})
	gob.Register(SplitTreeSnapshot{})
}

// IPCConn wraps a net.Conn with length-prefixed gob encoding.
type IPCConn struct {
	conn   net.Conn
	enc    *gob.Encoder
	dec    *gob.Decoder
	mu     sync.Mutex
	closed bool
}

// NewIPCConn creates a new IPC connection wrapper.
func NewIPCConn(conn net.Conn) *IPCConn {
	return &IPCConn{
		conn: conn,
		enc:  gob.NewEncoder(conn),
		dec:  gob.NewDecoder(conn),
	}
}

// Send encodes and sends a message over the connection.
func (c *IPCConn) Send(msg any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("connection closed")
	}

	env := IPCEnvelope{
		Type: fmt.Sprintf("%T", msg),
		Data: msg,
	}

	if err := c.enc.Encode(&env); err != nil {
		return fmt.Errorf("encode failed: %w", err)
	}

	return nil
}

// Receive decodes and returns a message from the connection.
func (c *IPCConn) Receive() (any, error) {
	var env IPCEnvelope
	if err := c.dec.Decode(&env); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}
	return env.Data, nil
}

// Close closes the connection.
func (c *IPCConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

// Send sends a message to the server.
func (c *IPCClient) Send(msg any) error {
	return c.conn.Send(msg)
}
