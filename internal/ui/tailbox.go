package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// internal tail state.
type tailState struct {
	name          string
	buf           []string
	lastBoxHeight int
	closed        bool
}

type tailHandle struct {
	ui         *Logger
	iowritebuf []byte
}

// Tail is a handle for writing into a tail box.
type Tail interface {
	// implement io.Writer
	Write([]byte) (int, error)
	Println(msg string)
	Printf(msg string, args ...any)
	Close()
}

// NewTail starts a new tail stream.
// If a previous tail exists, it is finalized into a static box before starting a new one.
func (l *Logger) NewTail(name string) Tail {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.tail != nil && !l.tail.closed {
		l.finalizeTailLocked()
	}

	l.tail = &tailState{
		name: name,
		buf:  make([]string, 0, l.tailLines),
	}

	if l.full != nil {
		fmt.Fprintf(l.full, "[Tail %s] start\n", name)
	}

	return &tailHandle{ui: l}
}

func (t *tailHandle) Write(p []byte) (int, error) {
	l := t.ui
	l.mu.Lock()
	defer l.mu.Unlock()

	// Accumulate chunks.
	t.iowritebuf = append(t.iowritebuf, p...)

	for {
		i := bytes.IndexByte(t.iowritebuf, '\n')
		if i == -1 {
			break
		}

		line := t.iowritebuf[:i]
		t.iowritebuf = t.iowritebuf[i+1:]

		// Trim optional CR before LF (Windows newlines / docker progress).
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		t.printLocked(string(line))
	}

	return len(p), nil
}

func (t *tailHandle) Printf(msg string, args ...any) {
	t.Println(fmt.Sprintf(msg, args...))
}

func (t *tailHandle) Println(msg string) {
	t.Print(msg)
}

func (t *tailHandle) Print(msg string) {
	l := t.ui
	l.mu.Lock()
	defer l.mu.Unlock()

	t.printLocked(msg)
}

func terminalWidth() int {
	ws := struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}{}

	// Try TIOCGWINSZ on stdout.
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if err == 0 && ws.Col > 0 {
		return int(ws.Col)
	}

	// Fallback
	return 120
}

// printLocked is your old Print body, but assumes l.mu is already held.
func (t *tailHandle) printLocked(msg string) {
	l := t.ui
	// TODO: significantly simplify this LLM slop. This whole tailbox thing is garbage.
	// Apperantly LLMs won't take out jobs in the nearest future :(
	truncatedMsg := msg
	widthLimit := terminalWidth() - 20
	if len(msg) > widthLimit {
		truncatedPrefix := "... [truncated]"
		truncatedMsg = msg[0:widthLimit-len(truncatedPrefix)] + truncatedPrefix
	}

	if len(msg) < widthLimit {
		truncatedMsg = msg + strings.Repeat(" ", widthLimit-len(msg))
	}

	// No active tail – log plainly.
	if l.tail == nil || l.tail.closed {
		if l.full != nil {
			fmt.Fprintf(l.full, "[TAIL] %s", msg)
		}
		fmt.Fprint(l.out, truncatedMsg)
		return
	}

	// Append to tail buffer and trim to last N lines.
	l.tail.buf = append(l.tail.buf, truncatedMsg)
	if len(l.tail.buf) > l.tailLines {
		l.tail.buf = l.tail.buf[len(l.tail.buf)-l.tailLines:]
	}

	// Full log gets every tail line.
	if l.full != nil {
		fmt.Fprintf(l.full, "[TAIL %s] %s", l.tail.name, msg)
	}

	if !l.enableTail {
		// Tailbox disabled: just print line-by-line.
		fmt.Fprint(l.out, msg)
		return
	}

	// Clear previous live box if present.
	if l.tail.lastBoxHeight > 0 {
		l.clearTailBoxLocked()
	}

	// Draw new box with updated last N lines.
	l.drawTailBoxLocked()
}

func (t *tailHandle) Close() {
	l := t.ui
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.tail == nil || l.tail.closed {
		return
	}
	l.finalizeTailLocked()
}

// TailWriter returns a writer that feeds data into a new Tail.
func (l *Logger) TailWriter(name string) io.WriteCloser {
	tail := l.NewTail(name)
	pr, pw := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			tail.Println(scanner.Text())
		}
		_ = pr.Close()
		tail.Close()
	}()

	return pw
}

//
// Tail rendering helpers (ANSI + Lipgloss)
//

// renderTailBox builds the box string for the given tail buffer.
func renderTailBox(title string, lines []string, s styles) string {
	if title == "" {
		title = "tail"
	}
	titleLine := s.tailTitle.Render(title)

	var content string
	if len(lines) > 0 {
		content = strings.Join(lines, "\n")
	}

	inner := titleLine
	if content != "" {
		inner = inner + "\n" + content
	}
	return s.tailBox.Render(inner)
}

// clearTailBoxLocked clears the last drawn tail box from the terminal.
// assumes l.mu is held.
func (l *Logger) clearTailBoxLocked() {
	if l.tail == nil || l.tail.lastBoxHeight <= 0 {
		return
	}
	h := l.tail.lastBoxHeight

	// Move cursor up h lines.
	fmt.Fprintf(l.out, "\x1b[%dF", h)

	// Clear h lines.
	for range h {
		fmt.Fprint(l.out, "\x1b[2K\r\n")
	}

	// Move back up to top of cleared area.
	fmt.Fprintf(l.out, "\x1b[%dF", h)

	l.tail.lastBoxHeight = 0
}

// drawTailBoxLocked prints the tail box at the current cursor position.
// assumes l.mu is held.
func (l *Logger) drawTailBoxLocked() {
	if l.tail == nil || len(l.tail.buf) == 0 {
		return
	}
	box := renderTailBox(l.tail.name, l.tail.buf, l.style)

	// Count lines in box (for later clearing).
	height := strings.Count(box, "\n") + 1

	fmt.Fprint(l.out, box+"\n")
	l.tail.lastBoxHeight = height
}

// finalizeTailLocked clears any live box, prints a static tail box into logs,
// and marks the tail as closed. assumes l.mu is held.
func (l *Logger) finalizeTailLocked() {
	if l.tail == nil || l.tail.closed {
		return
	}

	// Clear live box if present.
	if l.enableTail && l.tail.lastBoxHeight > 0 {
		l.clearTailBoxLocked()
	}

	// Print static box with last N lines so final state remains visible.
	if len(l.tail.buf) > 0 {
		box := renderTailBox(l.tail.name, l.tail.buf, l.style)
		fmt.Fprint(l.out, box+"\n")

		// Optionally mirror snapshot to full log in plain form.
		if l.full != nil {
			fmt.Fprintf(l.full, "=== tail %s ===\n", l.tail.name)
			for _, line := range l.tail.buf {
				fmt.Fprintln(l.full, line)
			}
			fmt.Fprintln(l.full)
		}
	}

	if l.full != nil {
		fmt.Fprintf(l.full, "[Tail %s] end\n", l.tail.name)
	}

	l.tail.closed = true
	l.tail = nil
}
