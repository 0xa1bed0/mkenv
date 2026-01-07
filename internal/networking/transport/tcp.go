package transport

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

// ServeTCP listens on addr and calls onConn for each accepted connection.
// Closing the Listener will stop the loop and close Done.
func ServeTCP(rt *runtime.Runtime, addr string, onConn func(context.Context, net.Conn)) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(rt.Ctx())

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	routineName := fmt.Sprintf("ServeTCP;Accept loop;%s;", addr)
	rt.GoNamed(routineName, func() {
		logs.Debugf("tcp protocol. start accepting connections")
		for {
			conn, err := ln.Accept()
			if err != nil {
				// this should be normal because we close connection concurently somewhere,
				// so there is a huge chance we accept on closed connection. We will log it just in case and exit
				if ctx.Err() != nil {
					// context canceled; listener closed intentionally
					logs.Debugf("accept loop stopped due to context: %v", err)
				} else {
					logs.Debugf("accept error: %v", err)
				}
				return
			}

			logs.Debugf("tcp protocol: handling connection")

			rt.GoNamed(routineName+"onConn", func() {
				logs.Debugf("Transport: call onConn")
				onConn(ctx, conn)
			})
		}
	})

	return &Server{
		Listener: ln,
		Cancel:   cancel,
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
