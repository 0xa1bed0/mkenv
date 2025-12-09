package protocol

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

var controlMagic = [4]byte{'M', 'K', 'E', 'N'}

const (
	maxControlFrameSize = 1 << 20 // 1MB
)

// ControlSignalEnvelope is the only thing that goes over the control wire.
type ControlSignalEnvelope struct {
	ID   string          `json:"id,omitempty"`
	Type string          `json:"type"`
	OK   bool            `json:"ok,omitempty"`
	Err  string          `json:"err,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// PackControlSignalEnvelope encodes typed payload into an Envelope.
func PackControlSignalEnvelope[T any](id, typ string, v T) (ControlSignalEnvelope, error) {
	envelope := ControlSignalEnvelope{
		ID:   id,
		Type: typ,
	}
	if any(v) != nil {
		b, err := json.Marshal(v)
		if err != nil {
			return ControlSignalEnvelope{}, err
		}
		envelope.Data = b
	}
	return envelope, nil
}

// UnpackControlSignalEnvelope decodes Envelope.Data into out.
func UnpackControlSignalEnvelope[T any](env ControlSignalEnvelope, out *T) error {
	if len(env.Data) == 0 {
		return errors.New("empty data")
	}
	dec := json.NewDecoder(bytes.NewReader(env.Data))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b[:])
}

// ControlConn is a framed, multiplexed control-plane connection.
type ControlConn struct {
	raw net.Conn
	br  *bufio.Reader
	bw  *bufio.Writer

	closed atomic.Bool

	// pending RPC calls
	muPending sync.Mutex
	pending   map[string]chan ControlSignalEnvelope

	// push subscribers
	muSubs sync.RWMutex
	subs   map[string][]chan ControlSignalEnvelope

	// generic handler (server-side)
	onMessage func(ControlSignalEnvelope)

	// read-loop error
	readErr atomic.Value // error
}

// NewControlConn wraps a net.Conn and starts the read loop.
func NewControlConn(raw net.Conn) *ControlConn {
	c := &ControlConn{
		raw:     raw,
		br:      bufio.NewReader(raw),
		bw:      bufio.NewWriter(raw),
		pending: make(map[string]chan ControlSignalEnvelope),
		subs:    make(map[string][]chan ControlSignalEnvelope),
	}
	go c.readLoop()
	return c
}

func (c *ControlConn) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	_ = c.raw.Close()

	c.muPending.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.muPending.Unlock()
	return nil
}

func (c *ControlConn) Err() error {
	v := c.readErr.Load()
	if v == nil {
		return nil
	}
	return v.(error)
}

// Send writes a single envelope (no RPC semantics).
func (c *ControlConn) Send(env ControlSignalEnvelope) error {
	if c.closed.Load() {
		return io.ErrClosedPipe
	}

	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if len(b) == 0 || len(b) > maxControlFrameSize {
		return fmt.Errorf("control payload size %d out of bounds", len(b))
	}

	// magic + length + payload
	var hdr [8]byte
	copy(hdr[:4], controlMagic[:])
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(b)))

	c.muPending.Lock() // reuse this lock as a simple send mutex
	defer c.muPending.Unlock()

	if _, err := c.bw.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := c.bw.Write(b); err != nil {
		return err
	}
	return c.bw.Flush()
}

// Call sends a request envelope and waits for the response with same ID.
func (c *ControlConn) Call(ctx context.Context, req ControlSignalEnvelope) (ControlSignalEnvelope, error) {
	if req.ID == "" {
		return ControlSignalEnvelope{}, errors.New("control Call requires req.ID")
	}

	ch := make(chan ControlSignalEnvelope, 1)

	c.muPending.Lock()
	if _, exists := c.pending[req.ID]; exists {
		c.muPending.Unlock()
		return ControlSignalEnvelope{}, fmt.Errorf("duplicate pending id %s", req.ID)
	}
	c.pending[req.ID] = ch
	c.muPending.Unlock()

	if err := c.Send(req); err != nil {
		c.muPending.Lock()
		delete(c.pending, req.ID)
		c.muPending.Unlock()
		return ControlSignalEnvelope{}, err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return ControlSignalEnvelope{}, io.ErrClosedPipe
		}
		return resp, nil
	case <-ctx.Done():
		c.muPending.Lock()
		delete(c.pending, req.ID)
		c.muPending.Unlock()
		return ControlSignalEnvelope{}, ctx.Err()
	}
}

// Subscribe returns a channel that receives all pushed envelopes of given Type.
func (c *ControlConn) Subscribe(typ string, buf int) <-chan ControlSignalEnvelope {
	ch := make(chan ControlSignalEnvelope, buf)
	c.muSubs.Lock()
	c.subs[typ] = append(c.subs[typ], ch)
	c.muSubs.Unlock()
	return ch
}

// OnMessage installs a generic handler (server-side) invoked for every message
// that isn't a direct RPC response.
func (c *ControlConn) OnMessage(fn func(ControlSignalEnvelope)) {
	c.onMessage = fn
}

func (c *ControlConn) readLoop() {
	defer c.Close()

	for {
		env, err := c.recvOne()
		if err != nil {
			c.readErr.Store(err)
			return
		}

		// Response path
		if env.ID != "" {
			c.muPending.Lock()
			ch, ok := c.pending[env.ID]
			if ok {
				delete(c.pending, env.ID)
				c.muPending.Unlock()
				ch <- env
				close(ch)
				continue
			}
			c.muPending.Unlock()
		}

		// Subscribers
		c.muSubs.RLock()
		slist := c.subs[env.Type]
		c.muSubs.RUnlock()
		for _, ch := range slist {
			select {
			case ch <- env:
			default:
			}
		}

		// Generic handler
		if c.onMessage != nil {
			c.onMessage(env)
		}
	}
}

func (c *ControlConn) recvOne() (ControlSignalEnvelope, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(c.br, hdr[:]); err != nil {
		return ControlSignalEnvelope{}, err
	}

	if !bytes.Equal(hdr[:4], controlMagic[:]) {
		return ControlSignalEnvelope{}, fmt.Errorf("bad control magic")
	}

	n := binary.BigEndian.Uint32(hdr[4:])
	if n == 0 || n > maxControlFrameSize {
		return ControlSignalEnvelope{}, fmt.Errorf("invalid control frame size: %d", n)
	}

	payload := make([]byte, n)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		return ControlSignalEnvelope{}, err
	}

	var env ControlSignalEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return ControlSignalEnvelope{}, err
	}
	if env.Type == "" {
		return ControlSignalEnvelope{}, fmt.Errorf("missing type")
	}
	if len(env.ID) > 128 {
		return ControlSignalEnvelope{}, fmt.Errorf("id too long")
	}

	return env, nil
}

// ControlCommandHandler is a control command handler
// TODO: think of generic struct or interface
type ControlCommandHandler func(ctx context.Context, req ControlSignalEnvelope) (any, error)

type ControlServerProtocol struct {
	ln net.Listener

	muAgents sync.RWMutex
	agents   map[*ControlConn]struct{}

	muHandlers sync.RWMutex
	handlers   map[string]ControlCommandHandler

	ctx    context.Context
	cancel context.CancelFunc

	rt *runtime.Runtime
}

func NewControlServerProtocol(rt *runtime.Runtime, ln net.Listener) *ControlServerProtocol {
	ctx, cancel := context.WithCancel(context.Background())
	return &ControlServerProtocol{
		ln:       ln,
		agents:   make(map[*ControlConn]struct{}),
		handlers: make(map[string]ControlCommandHandler),
		ctx:      ctx,
		cancel:   cancel,
		rt:       rt,
	}
}

func (s *ControlServerProtocol) Close() error {
	s.cancel()
	_ = s.ln.Close()

	s.muAgents.Lock()
	for c := range s.agents {
		c.Close()
	}
	s.agents = map[*ControlConn]struct{}{}
	s.muAgents.Unlock()
	return nil
}

func (s *ControlServerProtocol) Handle(typ string, h ControlCommandHandler) {
	s.muHandlers.Lock()
	logs.Debugf("handle %s", typ)
	s.handlers[typ] = h
	s.muHandlers.Unlock()
}

func (s *ControlServerProtocol) Serve() error {
	for {
		raw, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				return err
			}
		}

		conn := NewControlConn(raw)

		s.muAgents.Lock()
		s.agents[conn] = struct{}{}
		s.muAgents.Unlock()

		conn.OnMessage(func(env ControlSignalEnvelope) {
			s.rt.GoNamed("ControlServer:DispatchEnvelope", func() {
				s.dispatch(conn, env)
			})
		})
	}
}

func (s *ControlServerProtocol) dispatch(c *ControlConn, env ControlSignalEnvelope) {
	s.muHandlers.RLock()
	logs.Debugf("handlers: %v", s.handlers)
	logs.Debugf("type: %s", env.Type)
	h := s.handlers[env.Type]
	s.muHandlers.RUnlock()
	if h == nil {
		_ = c.Send(ControlSignalEnvelope{
			ID:   env.ID,
			Type: env.Type + ".resp",
			OK:   false,
			Err:  fmt.Sprintf("unknown type %q", env.Type),
		})
		return
	}

	// TODO: make configurable
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	response, err := h(ctx, env)
	if err != nil {
		_ = c.Send(ControlSignalEnvelope{
			ID:   env.ID,
			Type: env.Type + ".resp",
			OK:   false,
			Err:  err.Error(),
		})
	}

	responseEnvelope, err := PackControlSignalEnvelope(env.ID, env.Type+".resp", response)
	_ = c.Send(responseEnvelope)
}

func (s *ControlServerProtocol) Broadcast(env ControlSignalEnvelope) {
	s.muAgents.RLock()
	defer s.muAgents.RUnlock()
	for c := range s.agents {
		_ = c.Send(env)
	}
}

func (s *ControlServerProtocol) Agents() []*ControlConn {
	s.muAgents.RLock()
	defer s.muAgents.RUnlock()
	out := make([]*ControlConn, 0, len(s.agents))
	for c := range s.agents {
		out = append(out, c)
	}
	return out
}
