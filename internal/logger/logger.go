package logger

import (
	"fmt"
	"os"
	"time"

	constants "catops/config"
)

// Level represents log level
type Level string

const (
	LevelInfo    Level = "INFO"
	LevelWarning Level = "WARNING"
	LevelError   Level = "ERROR"
	LevelSuccess Level = "SUCCESS"
	LevelDebug   Level = "DEBUG"
)

// Logger handles centralized logging to file
type Logger struct {
	filePath string
}

// New creates a new logger instance
func New(filePath string) *Logger {
	return &Logger{
		filePath: filePath,
	}
}

// Default returns a logger with default settings
func Default() *Logger {
	return &Logger{
		filePath: constants.LOG_FILE,
	}
}

// write writes a log entry to the log file AND stdout (for Kubernetes)
func (l *Logger) write(level Level, message string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formattedMsg := fmt.Sprintf(message, args...)
	logEntry := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, formattedMsg)

	// Check if running in Kubernetes (NODE_NAME env var is set by K8s)
	isKubernetes := os.Getenv("NODE_NAME") != ""

	if isKubernetes {
		// In Kubernetes: write to stdout (for kubectl logs)
		fmt.Print(logEntry)
	} else {
		// In standalone mode: write to file only (existing behavior)
		if l.filePath != "" {
			logFile, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return // Silently fail if we can't open log file
			}
			defer logFile.Close()
			logFile.WriteString(logEntry)
		}
	}
}

// Info logs an informational message
func (l *Logger) Info(message string, args ...interface{}) {
	l.write(LevelInfo, message, args...)
}

// Warning logs a warning message
func (l *Logger) Warning(message string, args ...interface{}) {
	l.write(LevelWarning, message, args...)
}

// Error logs an error message
func (l *Logger) Error(message string, args ...interface{}) {
	l.write(LevelError, message, args...)
}

// Success logs a success message
func (l *Logger) Success(message string, args ...interface{}) {
	l.write(LevelSuccess, message, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(message string, args ...interface{}) {
	l.write(LevelDebug, message, args...)
}

// Global logger instance for convenience
var defaultLogger = Default()

// Info logs an informational message using the default logger
func Info(message string, args ...interface{}) {
	defaultLogger.Info(message, args...)
}

// Warning logs a warning message using the default logger
func Warning(message string, args ...interface{}) {
	defaultLogger.Warning(message, args...)
}

// Error logs an error message using the default logger
func Error(message string, args ...interface{}) {
	defaultLogger.Error(message, args...)
}

// Success logs a success message using the default logger
func Success(message string, args ...interface{}) {
	defaultLogger.Success(message, args...)
}

// Debug logs a debug message using the default logger
func Debug(message string, args ...interface{}) {
	defaultLogger.Debug(message, args...)
}
