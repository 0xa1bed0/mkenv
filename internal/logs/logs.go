package logs

import (
	"io"
	"os"
	"sync"

	"github.com/0xa1bed0/mkenv/internal/ui"
	"github.com/0xa1bed0/mkenv/internal/version"
)

var (
	initOnce  sync.Once
	logger    *ui.Logger
	component string
)

// SetComponent sets the component name for all log messages.
// Must be called before first log (before Init() or L()).
func SetComponent(name string) {
	component = name
}

func Init() {
	initOnce.Do(func() {
		logLevel := ui.LogLevelWarn
		if version.Get() == "local" {
			logLevel = ui.LogLevelDebugVerbose
		}

		opts := ui.Options{
			Out:        os.Stdout,
			TailLines:  15,
			EnableTail: true,
			LogLevel:   logLevel,
			Component:  component,
		}
		logger = ui.New(opts)
		logger.DebugSilent("logs initialized with opts %v", opts)
	})
}

func L() *ui.Logger {
	Init()
	return logger
}

func SetDebugVerbosity(cnt int) {
	switch {
	case cnt <= 0:
		L().SetLogLevel(ui.LogLevelWarn)
	case cnt == 1:
		L().SetLogLevel(ui.LogLevelDebug)
	default:
		L().SetLogLevel(ui.LogLevelDebugVerbose)
	}
}

func SetFullLogWriter(w io.Writer) {
	L().SetFullLogWriter(w)
}

func Mute() (restore func()) {
	return L().MuteStdout()
}

func Banner(title string) {
	L().Banner(title)
}

func Spacer() {
	L().Spacer()
}

func Infof(format string, args ...any) {
	L().Info(format, args...)
}

func InfofSilent(format string, args ...any) {
	L().InfoSilent(format, args...)
}

func Debugf(format string, args ...any) {
	L().Debug(format, args...)
}

func Warnf(format string, args ...any) {
	L().Warn(format, args...)
}

func Errorf(format string, args ...any) {
	L().Error(format, args...)
}

func NewTailBox(name string) ui.Tail {
	return L().NewTail(name)
}

type defaultSelectOption struct {
	Text string
	ID   string
}

func (so *defaultSelectOption) OptionLabel() string {
	return so.Text
}

func (so *defaultSelectOption) OptionID() string {
	return so.ID
}

func NewSelectOption(text, id string) ui.SelectOption {
	return &defaultSelectOption{Text: text, ID: id}
}

func PromptSelectOne(label string, options []ui.SelectOption) (ui.SelectOption, error) {
	return L().SelectOne(label, options)
}

func PromptSelectMany(label string, options []ui.SelectOption) ([]ui.SelectOption, error) {
	return L().SelectMany(label, options)
}

func PromptConfirm(text string) (bool, error) {
	return L().Confirm(text)
}

// Close closes the underlying log file, if any.
func Close() error {
	if logger != nil {
		return logger.Close()
	}
	return nil
}
