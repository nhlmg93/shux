package shux

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

func init() {
	// Register all IPC types with gob
	gob.Register(IPCActionMsg{})
	gob.Register(IPCKeyInput{})
	gob.Register(IPCMouseInput{})
	gob.Register(IPCWriteToPane{})
	gob.Register(IPCResizeMsg{})
	gob.Register(IPCSubscribeUpdates{})
	gob.Register(IPCUnsubscribeUpdates{})
	gob.Register(IPCGetWindowView{})
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
	// Register snapshot types for IPC
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

// IPCServer serves IPC connections for the session owner.
type IPCServer struct {
	listener   net.Listener
	socketPath string
	handler    func(msg any, reply func(any))
	updates    chan any
	mu         sync.RWMutex
	clients    map[*IPCConn]struct{}
	stop       chan struct{}
	wg         sync.WaitGroup
	logger     ShuxLogger
}

// NewIPCServer creates a new IPC server bound to the given socket path.
func NewIPCServer(socketPath string, logger ShuxLogger) (*IPCServer, error) {
	// Remove any stale socket file
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", socketPath, err)
	}

	return &IPCServer{
		listener:   listener,
		socketPath: socketPath,
		clients:    make(map[*IPCConn]struct{}),
		updates:    make(chan any, 32),
		stop:       make(chan struct{}),
		logger:     logger,
	}, nil
}

// Start begins accepting connections.
func (s *IPCServer) Start(handler func(msg any, reply func(any))) {
	s.handler = handler
	s.wg.Add(1)
	go s.acceptLoop()
}

// Stop shuts down the server and closes all connections.
func (s *IPCServer) Stop() {
	close(s.stop)
	s.listener.Close()

	s.mu.Lock()
	for client := range s.clients {
		client.Close()
	}
	s.clients = make(map[*IPCConn]struct{})
	s.mu.Unlock()

	s.wg.Wait()
	_ = os.Remove(s.socketPath)
}

// BroadcastUpdate sends an update message to all connected clients.
func (s *IPCServer) BroadcastUpdate(msg any) {
	s.mu.RLock()
	clients := make([]*IPCConn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.RUnlock()

	for _, c := range clients {
		if err := c.Send(msg); err != nil {
			// Client likely disconnected, remove it
			s.removeClient(c)
		}
	}
}

func (s *IPCServer) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stop:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
				if s.logger != nil {
					s.logger.Warnf("ipc: accept error: %v", err)
				}
				continue
			}
		}

		s.wg.Add(1)
		go s.handleClient(conn)
	}
}

func (s *IPCServer) handleClient(conn net.Conn) {
	defer s.wg.Done()

	ipcConn := NewIPCConn(conn)

	s.mu.Lock()
	s.clients[ipcConn] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.removeClient(ipcConn)
		ipcConn.Close()
	}()

	// Send initial window view
	if s.handler != nil {
		s.handler(IPCGetWindowView{}, func(reply any) {
			if err := ipcConn.Send(reply); err != nil && s.logger != nil {
				s.logger.Warnf("ipc: failed to send initial view: %v", err)
			}
		})
	}

	for {
		select {
		case <-s.stop:
			return
		default:
		}

		msg, err := ipcConn.Receive()
		if err != nil {
			if err != io.EOF && s.logger != nil {
				s.logger.Warnf("ipc: receive error: %v", err)
			}
			return
		}

		if s.handler != nil {
			s.handler(msg, func(reply any) {
				if err := ipcConn.Send(reply); err != nil && s.logger != nil {
					s.logger.Warnf("ipc: send error: %v", err)
				}
			})
		}
	}
}

func (s *IPCServer) removeClient(c *IPCConn) {
	s.mu.Lock()
	delete(s.clients, c)
	s.mu.Unlock()
}

// IPCClient connects to a session owner's IPC server.
type IPCClient struct {
	conn    *IPCConn
	mu      sync.RWMutex
	handler func(any)
	stop    chan struct{}
	wg      sync.WaitGroup
	logger  ShuxLogger
}

// DialIPC connects to an IPC server at the given socket path.
func DialIPC(socketPath string, logger ShuxLogger) (*IPCClient, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", socketPath, err)
	}

	return &IPCClient{
		conn:   NewIPCConn(conn),
		stop:   make(chan struct{}),
		logger: logger,
	}, nil
}

// Send sends a message to the server.
func (c *IPCClient) Send(msg any) error {
	return c.conn.Send(msg)
}

// Ask sends a message and waits for a reply.
func (c *IPCClient) Ask(msg any) (any, error) {
	if err := c.Send(msg); err != nil {
		return nil, err
	}

	reply, err := c.conn.Receive()
	if err != nil {
		return nil, err
	}

	return reply, nil
}

// Start begins handling incoming messages.
func (c *IPCClient) Start(handler func(any)) {
	c.handler = handler
	c.wg.Add(1)
	go c.receiveLoop()
}

// Stop disconnects from the server.
func (c *IPCClient) Stop() {
	close(c.stop)
	c.conn.Close()
	c.wg.Wait()
}

func (c *IPCClient) receiveLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stop:
			return
		default:
		}

		msg, err := c.conn.Receive()
		if err != nil {
			select {
			case <-c.stop:
				return
			default:
				if err != io.EOF && c.logger != nil {
					c.logger.Warnf("ipc client: receive error: %v", err)
				}
				return
			}
		}

		if c.handler != nil {
			c.handler(msg)
		}
	}
}

// WaitForSocket polls until the socket file exists or timeout.
func WaitForSocket(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for socket: %s", socketPath)
}
