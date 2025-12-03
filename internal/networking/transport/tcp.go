package transport

import (
	"context"
	"net"
	"time"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

// TCPServer represents a running TCP listener with a done channel.
type TCPServer struct {
	Listener net.Listener
	Done     <-chan struct{}
}

// ServeTCP listens on addr and calls onConn for each accepted connection.
// Closing the Listener will stop the loop and close Done.
func ServeTCP(rt *runtime.Runtime, addr string, onConn func(net.Conn)) (*TCPServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	done := make(chan struct{})

	rt.Go(func() {
		defer close(done)
		logs.Debugf("tcp protocol. start accepting connections")
		for {
			conn, err := ln.Accept()
			if err != nil {
				// this should be normal because we close connection concurently somewhere,
				// so there is a huge chance we accept on closed connection. We will log it just in case and exit
				logs.Debugf("tcp protocol: err while accept: %v", err)
				return
			}
			logs.Debugf("tcp protocol: handling connection")
			rt.Go(func() {
				onConn(conn)
			})
		}
	})

	return &TCPServer{
		Listener: ln,
		Done:     done,
	}, nil
}

// ListenTCP is a thin helper for control-plane servers that want to own their accept loop.
func ListenTCP(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// DialTCP dials addr with retries until ctx is done or the connection succeeds.
func DialTCP(ctx context.Context, addr string, attempts int, delayBetweenAttempts time.Duration) (net.Conn, error) {
	if attempts <= 0 {
		attempts = 1
	}
	if delayBetweenAttempts <= 0 {
		delayBetweenAttempts = 50 * time.Millisecond
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		dialer := &net.Dialer{}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			return conn, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delayBetweenAttempts):
		}
	}
	return nil, lastErr
}
