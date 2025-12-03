package tunnel

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// WriteHeader sends "PORT <port>\n" to w.
func WriteHeader(w io.Writer, port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	_, err := fmt.Fprintf(w, "PORT %d\n", port)
	return err
}

// ReadHeader reads "PORT <port>\n" from r and returns the parsed port.
// It expects to be called on a fresh connection (i.e. first bytes).
func ReadHeader(r *bufio.Reader) (int, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("read header: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 2 || strings.ToUpper(fields[0]) != "PORT" {
		return 0, fmt.Errorf("invalid header %q", line)
	}

	port, err := strconv.Atoi(fields[1])
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port %q", fields[1])
	}

	return port, nil
}

// PumpBidirectional copies bytes both ways and returns when both sides close.
func PumpBidirectional(a, b io.ReadWriter) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
	}()

	wg.Wait()
}
