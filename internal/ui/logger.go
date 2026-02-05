package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// syncer is an interface for types that can sync to disk.
// Both *os.File and *SyncWriter implement this.
type syncer interface {
	Sync() error
}

//
// Public types & options
//

type LogLevel int

const (
	LogLeverError LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelDebug
	LogLevelDebugVerbose
)

// Options configures the Logger.
type Options struct {
	// Out is where we print user-facing logs.
	// In most cases this should be os.Stdout.
	Out io.Writer

	// FullLogWriter, if non-nil, receives all logs and tail lines in plain text.
	FullLogWriter io.Writer

	// TailLines controls how many lines are kept and shown in the live tail box.
	// If <= 0, defaults to 5.
	TailLines int

	// EnableTail controls whether the live tail box is rendered.
	// If false, tail lines are printed as normal logs.
	EnableTail bool

	// LogLevel control amount of logs print to stdout
	// greater the number => more logs coming out
	// error < info < warn < debug < debugVerbose
	// warn level always prints to the full log file unless greater value provided
	LogLevel LogLevel

	// Component identifies the source of log messages (e.g., "host", "agent").
	// If empty, no component tag is included in log output.
	Component string
}

// Logger is the main stdout logger + tail manager.
type Logger struct {
	out       io.Writer
	full      io.Writer
	mu        sync.Mutex
	style     styles
	component string

	logLevel LogLevel

	// fullLogBuffer holds log lines written before full log writer is set.
	// Once the full writer is set, this buffer is flushed and cleared.
	fullLogBuffer []string

	tail       *tailState
	tailLines  int
	enableTail bool
}

func (l *Logger) MuteStdout() (restore func()) {
	l.out = io.Discard
	return func() {
		l.out = os.Stdout
	}
}

// styles for log levels and boxes.
type styles struct {
	spacer    lipgloss.Style
	logInfo   lipgloss.Style
	logWarn   lipgloss.Style
	logError  lipgloss.Style
	banner    lipgloss.Style
	tailBox   lipgloss.Style
	tailTitle lipgloss.Style
}

func defaultStyles() styles {
	return styles{
		spacer:    lipgloss.NewStyle(),
		logInfo:   lipgloss.NewStyle(),
		logWarn:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")), // orange-ish
		logError:  lipgloss.NewStyle().Foreground(lipgloss.Color("196")), // red
		banner:    lipgloss.NewStyle().Bold(true).Border(lipgloss.NormalBorder()).Padding(0, 1).Margin(1, 0),
		tailBox:   lipgloss.NewStyle().Bold(true).Border(lipgloss.NormalBorder()).Padding(0, 1).Margin(1, 0),
		tailTitle: lipgloss.NewStyle().Bold(true),
	}
}

//
// Construction & lifecycle
//

// New creates a new Logger.
func New(opts Options) *Logger {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.TailLines <= 0 {
		opts.TailLines = 5
	}

	return &Logger{
		out:        opts.Out,
		full:       opts.FullLogWriter,
		style:      defaultStyles(),
		tailLines:  opts.TailLines,
		enableTail: opts.EnableTail,
		logLevel:   opts.LogLevel,
		component:  opts.Component,
	}
}

func (l *Logger) SetFullLogWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Reject if already set
	if l.full != nil {
		timestamp := time.Now().Format("2006-01-02T15:04:05.000")
		errMsg := fmt.Sprintf("[%s] [ERR ] attempted to set full log writer when already set, ignoring\n", timestamp)
		fmt.Fprint(l.out, l.style.logError.Render(errMsg))
		return
	}

	l.full = w

	// Flush buffered log lines
	for _, line := range l.fullLogBuffer {
		io.WriteString(l.full, line)
	}
	l.fullLogBuffer = nil
}

// writeFullLogLocked writes to the full log writer if set, otherwise buffers.
// Must be called with l.mu held.
func (l *Logger) writeFullLogLocked(line string) {
	if l.full != nil {
		io.WriteString(l.full, line)
	} else {
		l.fullLogBuffer = append(l.fullLogBuffer, line)
	}
}

