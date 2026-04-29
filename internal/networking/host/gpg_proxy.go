package host

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/networking/protocol"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

const (
	// SSH agent protocol message types
	sshAgentSignRequest  = 13
	sshAgentSignResponse = 14
	sshAgentFailure      = 5
)

// UpdateGPGStartupTTY runs "gpg-connect-agent updatestartuptty /bye" on the
// host so that the gpg-agent reattaches pinentry to the current terminal.
// This is best-effort — if gpg-connect-agent is not installed or fails, we
// log and continue (the proxy will still work, pinentry just might appear
// on a stale TTY).
func UpdateGPGStartupTTY() {
	cmd := exec.Command("gpg-connect-agent", "updatestartuptty", "/bye")
	cmd.Env = append(os.Environ(), "GPG_TTY="+ttyName())
	if out, err := cmd.CombinedOutput(); err != nil {
		logs.Debugf("gpg-connect-agent updatestartuptty: %v: %s", err, out)
	} else {
		logs.Debugf("gpg-connect-agent updatestartuptty: OK")
	}
}

// ttyName returns the name of the controlling terminal, or empty string.
func ttyName() string {
	// Check GPG_TTY first, then fall back to /proc/self/fd/0 readlink (Linux)
	// or "tty" command.
	if t := os.Getenv("GPG_TTY"); t != "" {
		return t
	}
	if target, err := os.Readlink("/proc/self/fd/0"); err == nil {
		return target
	}
	if out, err := exec.Command("tty").Output(); err == nil {
		return string(out[:len(out)-1]) // trim newline
	}
	return ""
}

// GPGSocketProxy creates proxy Unix sockets on the host and forwards
// connections to the real GPG agent sockets. For the SSH agent socket,
// it parses the SSH agent binary protocol to detect sign requests and
// temporarily exits terminal raw mode so pinentry can render cleanly.
type GPGSocketProxy struct {
	rt        *runtime.Runtime
	term      *runtime.TerminalGuard
	proxyDir  string
	listeners []net.Listener

	rawModeMu    sync.Mutex
	pendingSigns int       // number of sign requests awaiting response
	lastResume   time.Time // cooldown: skip pause if PIN was just entered (likely cached)
	once         sync.Once
}

// StartGPGProxy creates proxy Unix sockets and starts forwarding connections
// to the real GPG agent sockets. baseDir must be a directory that is accessible
// from inside Docker (on macOS, /tmp and $TMPDIR may not be shared with the
// Docker VM — use a path under the user's home directory instead).
func StartGPGProxy(rt *runtime.Runtime, term *runtime.TerminalGuard, gpgSocketPath, sshSocketPath, baseDir string) (*GPGSocketProxy, error) {
	proxyDir, err := os.MkdirTemp(baseDir, "gpg-proxy-*")
	if err != nil {
		return nil, fmt.Errorf("create GPG proxy dir: %w", err)
	}
	// Make dir world-accessible so the container user can reach the sockets.
	if err := os.Chmod(proxyDir, 0777); err != nil {
		return nil, fmt.Errorf("chmod GPG proxy dir: %w", err)
	}

	p := &GPGSocketProxy{
		rt:       rt,
		term:     term,
		proxyDir: proxyDir,
	}

	if gpgSocketPath != "" {
		ln, err := p.listenProxy("S.gpg-agent", gpgSocketPath, p.handleAssuan)
		if err != nil {
			p.Stop()
			return nil, err
		}
		p.listeners = append(p.listeners, ln)
	}

	if sshSocketPath != "" {
		ln, err := p.listenProxy("S.gpg-agent.ssh", sshSocketPath, p.handleSSHAgent)
		if err != nil {
			p.Stop()
			return nil, err
		}
		p.listeners = append(p.listeners, ln)
	}

	rt.OnShutdown(func(_ context.Context) {
		p.Stop()
	})

	logs.Debugf("GPG proxy started in %s", proxyDir)
	return p, nil
}

// ProxyDir returns the temp directory containing the proxy Unix sockets.
func (p *GPGSocketProxy) ProxyDir() string { return p.proxyDir }

// Stop shuts down all listeners and cleans up.
func (p *GPGSocketProxy) Stop() {
	p.once.Do(func() {
		for _, ln := range p.listeners {
			ln.Close()
		}
		os.RemoveAll(p.proxyDir)
		// Ensure raw mode is restored if we're still paused.
		p.rawModeMu.Lock()
		if p.pendingSigns > 0 {
			p.pendingSigns = 0
			p.term.ResumeRaw()
		}
		p.rawModeMu.Unlock()
	})
}

