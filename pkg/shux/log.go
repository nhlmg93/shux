package shux

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// LogLevel for controlling log output
type LogLevel int

const (
	LevelError LogLevel = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

var (
	// Logger is the global shux logger, writes to ~/.local/share/shux/shux.log
	Logger   *log.Logger
	logLevel LogLevel = LevelInfo // Default level (set to LevelDebug for development)
	initOnce sync.Once
)

// InitLogger initializes the global logger to write to file
func InitLogger() error {
	var err error
	initOnce.Do(func() {
		Logger, err = newLogger()
	})
	return err
}

// SetLogLevel sets the minimum log level to output
func SetLogLevel(level LogLevel) {
	logLevel = level
}

func newLogger() (*log.Logger, error) {
	// Create log directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	logDir := filepath.Join(home, ".local", "share", "shux")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, "shux.log")

	// Open log file (append mode, create if not exists)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return log.New(file, "", log.LstdFlags|log.Lmicroseconds), nil
}

// Errorf logs an error message (always logged)
func Errorf(format string, args ...interface{}) {
	if Logger == nil {
		return
	}
	Logger.Printf("[ERROR] "+format, args...)
}

// Warnf logs a warning message (logged if level >= Warn)
func Warnf(format string, args ...interface{}) {
	if Logger == nil || logLevel > LevelWarn {
		return
	}
	Logger.Printf("[WARN] "+format, args...)
}

// Infof logs an info message (logged if level >= Info)
func Infof(format string, args ...interface{}) {
	if Logger == nil || logLevel > LevelInfo {
		return
	}
	Logger.Printf("[INFO] "+format, args...)
}

// Debugf logs a debug message (logged if level >= Debug)
func Debugf(format string, args ...interface{}) {
	if Logger == nil || logLevel > LevelDebug {
		return
	}
	Logger.Printf("[DEBUG] "+format, args...)
}

// Printf is an alias for Infof (for compatibility)
func Printf(format string, args ...interface{}) {
	Infof(format, args...)
}

// LogPath returns the path to the log file
func LogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "shux", "shux.log")
}

// Logf provides formatted logging with a specific level
func Logf(level LogLevel, format string, args ...interface{}) {
	switch level {
	case LevelError:
		Errorf(format, args...)
	case LevelWarn:
		Warnf(format, args...)
	case LevelInfo:
		Infof(format, args...)
	case LevelDebug:
		Debugf(format, args...)
	default:
		if Logger != nil {
			Logger.Printf(format, args...)
		}
	}
}

// Debug helper to dump current log file path and status
func LogStatus() string {
	if Logger == nil {
		return "Logger not initialized"
	}
	path := LogPath()
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("Log path: %s (error: %v)", path, err)
	}
	return fmt.Sprintf("Log path: %s (size: %d bytes)", path, info.Size())
}