// Close closes the full log if it's an io.Closer and finalizes any active tail.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.tail != nil && !l.tail.closed {
		l.finalizeTailLocked()
	}

	if c, ok := l.full.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

//
// Basic logging
//

func (l *Logger) Spacer() {
	l.printLog(false, "", l.style.spacer, "")
}

func (l *Logger) Error(format string, args ...any) {
	l.printLog(false, "ERR ", l.style.logError, format, args...)
}

func (l *Logger) Info(format string, args ...any) {
	silent := l.logLevel < LogLevelInfo
	l.printLog(silent, "INFO", l.style.logInfo, format, args...)
}

func (l *Logger) Warn(format string, args ...any) {
	silent := l.logLevel < LogLevelWarn
	l.printLog(silent, "WARN", l.style.logWarn, format, args...)
}

func (l *Logger) InfoSilent(format string, args ...any) {
	l.printLog(true, "INFO", l.style.logInfo, format, args...)
}

func (l *Logger) Debug(format string, args ...any) {
	if l.logLevel >= LogLevelDebug {
		l.printLog(false, "DEBG", l.style.logInfo, format, args...)
	}
}

func (l *Logger) SetLogLevel(logLevel LogLevel) {
	l.logLevel = logLevel
}

func (l *Logger) formatCaller(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	if l.logLevel < LogLevelDebugVerbose {
		return msg
	}
	pc, file, line, ok := runtime.Caller(4)
	if !ok {
		file = "?"
		line = 0
	}

	fn := runtime.FuncForPC(pc)
	var fnName string
	if fn != nil {
		fnName = strings.ReplaceAll(fn.Name(), "github.com/0xa1bed0/mkenv", "")
	}

	return fmt.Sprintf("[%s:%d %s] %s", filepath.Base(file), line, fnName, msg)
}

// printLog handles clearing/redrawing tail box around a log line.
func (l *Logger) printLog(silent bool, level string, style lipgloss.Style, format string, args ...any) {
	msg := l.formatCaller(format, args...)
	timestamp := time.Now().Format("2006-01-02T15:04:05.000")

	// Build component tag if set
	componentTag := ""
	if l.component != "" {
		componentTag = fmt.Sprintf("[%s] ", l.component)
	}

	// Format for full log: no timestamp (TimestampWriter adds it at the destination)
	logLine := componentTag + msg + "\n"
	if level != "" {
		logLine = fmt.Sprintf("[%s] %s%s\n", level, componentTag, msg)
	}

	// Format for stdout: includes timestamp
	stdoutLine := fmt.Sprintf("[%s] %s%s", timestamp, componentTag, msg)
	if level != "" {
		stdoutLine = fmt.Sprintf("[%s] [%s] %s%s", timestamp, level, componentTag, msg)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// If there's a live box, clear it first.
	if l.enableTail && l.tail != nil && !l.tail.closed && l.tail.lastBoxHeight > 0 {
		l.clearTailBoxLocked()
	}

	// Write to full log (without timestamp - TimestampWriter adds it).
	l.writeFullLogLocked(logLine)

	if !silent {
		// Write styled to stdout (with timestamp).
		styled := style.Render(stdoutLine)
		fmt.Fprintln(l.out, styled)

		// Redraw live tail box if still active.
		if l.enableTail && l.tail != nil && !l.tail.closed && len(l.tail.buf) > 0 {
			l.drawTailBoxLocked()
		}
	}
}

// Banner prints a nice box title.
func (l *Logger) Banner(title string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.enableTail && l.tail != nil && !l.tail.closed && l.tail.lastBoxHeight > 0 {
		l.clearTailBoxLocked()
	}

	bannerLine := fmt.Sprintf("\n===== %s =====\n\n", title)
	l.writeFullLogLocked(bannerLine)
	// Force sync for important banners to ensure immediate visibility
	if s, ok := l.full.(syncer); ok {
		s.Sync()
	}

	box := l.style.banner.Render(title)
	fmt.Fprintln(l.out, box)

	if l.enableTail && l.tail != nil && !l.tail.closed && len(l.tail.buf) > 0 {
		l.drawTailBoxLocked()
	}
}
