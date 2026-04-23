package main

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	wishlog "github.com/charmbracelet/wish/logging"
)

type Shux struct {
	Logger *Logger
	Wish   *ssh.Server
}

func NewShux() (*Shux, error) {
	logger, err := NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	hostKey, err := hostKeyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host key path: %w", err)
	}

	server, err := wish.NewServer(
		wish.WithAddress(":23234"),
		wish.WithHostKeyPath(hostKey),
		wish.WithMiddleware(
			wishlog.MiddlewareWithLogger(logger),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init wish server: %w", err)
	}

	return &Shux{
		Logger: logger,
		Wish:   server,
	}, nil
}

func (a *Shux) Run() error {
	defer a.Logger.Close()

	err := a.Wish.ListenAndServe()
	if err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		a.Logger.Error(err.Error())
		return err
	}
	return nil
}
