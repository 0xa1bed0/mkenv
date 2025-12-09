package protocol

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/0xa1bed0/mkenv/internal/logs"
)

// WriteProxyHeader writes "PORT <port>\n" to w.
func WriteProxyHeader(w io.Writer, port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	_, err := fmt.Fprintf(w, "PORT %d\n", port)
	return err
}

// ReadProxyHeader reads "PORT <port>\n" from r and returns the parsed port.
func ReadProxyHeader(r *bufio.Reader) (int, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("read proxy header: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 2 || strings.ToUpper(fields[0]) != "PORT" {
		return 0, fmt.Errorf("invalid proxy header %q", line)
	}

	port, err := strconv.Atoi(fields[1])
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port %q", fields[1])
	}

	return port, nil
}

// PumpBidirectional copies bytes both ways between a and b until both sides close.
func PumpBidirectional(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// a -> b
	go func() {
		defer wg.Done()
		copyOneWay(b, a)
	}()

	// b -> a
	go func() {
		defer wg.Done()
		copyOneWay(a, b)
	}()

	// Wait until both directions finished, then fully close.
	wg.Wait()
	_ = a.Close()
	_ = b.Close()
}

// copyOneWay copies src -> dst, then half-closes dst's write side.
// If the other goroutine is still copying the opposite direction,
// its io.Copy will eventually hit EOF / error and cleanly finish.
func copyOneWay(dst, src net.Conn) {
	_, err := io.Copy(dst, src)
	if err != nil && !isNormalCloseErr(err) {
		log.Printf("copy %s -> %s error: %v", src.RemoteAddr(), dst.RemoteAddr(), err)
	}

	// Important: half-close write side, not full close, so the
	// other direction can still use the connection for a bit.
	if tcp, ok := dst.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	} else {
		_ = dst.Close()
	}
}

func isNormalCloseErr(err error) bool {
	logs.Infof("got error: %v", err)
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	msg := err.Error()
	// These are common, non-fatal shutdown cases.
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "closed by the remote host") ||
		strings.Contains(msg, "reset by peer") ||
		strings.Contains(msg, "broken pipe")
}
