package shux

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

// IPCServer serves IPC connections for the session owner.
type IPCServer struct {
	listener    net.Listener
	socketPath  string
	handler     func(msg any, reply func(any))
	updates     chan any
	mu          sync.RWMutex
	clients     map[*IPCConn]struct{}
	subscribers map[*IPCConn]struct{}
	stop        chan struct{}
	wg          sync.WaitGroup
	logger      ShuxLogger
}

// NewIPCServer creates a new IPC server bound to the given socket path.
func NewIPCServer(socketPath string, logger ShuxLogger) (*IPCServer, error) {
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", socketPath, err)
	}

	return &IPCServer{
		listener:    listener,
		socketPath:  socketPath,
		clients:     make(map[*IPCConn]struct{}),
		subscribers: make(map[*IPCConn]struct{}),
		updates:     make(chan any, 32),
		stop:        make(chan struct{}),
		logger:      logger,
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
	_ = s.listener.Close()

	s.mu.Lock()
	for client := range s.clients {
		_ = client.Close()
	}
	s.clients = make(map[*IPCConn]struct{})
	s.subscribers = make(map[*IPCConn]struct{})
	s.mu.Unlock()

	s.wg.Wait()
	_ = os.Remove(s.socketPath)
}

// BroadcastUpdate sends an update message to subscribed clients only.
func (s *IPCServer) BroadcastUpdate(msg any) {
	s.mu.RLock()
	clients := make([]*IPCConn, 0, len(s.subscribers))
	for c := range s.subscribers {
		clients = append(clients, c)
	}
	s.mu.RUnlock()

	for _, c := range clients {
		if err := c.Send(msg); err != nil {
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
		_ = ipcConn.Close()
	}()

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

		switch msg.(type) {
		case IPCSubscribeUpdates:
			s.mu.Lock()
			s.subscribers[ipcConn] = struct{}{}
			s.mu.Unlock()
			continue
		case IPCUnsubscribeUpdates:
			s.mu.Lock()
			delete(s.subscribers, ipcConn)
			s.mu.Unlock()
			continue
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
	delete(s.subscribers, c)
	s.mu.Unlock()
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
	_ = c.conn.Close()
	c.wg.Wait()
}
