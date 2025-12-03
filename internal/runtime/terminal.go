package runtime

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/moby/term" // same package you already use
)

type TerminalGuard struct {
	mu         sync.Mutex
	inFd       uintptr
	oldState   *term.State
	resizeCh   chan os.Signal
	resizeDone chan struct{}
	resizeWg   sync.WaitGroup
}

// NewTerminalGuard creates an empty guard.
func NewTerminalGuard() *TerminalGuard {
	return &TerminalGuard{}
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

// Restore resets the terminal to its previous state and stops resize watching.
// Safe to call multiple times.
func (g *TerminalGuard) Restore() {
	g.mu.Lock()
	defer g.mu.Unlock()

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
