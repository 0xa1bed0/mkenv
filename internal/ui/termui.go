package termui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/charmbracelet/lipgloss"
)

//
// Public types & options
//

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
}

// SelectOption is the interface for items that can be used in SelectOne/SelectMany.
type SelectOption interface {
	OptionLabel() string // what user sees
	OptionID() string    // stable identifier for logs/logic
}

// ToSelectOptions converts a slice of any SelectOption implementation
// (including pointer types like []*ContainerInfo) into []SelectOption.
func ToSelectOptions[T SelectOption](items []T) []SelectOption {
	out := make([]SelectOption, len(items))
	for i := range items {
		out[i] = items[i]
	}
	return out
}

// Logger is the main stdout logger + tail manager.
type Logger struct {
	out   io.Writer
	full  io.Writer
	mu    sync.Mutex
	style styles

	tail       *tailState
	tailLines  int
	enableTail bool
}

// Tail is a handle for writing into a tail box.
type Tail interface {
	Println(msg string)
	Close()
}

//
// Internal types
//

// internal tail state.
type tailState struct {
	name          string
	buf           []string
	lastBoxHeight int
	closed        bool
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

func (l *Logger) Info(format string, args ...any) {
	l.printLog(false, "INFO", l.style.logInfo, format, args...)
}

func (l *Logger) InfoSilent(format string, args ...any) {
	l.printLog(true, "INFO", l.style.logInfo, format, args...)
}

func (l *Logger) Warn(format string, args ...any) {
	l.printLog(false, "WARN", l.style.logWarn, format, args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.printLog(false, "ERR ", l.style.logError, format, args...)
}

// printLog handles clearing/redrawing tail box around a log line.
func (l *Logger) printLog(silent bool, level string, style lipgloss.Style, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
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

//
// Tail API
//

type tailHandle struct {
	ui *Logger
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

func (t *tailHandle) Println(msg string) {
	l := t.ui
	l.mu.Lock()
	defer l.mu.Unlock()

	// No active tail – log plainly.
	if l.tail == nil || l.tail.closed {
		if l.full != nil {
			fmt.Fprintf(l.full, "[TAIL] %s\n", msg)
		}
		fmt.Fprintln(l.out, msg)
		return
	}

	// Append to tail buffer and trim to last N lines.
	l.tail.buf = append(l.tail.buf, msg)
	if len(l.tail.buf) > l.tailLines {
		l.tail.buf = l.tail.buf[len(l.tail.buf)-l.tailLines:]
	}

	// Full log gets every tail line.
	if l.full != nil {
		fmt.Fprintf(l.full, "[TAIL %s] %s\n", l.tail.name, msg)
	}

	if !l.enableTail {
		// Tailbox disabled: just print line-by-line.
		fmt.Fprintln(l.out, msg)
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

//
// SelectOne / SelectMany with arrow keys (survey)
//

func (l *Logger) finalizeTailForPrompt() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.tail != nil && !l.tail.closed {
		// This will clear live box and print static box once.
		l.finalizeTailLocked()
	}
}

func formatSelectOptionsForLog(options []SelectOption) string {
	var parts []string
	for _, opt := range options {
		parts = append(parts, fmt.Sprintf("%s(%s)", opt.OptionID(), opt.OptionLabel()))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// SelectOne asks the user to choose one option with an arrow-key menu.
// It logs the prompt and the answer (ID + label) to the full log.
func (l *Logger) SelectOne(label string, options []SelectOption) (idx int, value SelectOption, err error) {
	if len(options) == 0 {
		return -1, nil, fmt.Errorf("SelectOne: no options provided")
	}

	// Ensure no live tail box is on screen.
	l.finalizeTailForPrompt()

	l.Spacer()
	l.InfoSilent("PROMPT: %s (options: %s)", label, formatSelectOptionsForLog(options))

	display := make([]string, len(options))
	for i, opt := range options {
		display[i] = opt.OptionLabel()
	}

	var chosenLabel string

	prompt := &survey.Select{
		Message: label,
		Options: display,
	}

	err = survey.AskOne(
		prompt,
		&chosenLabel,
		survey.WithStdio(os.Stdin, os.Stdout, os.Stderr),
	)
	if err != nil {
		l.Error("PROMPT FAILED: %v", err)
		return -1, nil, err
	}

	// Map chosen label back to index.
	for i, opt := range options {
		if opt.OptionLabel() == chosenLabel {
			idx = i
			value = opt
			l.InfoSilent("ANSWER: id=%s label=%s", value.OptionID(), value.OptionLabel())
			return idx, value, nil
		}
	}

	// Shouldn't happen, but be defensive.
	l.Error("PROMPT ERROR: chosen label %q not found in options", chosenLabel)
	return -1, nil, fmt.Errorf("chosen label not found")
}

// SelectMany asks the user to choose multiple options using an arrow-key multi-select
// (space to toggle, enter to confirm). It logs the prompt and selected IDs/labels.
func (l *Logger) SelectMany(label string, options []SelectOption) (indices []int, values []SelectOption, err error) {
	if len(options) == 0 {
		return nil, nil, fmt.Errorf("SelectMany: no options provided")
	}

	l.finalizeTailForPrompt()

	l.Spacer()
	l.InfoSilent("PROMPT: %s (multi-select; options: %s)", label, formatSelectOptionsForLog(options))

	display := make([]string, len(options))
	for i, opt := range options {
		display[i] = opt.OptionLabel()
	}

	var chosenLabels []string

	prompt := &survey.MultiSelect{
		Message: label,
		Options: display,
	}

	err = survey.AskOne(
		prompt,
		&chosenLabels,
		survey.WithStdio(os.Stdin, os.Stdout, os.Stderr),
	)
	if err != nil {
		l.Error("PROMPT FAILED: %v", err)
		return nil, nil, err
	}

	if len(chosenLabels) == 0 {
		l.InfoSilent("ANSWER: []")
		return nil, nil, nil
	}

	// Map labels back to indices/options.
	labelSet := make(map[string]struct{}, len(chosenLabels))
	for _, lab := range chosenLabels {
		labelSet[lab] = struct{}{}
	}

	for i, opt := range options {
		if _, ok := labelSet[opt.OptionLabel()]; ok {
			indices = append(indices, i)
			values = append(values, opt)
		}
	}

	// Log answer.
	var answerParts []string
	for _, v := range values {
		answerParts = append(answerParts, fmt.Sprintf("%s(%s)", v.OptionID(), v.OptionLabel()))
	}
	l.InfoSilent("ANSWER: %s", "["+strings.Join(answerParts, ", ")+"]")

	return indices, values, nil
}

//
// Utility for SelectMany (we still keep it, might be useful elsewhere)
//

func splitIndexes(s string) []string {
	s = strings.ReplaceAll(s, ",", " ")
	fields := strings.Fields(s)
	return fields
}
