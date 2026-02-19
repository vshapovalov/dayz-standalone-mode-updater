package logging

import (
	"log"
	"os"
)

type Logger interface {
	Info(msg string, fields map[string]any)
	Error(msg string, err error, fields map[string]any)
}

type StdLogger struct {
	logger *log.Logger
}

func New() *StdLogger {
	return &StdLogger{logger: log.New(os.Stdout, "", log.LstdFlags)}
}

func (l *StdLogger) Info(msg string, fields map[string]any) {
	l.logger.Printf("INFO %s fields=%v", msg, fields)
}

func (l *StdLogger) Error(msg string, err error, fields map[string]any) {
	l.logger.Printf("ERROR %s err=%v fields=%v", msg, err, fields)
}
