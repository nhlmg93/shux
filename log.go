package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type LogLevel int

const (
	LevelError LogLevel = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

type Logger struct {
	logger   *log.Logger
	Level    LogLevel
	initOnce sync.Once
}

func NewLogger() *Logger {
	return &Logger{
		Level: LevelInfo,
	}
}

func (l *Logger) Init() error {
	var err error
	l.initOnce.Do(func() {
		l.logger, err = l.openLogFile()
	})
	return err
}

func (l *Logger) openLogFile() (*log.Logger, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	logDir := filepath.Join(home, ".local", "share", "shux-dev")
	err = os.MkdirAll(logDir, 0o755)
	if err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, "app.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	return log.New(file, "", log.LstdFlags|log.Lmicroseconds), nil
}

func (l *Logger) Errorf(format string, args ...any) {
	if l.logger == nil {
		return
	}
	l.logger.Printf("[ERROR] "+format, args...)
}

func (l *Logger) Warnf(format string, args ...any) {
	if l.logger == nil || l.Level < LevelWarn {
		return
	}
	l.logger.Printf("[WARN] "+format, args...)
}

func (l *Logger) Infof(format string, args ...any) {
	if l.logger == nil || l.Level < LevelInfo {
		return
	}
	l.logger.Printf("[INFO] "+format, args...)
}

func (l *Logger) Debugf(format string, args ...any) {
	if l.logger == nil || l.Level < LevelDebug {
		return
	}
	l.logger.Printf("[DEBUG] "+format, args...)
}

func (l *Logger) Logf(level LogLevel, format string, args ...any) {
	switch level {
	case LevelError:
		l.Errorf(format, args...)
	case LevelWarn:
		l.Warnf(format, args...)
	case LevelInfo:
		l.Infof(format, args...)
	case LevelDebug:
		l.Debugf(format, args...)
	default:
		if l.logger != nil {
			l.logger.Printf(format, args...)
		}
	}
}

func (l *Logger) Path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "shux-dev", "app.log")
}

func (l *Logger) Status() string {
	if l.logger == nil {
		return "Logger not initialized"
	}
	path := l.Path()
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("Log path: %s (error: %v)", path, err)
	}
	return fmt.Sprintf("Log path: %s (size: %d bytes)", path, info.Size())
}
