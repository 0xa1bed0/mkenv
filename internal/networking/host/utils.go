package host

import (
	"fmt"
	"net"
)

type PortReservation struct {
	Port  int
	Err   error
	Claim func() error
}

func ReserveFreeTCPPort() *PortReservation {
	var reservation PortReservation

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		errf := fmt.Errorf("FindFreeTCPPort: %w", err)
		reservation.Err = errf
		reservation.Claim = func() error { return errf }
		return &reservation
	}

	addr := l.Addr().(*net.TCPAddr)

	reservation.Port = addr.Port
	reservation.Claim = func() error { return l.Close() }

	return &reservation
}
