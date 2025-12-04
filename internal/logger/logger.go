package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// Level represents logging level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// String returns string representation of log level
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses log level from string
func ParseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// Logger provides structured logging
type Logger struct {
	level  Level
	logger *log.Logger
}

// New creates a new logger with specified level
func New(level Level) *Logger {
	return NewWithWriter(level, os.Stdout)
}

// NewWithWriter creates a new logger with specified level and writer
func NewWithWriter(level Level, w io.Writer) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(w, "", 0),
	}
}

// log writes a log message with specified level
func (l *Logger) log(level Level, msg string, fields ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := level.String()

	// Build fields string
	fieldsStr := ""
	if len(fields) > 0 {
		parts := make([]string, 0, len(fields)/2)
		for i := 0; i < len(fields)-1; i += 2 {
			key := fmt.Sprintf("%v", fields[i])
			value := fmt.Sprintf("%v", fields[i+1])
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
		if len(parts) > 0 {
			fieldsStr = " " + strings.Join(parts, " ")
		}
	}

	l.logger.Printf("[%s] %s: %s%s", timestamp, levelStr, msg, fieldsStr)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.log(DEBUG, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...interface{}) {
	l.log(INFO, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.log(WARN, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...interface{}) {
	l.log(ERROR, msg, fields...)
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level Level) {
	l.level = level
}