func (p *GPGSocketProxy) listenProxy(name, realSocketPath string, handler func(client, backend net.Conn)) (net.Listener, error) {
	proxyPath := filepath.Join(p.proxyDir, name)
	ln, err := net.Listen("unix", proxyPath)
	if err != nil {
		return nil, fmt.Errorf("listen on proxy socket %s: %w", proxyPath, err)
	}

	// Make socket accessible to the container user
	if err := os.Chmod(proxyPath, 0666); err != nil {
		ln.Close()
		return nil, fmt.Errorf("chmod proxy socket: %w", err)
	}

	routineName := fmt.Sprintf("GPGProxy;%s;accept", name)
	p.rt.GoNamed(routineName, func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if p.rt.Ctx().Err() != nil {
					return
				}
				logs.Debugf("gpg-proxy %s: accept error: %v", name, err)
				return
			}

			p.rt.GoNamed(fmt.Sprintf("GPGProxy;%s;conn", name), func() {
				backend, err := net.Dial("unix", realSocketPath)
				if err != nil {
					logs.Errorf("gpg-proxy: can't dial real socket %s: %v", realSocketPath, err)
					conn.Close()
					return
				}
				handler(conn, backend)
			})
		}
	})

	return ln, nil
}

// handleAssuan does blind bidirectional proxying for the GPG Assuan socket.
func (p *GPGSocketProxy) handleAssuan(client, backend net.Conn) {
	go func() {
		<-p.rt.Ctx().Done()
		client.Close()
		backend.Close()
	}()
	protocol.PumpBidirectional(client, backend)
}

// handleSSHAgent proxies SSH agent protocol, detecting sign requests
// to temporarily exit terminal raw mode for pinentry.
func (p *GPGSocketProxy) handleSSHAgent(client, backend net.Conn) {
	// Close both connections when the runtime context is cancelled,
	// which unblocks any pending reads in proxySSHFrames.
	go func() {
		<-p.rt.Ctx().Done()
		client.Close()
		backend.Close()
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	// client → backend: detect sign requests
	go func() {
		defer wg.Done()
		p.proxySSHFrames(client, backend, true)
	}()

	// backend → client: detect sign responses
	go func() {
		defer wg.Done()
		p.proxySSHFrames(backend, client, false)
	}()

	wg.Wait()
	client.Close()
	backend.Close()
}

func (p *GPGSocketProxy) proxySSHFrames(src, dst net.Conn, isClientToBackend bool) {
	for {
		msgType, frame, err := readSSHAgentFrame(src)
		if err != nil {
			if err != io.EOF {
				logs.Debugf("gpg-proxy: frame read error: %v", err)
			}
			return
		}

		if isClientToBackend && msgType == sshAgentSignRequest {
			logs.Debugf("gpg-proxy: sign request detected, pausing raw mode")
			p.enterPinentryMode()
		}

		_, err = dst.Write(frame)
		if err != nil {
			logs.Debugf("gpg-proxy: frame write error: %v", err)
			return
		}

		if !isClientToBackend && (msgType == sshAgentSignResponse || msgType == sshAgentFailure) {
			if p.isPaused() {
				logs.Debugf("gpg-proxy: sign response received, resuming raw mode")
				p.exitPinentryMode()
			}
		}
	}
}

// readSSHAgentFrame reads one SSH agent protocol frame.
// Format: 4-byte big-endian length + body (first byte of body is message type).
func readSSHAgentFrame(r io.Reader) (msgType byte, frame []byte, err error) {
	var lenBuf [4]byte
	if _, err = io.ReadFull(r, lenBuf[:]); err != nil {
		return 0, nil, err
	}

	bodyLen := binary.BigEndian.Uint32(lenBuf[:])
	if bodyLen == 0 || bodyLen > 256*1024 {
		return 0, nil, fmt.Errorf("invalid SSH agent frame length: %d", bodyLen)
	}

	body := make([]byte, bodyLen)
	if _, err = io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}

	// Build full frame for forwarding
	frame = make([]byte, 4+bodyLen)
	copy(frame[:4], lenBuf[:])
	copy(frame[4:], body)

	return body[0], frame, nil
}

func (p *GPGSocketProxy) enterPinentryMode() {
	p.rawModeMu.Lock()
	defer p.rawModeMu.Unlock()

	// Most users have GPG PIN caching enabled (the default). After a successful
	// PIN entry, subsequent sign requests use the cached PIN without triggering
	// pinentry. Skip pausing raw mode during the cooldown window to avoid
	// unnecessary terminal disruption for these cached requests.
	if !p.lastResume.IsZero() && time.Since(p.lastResume) < 5*time.Second {
		logs.Debugf("gpg-proxy: sign request within cooldown, skipping pause (PIN likely cached)")
		return
	}

	p.pendingSigns++
	if p.pendingSigns == 1 {
		logs.Debugf("gpg-proxy: first sign request, pausing raw mode")
		p.term.PauseRaw()
	}
}

func (p *GPGSocketProxy) exitPinentryMode() {
	p.rawModeMu.Lock()
	defer p.rawModeMu.Unlock()
	if p.pendingSigns <= 0 {
		return
	}
	p.pendingSigns--
	if p.pendingSigns == 0 {
		// All sign requests resolved — resume raw mode.
		logs.Debugf("gpg-proxy: all sign requests done, resuming raw mode")
		p.lastResume = time.Now()
		// Clear screen before resuming so pinentry-curses
		// artifacts don't mix with the container's terminal output.
		os.Stdout.Write([]byte("\033[2J\033[H"))
		p.term.ResumeRaw()
	}
}

func (p *GPGSocketProxy) isPaused() bool {
	p.rawModeMu.Lock()
	defer p.rawModeMu.Unlock()
	return p.pendingSigns > 0
}
