package log

import (
	"fmt"
	"log"
	"os"
)

// Logger allows one to add Zipkin expections and errors to their structured
// logging infrastructure. It is the Go kit Logger compatible.
type Logger interface {
	Log(keyvals ...interface{}) error
}

type noopLogger struct {
	Logger
}

func (noopLogger) Log(_ ...interface{}) error { return nil }

// NewNoopLogger returns a noop logger
func NewNoopLogger() Logger {
	return &noopLogger{}
}

type stdLogger struct {
	logger *log.Logger
}

func (s *stdLogger) Log(keyvals ...interface{}) error {
	return s.logger.Output(2, fmt.Sprintln(keyvals...))
}

// WrapStdLogger returns a Zipkin compatible Logger by wrapping a Go standard
// library Logger. If logger is nil a new Logger with regular defaults is
// created first.
func WrapStdLogger(logger *log.Logger) Logger {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	return &stdLogger{logger: logger}
}
