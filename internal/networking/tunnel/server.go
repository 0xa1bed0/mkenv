package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
)

// BackendDialer decides how to connect to the real target.
type BackendDialer func(ctx context.Context, port int) (net.Conn, error)

// Serve listens on addr and handles tunnel connections until ctx is done.
func Serve(ctx context.Context, listenAddr string, dial BackendDialer) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}
	defer ln.Close()

	log.Printf("[tunnel] listening on %s", listenAddr)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}
		go handleServerConn(ctx, conn, dial)
	}
}

func handleServerConn(ctx context.Context, client net.Conn, dial BackendDialer) {
	defer client.Close()

	r := bufio.NewReader(client)

	port, err := ReadHeader(r)
	if err != nil {
		log.Printf("[tunnel] server: bad header from %s: %v", client.RemoteAddr(), err)
		return
	}

	backend, err := dial(ctx, port)
	if err != nil {
		log.Printf("[tunnel] server: dial backend %d: %v", port, err)
		return
	}
	defer backend.Close()

	// Important: upstream read continues from r so we don't lose buffered data.
	PumpBidirectional(struct {
		io.Reader
		io.Writer
	}{Reader: r, Writer: client}, backend)
}
