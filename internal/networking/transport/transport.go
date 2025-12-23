package transport

import (
	"context"
	"net"
)

// Server represents a running TCP listener with a done channel.
type Server struct {
	Listener net.Listener
	Cancel   context.CancelFunc
}
