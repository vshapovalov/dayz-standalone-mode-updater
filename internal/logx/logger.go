package logx

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"
)

type Logger struct {
	base *log.Logger
}

func New() *Logger {
	return &Logger{base: log.New(os.Stdout, "", 0)}
}

func (l *Logger) Info(msg string, fields map[string]any) {
	l.emit("info", msg, fields)
}

func (l *Logger) Error(msg string, err error, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	l.emit("error", msg, fields)
}

func (l *Logger) emit(level, msg string, fields map[string]any) {
	entry := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339),
		"level": level,
		"msg":   msg,
	}
	for k, v := range fields {
		entry[k] = sanitize(k, v)
	}
	b, _ := json.Marshal(entry)
	l.base.Println(string(b))
}

func sanitize(key string, value any) any {
	k := strings.ToLower(key)
	if strings.Contains(k, "password") || strings.Contains(k, "passphrase") || strings.Contains(k, "secret") || strings.Contains(k, "token") || strings.Contains(k, "api_key") || strings.Contains(k, "key") {
		return "***"
	}
	return value
}
