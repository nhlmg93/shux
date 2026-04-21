package shux

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// IPCClient connects to a session owner's IPC server.
type IPCClient struct {
	conn    *IPCConn
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
