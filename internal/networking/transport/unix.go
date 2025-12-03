package transport

import (
	"context"
	"net"
	"os"
	"time"
)

// ServeUnix is like ServeTCP but for Unix domain sockets.
func ServeUnix(path string, onConn func(net.Conn)) (*TCPServer, error) {
	_ = os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go onConn(conn)
		}
	}()

	return &TCPServer{
		Listener: ln,
		Done:     done,
	}, nil
}

// ListenUnix is for control-plane servers that want their own accept loop.
func ListenUnix(path string) (net.Listener, error) {
	_ = os.Remove(path)
	return net.Listen("unix", path)
}

// DialUnix dials a Unix socket with retries.
func DialUnix(ctx context.Context, path string, attempts int, delay time.Duration) (net.Conn, error) {
	if attempts <= 0 {
		attempts = 1
	}
	if delay <= 0 {
		delay = 50 * time.Millisecond
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		dialer := &net.Dialer{}
		conn, err := dialer.DialContext(ctx, "unix", path)
		if err == nil {
			return conn, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}
