package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

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
}

// Logger is the main stdout logger + tail manager.
type Logger struct {
	out   io.Writer
	full  io.Writer
	mu    sync.Mutex
	style styles

	logLevel LogLevel

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
	}
}

func (l *Logger) SetFullLogPath(path string) {
	if l.full != nil {
		l.Warn("attempt to change project audit log file. Skipping...")
		return
	}

	var err error
	l.full, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		l.Error("can't open full log path: %v", err)
		return
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
	plain := fmt.Sprintf("%s\n", msg)
	if level != "" {
		plain = fmt.Sprintf("[%s] %s\n", level, msg)
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	// If there's a live box, clear it first.
	if l.enableTail && l.tail != nil && !l.tail.closed && l.tail.lastBoxHeight > 0 {
		l.clearTailBoxLocked()
	}

	// Write to full log.
	if l.full != nil {
		io.WriteString(l.full, plain)
	}

	if !silent {
		// Write styled to stdout.
		line := strings.TrimRight(plain, "\n")
		styled := style.Render(line)
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

	if l.full != nil {
		fmt.Fprintf(l.full, "\n===== %s =====\n\n", title)
	}

	box := l.style.banner.Render(title)
	fmt.Fprintln(l.out, box)

	if l.enableTail && l.tail != nil && !l.tail.closed && len(l.tail.buf) > 0 {
		l.drawTailBoxLocked()
	}
}
