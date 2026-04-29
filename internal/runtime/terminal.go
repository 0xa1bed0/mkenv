package runtime

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/moby/term" // same package you already use
	"golang.org/x/sys/unix"
)

type TerminalGuard struct {
	mu         sync.Mutex
	inFd       uintptr
	oldState   *term.State
	paused     bool
	resizeCh   chan os.Signal
	resizeDone chan struct{}
	resizeWg   sync.WaitGroup
	stdinGate  *stdinGate
}

// stdinGate is an io.Reader that wraps os.Stdin but can be paused.
// When paused, Read blocks until resumed — this prevents the container's
// stdin pump from consuming bytes meant for pinentry.
//
// It uses unix.Poll with a short timeout instead of a blocking read,
// so it can check the paused flag between poll cycles. This ensures
// that no bytes are consumed from stdin while paused.
type stdinGate struct {
	fd     int32
	mu     sync.Mutex
	cond   *sync.Cond
	paus   bool
	closed bool
}

func newStdinGate() *stdinGate {
	g := &stdinGate{fd: int32(os.Stdin.Fd())}
	g.cond = sync.NewCond(&g.mu)
	return g
}

func (g *stdinGate) Read(p []byte) (int, error) {
	for {
		g.mu.Lock()
		if g.closed {
			g.mu.Unlock()
			return 0, io.EOF
		}
		for g.paus {
			g.cond.Wait()
			if g.closed {
				g.mu.Unlock()
				return 0, io.EOF
			}
		}
		g.mu.Unlock()

		// Poll stdin with a 50ms timeout so we can re-check the paused flag.
		fds := []unix.PollFd{{Fd: g.fd, Events: unix.POLLIN}}
		n, err := unix.Poll(fds, 50)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return 0, err
		}
		if n == 0 {
			continue // timeout — loop back and check paused flag
		}

		// Data is available, do the actual read via os.Stdin.
		return os.Stdin.Read(p)
	}
}

func (g *stdinGate) pause() {
	g.mu.Lock()
	g.paus = true
	g.mu.Unlock()
}

func (g *stdinGate) resume() {
	g.mu.Lock()
	g.paus = false
	g.cond.Broadcast()
	g.mu.Unlock()
}

func (g *stdinGate) close() {
	g.mu.Lock()
	g.closed = true
	g.cond.Broadcast()
	g.mu.Unlock()
}

// NewTerminalGuard creates an empty guard.
func NewTerminalGuard() *TerminalGuard {
	return &TerminalGuard{
		stdinGate: newStdinGate(),
	}
}

// StdinReader returns an io.Reader that wraps os.Stdin but pauses when
// the terminal is in paused mode. Use this for the container stdin pump
// so that pinentry can read from the real stdin during PIN prompts.
func (g *TerminalGuard) StdinReader() io.Reader {
	return g.stdinGate
}

// EnterRawAndWatch puts the terminal into raw mode (if stdin is a TTY) and
// optionally watches for SIGWINCH to call onResize(width, height).
//
// onResize will also be called once immediately with the current size,
// if possible.
func (g *TerminalGuard) EnterRawAndWatch(onResize func(width, height uint)) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Already active? Do nothing.
	if g.oldState != nil {
		return nil
	}

	inFd, isTerm := term.GetFdInfo(os.Stdin)
	if !isTerm {
		// Not a TTY, nothing to do.
		return nil
	}

	st, err := term.MakeRaw(inFd)
	if err != nil {
		return err
	}

	g.inFd = inFd
	g.oldState = st

	if onResize != nil {
		g.resizeCh = make(chan os.Signal, 1)
		g.resizeDone = make(chan struct{})

		signal.Notify(g.resizeCh, syscall.SIGWINCH)

		g.resizeWg.Add(1)
		go func(fd uintptr) {
			defer g.resizeWg.Done()
			for {
				select {
				case <-g.resizeDone:
					return
				case <-g.resizeCh:
					if ws, err := term.GetWinsize(fd); err == nil {
						onResize(uint(ws.Width), uint(ws.Height))
					}
				}
			}
		}(inFd)

		// Initial resize
		if ws, err := term.GetWinsize(inFd); err == nil {
			onResize(uint(ws.Width), uint(ws.Height))
		}
	}

	return nil
}

func (g *TerminalGuard) Size() (width uint, height uint, err error) {
	ws, err := term.GetWinsize(g.inFd)
	if err != nil {
		return
	}
	width = uint(ws.Width)
	height = uint(ws.Height)
	return
}

// PauseRaw temporarily exits raw mode but preserves all state so ResumeRaw
// can re-enter it. The resize watcher keeps running. No-op if not in raw mode
// or already paused.
func (g *TerminalGuard) PauseRaw() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.oldState == nil || g.paused {
		return
	}
	g.stdinGate.pause()
	_ = term.RestoreTerminal(g.inFd, g.oldState)
	g.paused = true
}

// ResumeRaw re-enters raw mode after a PauseRaw. No-op if not paused.
func (g *TerminalGuard) ResumeRaw() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.paused {
		return
	}
	// MakeRaw returns a new saved state, but we keep our original oldState
	// so that the final Restore() still restores to the pre-raw-mode state.
	_, _ = term.MakeRaw(g.inFd)
	g.paused = false
	g.stdinGate.resume()
}

// Restore resets the terminal to its previous state and stops resize watching.
// Safe to call multiple times.
func (g *TerminalGuard) Restore() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.paused = false
	g.stdinGate.close()

	if g.oldState != nil {
		_ = term.RestoreTerminal(g.inFd, g.oldState)
		g.oldState = nil
	}

	if g.resizeCh != nil {
		signal.Stop(g.resizeCh)
		close(g.resizeDone)
		g.resizeCh = nil
	}

	// Wait for resize goroutine to exit (outside lock to avoid deadlocks).
	g.mu.Unlock()
	g.resizeWg.Wait()
	g.mu.Lock()

	g.inFd = 0

	// Best effort: turn off common mouse tracking modes.
	// Ignore errors; it's just writing escape sequences.
	os.Stdout.Write([]byte("\x1b[?1000l")) // X10/normal mouse
	os.Stdout.Write([]byte("\x1b[?1002l")) // button event mouse
	os.Stdout.Write([]byte("\x1b[?1003l")) // any event mouse
	os.Stdout.Write([]byte("\x1b[?1006l")) // SGR mouse mode
}
