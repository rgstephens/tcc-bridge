package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Entry represents a structured log entry
type Entry struct {
	Time    time.Time              `json:"time"`
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// Logger provides structured logging
type Logger struct {
	mu       sync.Mutex
	out      io.Writer
	level    Level
	fields   map[string]interface{}
	jsonMode bool
}

// New creates a new logger
func New() *Logger {
	return &Logger{
		out:    os.Stdout,
		level:  LevelInfo,
		fields: make(map[string]interface{}),
	}
}

// SetOutput sets the log output destination
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetJSONMode enables or disables JSON output
func (l *Logger) SetJSONMode(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.jsonMode = enabled
}

// WithField returns a new logger with an additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newFields := make(map[string]interface{}, len(l.fields)+1)
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value

	return &Logger{
		out:      l.out,
		level:    l.level,
		fields:   newFields,
		jsonMode: l.jsonMode,
	}
}

// WithFields returns a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newFields := make(map[string]interface{}, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &Logger{
		out:      l.out,
		level:    l.level,
		fields:   newFields,
		jsonMode: l.jsonMode,
	}
}

func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	formattedMsg := msg
	if len(args) > 0 {
		formattedMsg = fmt.Sprintf(msg, args...)
	}

	if l.jsonMode {
		entry := Entry{
			Time:    time.Now().UTC(),
			Level:   level.String(),
			Message: formattedMsg,
			Fields:  l.fields,
		}
		data, _ := json.Marshal(entry)
		fmt.Fprintln(l.out, string(data))
	} else {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		if len(l.fields) > 0 {
			fieldsStr, _ := json.Marshal(l.fields)
			fmt.Fprintf(l.out, "%s [%s] %s %s\n", timestamp, level.String(), formattedMsg, fieldsStr)
		} else {
			fmt.Fprintf(l.out, "%s [%s] %s\n", timestamp, level.String(), formattedMsg)
		}
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(LevelDebug, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(LevelWarn, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(LevelError, msg, args...)
}

// Default logger instance
var defaultLogger = New()

// SetDefaultLevel sets the level for the default logger
func SetDefaultLevel(level Level) {
	defaultLogger.SetLevel(level)
}

// Debug logs using the default logger
func Debug(msg string, args ...interface{}) {
	defaultLogger.Debug(msg, args...)
}

// Info logs using the default logger
func Info(msg string, args ...interface{}) {
	defaultLogger.Info(msg, args...)
}

// Warn logs using the default logger
func Warn(msg string, args ...interface{}) {
	defaultLogger.Warn(msg, args...)
}

// Error logs using the default logger
func Error(msg string, args ...interface{}) {
	defaultLogger.Error(msg, args...)
}

// WithField returns a logger with an additional field
func WithField(key string, value interface{}) *Logger {
	return defaultLogger.WithField(key, value)
}

// WithFields returns a logger with additional fields
func WithFields(fields map[string]interface{}) *Logger {
	return defaultLogger.WithFields(fields)
}
