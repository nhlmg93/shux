package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	file *os.File
	ch   chan string
	done chan struct{}
}

func appDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(home, ".local", "share", "shux-dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return dir, nil
}

func logPath() (string, error) {
	dir, err := appDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "shux.log"), nil
}

func hostKeyPath() (string, error) {
	dir, err := appDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "id_ed25519"), nil
}

func NewLogger() (*Logger, error) {
	path, err := logPath()
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	logger := &Logger{
		file: file,
		ch:   make(chan string, 128),
		done: make(chan struct{}),
	}
	go logger.run()

	return logger, nil
}

func (l *Logger) run() {
	defer close(l.done)

	for line := range l.ch {
		fmt.Fprintln(l.file, line)
	}
}

func (l *Logger) log(level, msg string) {
	if l == nil || l.file == nil {
		return
	}

	l.ch <- fmt.Sprintf("%s [%s] %s", time.Now().Format(time.RFC3339), level, msg)
}

func (l *Logger) Printf(format string, args ...any) {
	l.log("INFO", fmt.Sprintf(format, args...))
}

func (l *Logger) Info(msg string) {
	l.log("INFO", msg)
}

func (l *Logger) Error(msg string) {
	l.log("ERROR", msg)
}

func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}

	close(l.ch)
	<-l.done

	return l.file.Close()
}
